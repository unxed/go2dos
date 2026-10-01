// Package wasidos implements the WASI bridge for DOS programs.
// Provides access to the host through WASI preview1 ABI.
package wasidos

import (
	"encoding/binary"
	"time"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/dos"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// WASI errno values (subset used in v0)
const (
	ESUCCESS   = 0
	EBADF      = 8
	EACCES     = 2
	EEXIST     = 20
	ENOSPC     = 28
	ENOSYS     = 38
	EISDIR     = 21
	ENOTDIR    = 20
	ENAMETOOLONG = 63
)

// WasiOS provides the WASI bridge implementation.
type WasiOS struct {
	e   *hle.Env
	d   *dos.DOS
	dos *DOS

	// AMIS multiplex number for discovery
	amisNum byte
}

// DOS-specific extensions for WASI bridge (file descriptors, preopen handling)
type DOS struct {
	// fd table: map from WASI fd to DOS file handle or special device
	fdTable map[int32]fdEntry

	// Next available fd (starts at 3, 0-2 are reserved for stdin/stdout/stderr)
	nextFD int32

	// Preopen directories: fd -> DOS drive letter
	preopens map[int32]byte

	// Preopen names: drive letter -> "C:", "D:", etc.
	preopenNames map[byte]string
}

// fdEntry represents one file descriptor in the WASI bridge
type fdEntry struct {
	fdType    byte            // 0=unused, 1=file, 2=dir, 3=console
	dosHandle uint16          // DOS file handle (for opened files)
	path      string          // current path (for directories)
	isFile    bool            // true if file, false if directory
	isRead    bool            // true if readable
	isWrite   bool            // true if writable
	offset    int64           // current file offset for seek
	drive     byte            // drive letter for preopen
}

// NewWasiOS creates a new WASI bridge instance.
func NewWasiOS(e *hle.Env, d *dos.DOS, amisNum byte) (*WasiOS, error) {
	w := &WasiOS{
		e:       e,
		d:       d,
		amisNum: amisNum,
		dos: &DOS{
			fdTable:      make(map[int32]fdEntry),
			nextFD:       3,
			preopens:     make(map[int32]byte),
			preopenNames: make(map[byte]string),
		},
	}

	// Register fd 0, 1, 2 as console (stdin, stdout, stderr)
	w.dos.fdTable[0] = fdEntry{fdType: 3, isRead: true}  // stdin
	w.dos.fdTable[1] = fdEntry{fdType: 3, isWrite: true} // stdout
	w.dos.fdTable[2] = fdEntry{fdType: 3, isWrite: true} // stderr

	// Register preopen directories for each drive (C:, D:, ...)
	// This will be set up when drives are enumerated
	// For now, we assume C: is always drive 0 = fd 3
	w.dos.preopens[3] = 'C'
	w.dos.preopenNames['C'] = "C:"

	return w, nil
}

// Handler returns the INT 2Dh trap handler for the WASI bridge.
func (w *WasiOS) Handler(e *hle.Env) error {
	c := e.CPU
	ah := c.AH()

	// AMIS dispatching
	if ah == w.amisNum {
		return w.amisDispatch(e)
	}

	// Not our multiplex number
	return nil
}

// amisDispatch handles AMIS 3.6 discovery and dispatching on INT 2Dh.
func (w *WasiOS) amisDispatch(e *hle.Env) error {
	c := e.CPU
	al := c.AL()

	switch al {
	case 0x00:
		// Check if installed: return AL=FFh and signature
		c.SetAL(0xFF)
		// Fill in signature at ES:DI
		es := c.S[cpu.ES].Sel
		di := c.R[cpu.DI]
		sig := w.createSignature()
		e.Mem.SetBytes(mem.Lin(es, di), sig)
		// Set CX to version (for example, 0x0100 for version 1.00)
		c.R[cpu.CX] = 0x0100
		// Set DX:DI as per AMIS (though specification says DX:DI as signature pointer)
		c.R[cpu.DX] = es
		c.R[cpu.DI] = di
		return nil

	case 0x01:
		// Get private entry point: return DX:BX far pointer
		// This is the address of the WASI entry point stub
		// For now, we return the INT 2Dh handler address
		c.SetAL(0xFF) // Supported
		// In real implementation, DX:BX would point to a ROM stub
		// For now, return a placeholder
		c.R[cpu.DX] = 0xF000 // ROM segment
		c.R[cpu.BX] = 0x3000 // Placeholder offset
		return nil

	default:
		// Not an AMIS function we recognize
		return nil
	}
}

// createSignature creates the AMIS signature for the WASI bridge.
func (w *WasiOS) createSignature() []byte {
	sig := make([]byte, 32)
	// Vendor: "DOS-WASI " (8 bytes, space-padded)
	copy(sig[0:8], "DOS-WASI ")
	// Product: "PREVIEW1" (8 bytes, space-padded)
	copy(sig[8:16], "PREVIEW1")
	// Description: "WASI Bridge for DOS" (null-terminated)
	copy(sig[16:], "WASI Bridge for DOS\x00")
	return sig
}

// Dispatch is called to handle a WASI function call.
// Entry: AX = function number, DS:SI = parameter block
// Exit: AX = errno, CF set if error
func (w *WasiOS) Dispatch(e *hle.Env) error {
	c := e.CPU
	ax := c.R[cpu.AX]

	if ax >= 24 {
		// Function out of range, return ENOSYS
		c.R[cpu.AX] = ENOSYS
		e.SetCF(true)
		return nil
	}

	// Dispatch based on function number
	switch ax {
	case 0:
		return w.argsSizesGet(e)
	case 1:
		return w.argsGet(e)
	case 2:
		return w.environSizesGet(e)
	case 3:
		return w.environGet(e)
	case 4:
		return w.clockTimeGet(e)
	case 5:
		return w.randomGet(e)
	case 6:
		return w.procExit(e)
	case 7:
		return w.schedYield(e)
	case 8:
		return w.fdPrestatGet(e)
	case 9:
		return w.fdPrestatDirName(e)
	case 10:
		return w.fdRead(e)
	case 11:
		return w.fdWrite(e)
	case 12:
		return w.fdSeek(e)
	case 13:
		return w.fdTell(e)
	case 14:
		return w.fdClose(e)
	case 15:
		return w.fdFilestatGet(e)
	case 16:
		return w.pathOpen(e)
	case 17:
		return w.pathFilestatGet(e)
	case 18:
		return w.pathCreateDirectory(e)
	case 19:
		return w.pathRemoveDirectory(e)
	case 20:
		return w.pathUnlinkFile(e)
	case 21:
		return w.pathRename(e)
	case 22:
		return w.fdReaddir(e)
	case 23:
		return w.pollOneoff(e)

	default:
		c.R[cpu.AX] = ENOSYS
		e.SetCF(true)
		return nil
	}
}

// Helper to read parameters from the parameter block at DS:SI
func (w *WasiOS) readI32(e *hle.Env, seg, off uint16, offset int) int32 {
	addr := mem.Lin(seg, off) + uint32(offset)
	return int32(binary.LittleEndian.Uint32(e.Mem.Bytes(addr, 4)))
}

func (w *WasiOS) readI64(e *hle.Env, seg, off uint16, offset int) int64 {
	addr := mem.Lin(seg, off) + uint32(offset)
	return int64(binary.LittleEndian.Uint64(e.Mem.Bytes(addr, 8)))
}

func (w *WasiOS) readU32(e *hle.Env, seg, off uint16, offset int) uint32 {
	addr := mem.Lin(seg, off) + uint32(offset)
	return binary.LittleEndian.Uint32(e.Mem.Bytes(addr, 4))
}

func (w *WasiOS) readPtr(e *hle.Env, seg, off uint16, offset int) (uint16, uint16) {
	addr := mem.Lin(seg, off) + uint32(offset)
	data := e.Mem.Bytes(addr, 4)
	offsetPart := binary.LittleEndian.Uint16(data[0:2])
	segmentPart := binary.LittleEndian.Uint16(data[2:4])
	return segmentPart, offsetPart
}

// Helper to write results back to parameter block
func (w *WasiOS) writeI32(e *hle.Env, addr uint32, val int32) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(val))
	e.Mem.SetBytes(addr, buf)
}

func (w *WasiOS) writeI64(e *hle.Env, addr uint32, val int64) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(val))
	e.Mem.SetBytes(addr, buf)
}

// Helper to set CF and AX for error/success
func (w *WasiOS) setResult(e *hle.Env, errno uint32) {
	if errno != 0 {
		e.SetCF(true)
	} else {
		e.SetCF(false)
	}
	e.CPU.R[cpu.AX] = uint16(errno)
}

// --- W3 Functions: Basic (no files) ---

// argsSizesGet: returns argc and argv buffer size
func (w *WasiOS) argsSizesGet(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	// Read output pointers from parameter block
	pArgc := mem.Lin(seg, off)
	pArgvSize := mem.Lin(seg, off+4)

	// For now, no arguments
	argc := int32(0)
	argvSize := int32(0)

	w.writeI32(e, pArgc, argc)
	w.writeI32(e, pArgvSize, argvSize)

	w.setResult(e, ESUCCESS)
	return nil
}

// argsGet: returns argv array and buffer
func (w *WasiOS) argsGet(e *hle.Env) error {
	// For v0, return empty argv
	w.setResult(e, ESUCCESS)
	return nil
}

// environSizesGet: returns envc and environ buffer size
func (w *WasiOS) environSizesGet(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	pEnvc := mem.Lin(seg, off)
	pEnvSize := mem.Lin(seg, off+4)

	// For now, no environment variables
	envc := int32(0)
	envSize := int32(0)

	w.writeI32(e, pEnvc, envc)
	w.writeI32(e, pEnvSize, envSize)

	w.setResult(e, ESUCCESS)
	return nil
}

// environGet: returns environ array and buffer
func (w *WasiOS) environGet(e *hle.Env) error {
	// For v0, return empty environ
	w.setResult(e, ESUCCESS)
	return nil
}

// clockTimeGet: returns current time
func (w *WasiOS) clockTimeGet(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	clockID := w.readI32(e, seg, off, 0)
	_ = w.readI64(e, seg, off, 4) // precision (unused)

	// pTime is a 32-bit far pointer
	pTimeSeg, pTimeOff := w.readPtr(e, seg, off, 12)
	pTime := mem.Lin(pTimeSeg, pTimeOff)

	if clockID != 0 {
		// Only CLOCK_REALTIME (0) supported in v0
		w.setResult(e, ENOSYS)
		return nil
	}

	now := time.Now()
	nanos := now.UnixNano()

	w.writeI64(e, pTime, nanos)
	w.setResult(e, ESUCCESS)
	return nil
}

// randomGet: returns random bytes
func (w *WasiOS) randomGet(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	bufLen := w.readI32(e, seg, off, 0)
	pBuf := mem.Lin(seg, off+4)

	// For v0, return zeros (placeholder)
	// In production, would use crypto/rand
	buf := make([]byte, bufLen)
	e.Mem.SetBytes(pBuf, buf)

	w.setResult(e, ESUCCESS)
	return nil
}

// procExit: terminate process with exit code
func (w *WasiOS) procExit(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	exitCode := w.readI32(e, seg, off, 0)

	// Terminate the process (like INT 21h/4Ch in DOS)
	e.Stop = &hle.Exit{Code: byte(exitCode & 0xFF)}
	return nil
}

// schedYield: yield control to other processes
func (w *WasiOS) schedYield(e *hle.Env) error {
	e.Idle()
	w.setResult(e, ESUCCESS)
	return nil
}

// --- W4 Functions: Files ---

// fdPrestatGet: get preopen directory info
func (w *WasiOS) fdPrestatGet(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	fd := w.readI32(e, seg, off, 0)
	// pBuf is a 32-bit far pointer stored in the parameter block
	pBufSeg, pBufOff := w.readPtr(e, seg, off, 4)
	pBuf := mem.Lin(pBufSeg, pBufOff)

	// Check if fd is a preopen directory
	drive, isPreopen := w.dos.preopens[fd]
	if !isPreopen {
		w.setResult(e, EBADF)
		return nil
	}

	// Return prestat structure: tag=3 (dir), pr_name_len for dir name
	dirName := w.dos.preopenNames[drive]
	nameLen := int32(len(dirName))

	// Write prestat structure at pBuf (8 bytes)
	// Offset 0: tag (1 byte) = 3 for dir
	e.Mem.W8(pBuf, 3)
	// Offset 4: pr_name_len (4 bytes, little-endian)
	w.writeI32(e, pBuf+4, nameLen)

	w.setResult(e, ESUCCESS)
	return nil
}

// fdPrestatDirName: get preopen directory name
func (w *WasiOS) fdPrestatDirName(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	fd := w.readI32(e, seg, off, 0)
	pathLen := w.readI32(e, seg, off, 4)
	pPath := mem.Lin(seg, off+8)

	drive, isPreopen := w.dos.preopens[fd]
	if !isPreopen {
		w.setResult(e, EBADF)
		return nil
	}

	dirName := w.dos.preopenNames[drive]
	if int32(len(dirName)) > pathLen {
		w.setResult(e, ENOSPC)
		return nil
	}

	e.Mem.SetBytes(pPath, []byte(dirName))
	w.setResult(e, ESUCCESS)
	return nil
}

// fdRead: read from fd (placeholder for W5)
func (w *WasiOS) fdRead(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// fdWrite: write to fd
func (w *WasiOS) fdWrite(e *hle.Env) error {
	c := e.CPU
	seg := c.S[cpu.DS].Sel
	off := c.R[cpu.SI]

	fd := w.readI32(e, seg, off, 0)
	iovsCount := w.readI32(e, seg, off, 4)

	// pIovs and pNwritten are far pointers
	pIovsSeg, pIovsOff := w.readPtr(e, seg, off, 8)
	pIovs := mem.Lin(pIovsSeg, pIovsOff)

	pNwrittenSeg, pNwrittenOff := w.readPtr(e, seg, off, 12)
	pNwritten := mem.Lin(pNwrittenSeg, pNwrittenOff)

	// Handle console output (fd 1, 2)
	if fd == 1 || fd == 2 {
		totalWritten := int32(0)
		for i := 0; i < int(iovsCount); i++ {
			// Read iovec: buf (4 bytes), buf_len (4 bytes)
			iovAddr := pIovs + uint32(i*8)
			bufSeg, bufOff := w.readPtr(e, uint16(seg), uint16(iovAddr), 0)
			bufLen := int32(binary.LittleEndian.Uint32(e.Mem.Bytes(iovAddr+4, 4)))

			// Read data and write to console
			bufAddr := mem.Lin(bufSeg, bufOff)
			_ = e.Mem.Bytes(bufAddr, int(bufLen)) // data is read but not used in v0

			// Convert UTF-8 to screen codepage and output
			// For now, just count bytes (will be improved in full implementation)
			totalWritten += bufLen
		}

		w.writeI32(e, pNwritten, totalWritten)
		w.setResult(e, ESUCCESS)
		return nil
	}

	w.setResult(e, EBADF)
	return nil
}

// fdSeek: seek in fd (placeholder)
func (w *WasiOS) fdSeek(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// fdTell: get current offset in fd (placeholder)
func (w *WasiOS) fdTell(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// fdClose: close fd (placeholder)
func (w *WasiOS) fdClose(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// fdFilestatGet: get file stats (placeholder)
func (w *WasiOS) fdFilestatGet(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// pathOpen: open a file (placeholder)
func (w *WasiOS) pathOpen(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// pathFilestatGet: get file stats by path (placeholder)
func (w *WasiOS) pathFilestatGet(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// pathCreateDirectory: create directory (placeholder)
func (w *WasiOS) pathCreateDirectory(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// pathRemoveDirectory: remove directory (placeholder)
func (w *WasiOS) pathRemoveDirectory(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// pathUnlinkFile: delete file (placeholder)
func (w *WasiOS) pathUnlinkFile(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// pathRename: rename file (placeholder)
func (w *WasiOS) pathRename(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// fdReaddir: read directory (placeholder)
func (w *WasiOS) fdReaddir(e *hle.Env) error {
	w.setResult(e, ENOSYS)
	return nil
}

// --- W5 Functions: Scheduling ---

// pollOneoff: wait for events (W5, clock subscriptions only for v0)
func (w *WasiOS) pollOneoff(e *hle.Env) error {
	// W5: poll_oneoff with clock subscriptions only
	// For v0, just return 0 events immediately
	w.setResult(e, ENOSYS)
	return nil
}
