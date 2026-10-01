// Package dos implements a high-level-emulated DOS kernel (INT 20h/21h/2Fh
// and friends) on top of host directories.
package dos

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Memory layout of the emulated kernel.
const (
	dataSeg   = 0x0070 // kernel data
	firstMCB  = 0x0150
	memTop    = 0xA000 // end of conventional memory
	offInDOS  = 0x0000
	offDBCS   = 0x0010
	offUpper  = 0x0020 // 6502h table
	offFUpper = 0x00B0 // 6504h table
	offSFT    = 0x01E0
	offLoL    = 0x0200
	offCDS    = 0x0300
	cdsSize   = 0x58
	lastDrive = 26
)

// Version reported by INT 21h/30h.
const (
	VersionMajor = 5
	VersionMinor = 0
)

// Config configures the kernel.
type Config struct {
	Drives  map[byte]string // drive letter (A-Z) -> host directory
	Current byte            // current drive letter
	Env     []string        // environment variables (NAME=VALUE)
	// Labels maps drive letters to volume labels (up to 11 characters).
	// A host directory has no label, so without an entry the drive reports
	// "NO NAME" and FindFirst with attribute 08h finds nothing.
	Labels map[byte]string
}

// DOS is the kernel state.
type DOS struct {
	e              *hle.Env
	b              *bios.BIOS
	fs             *fsys
	sft            []*openFile
	psp            uint16 // current PSP
	dta            uint32 // linear address of the current DTA
	dtaSeg, dtaOff uint16
	retf           uint16
	env            []string
	exit           byte // last return code (INT 21h/4Dh)

	frames   []parentFrame // EXEC nesting
	exitType byte

	lastErr   uint16
	breakFlag byte
	verify    byte
	strategy  uint16
	pendScan  byte // second byte of an extended key for character input
	line      []byte
	lineDone  bool
	conIn     []byte // cooked CON input not yet consumed by read
}

// New installs the kernel.
func New(e *hle.Env, b *bios.BIOS, cfg Config) (*DOS, error) {
	d := &DOS{e: e, b: b, env: cfg.Env, breakFlag: 0}
	fs, err := newFS(e, cfg)
	if err != nil {
		return nil, err
	}
	d.fs = fs
	m := e.Mem

	// MCB chain: one free block covering the rest of conventional memory.
	d.writeMCB(firstMCB, 'Z', 0, memTop-firstMCB-1, "")

	// List of lists (DOS 4+ layout, minimal), first MCB word just before it.
	lol := mem.Lin(dataSeg, offLoL)
	m.W16(lol-2, firstMCB)
	m.W32(lol+0x00, 0xFFFFFFFF)                 // DPB chain
	m.W32(lol+0x04, uint32(dataSeg)<<16|offSFT) // SFT chain
	m.W32(lol+0x16, uint32(dataSeg)<<16|offCDS) // CDS
	m.W32(lol+0x08, 0xFFFFFFFF)                 // CLOCK$
	m.W32(lol+0x0C, 0xFFFFFFFF)                 // CON
	m.W16(lol+0x10, 512)                        // max bytes per sector
	m.W8(lol+0x20, byte(len(cfg.Drives)))       // block devices
	m.W8(lol+0x21, lastDrive)                   // LASTDRIVE
	m.W32(lol+0x22, 0xFFFFFFFF)                 // NUL device header: next
	m.W16(lol+0x26, 0x8004)                     // attributes: char device, NUL
	m.SetBytes(lol+0x2C, []byte("NUL     "))    //
	m.W32(mem.Lin(dataSeg, offSFT), 0xFFFFFFFF) // empty SFT block
	m.W16(mem.Lin(dataSeg, offSFT)+4, 0)        //
	d.writeCDS()
	// Upper-case tables for INT 21h/65h (size word + 128 bytes).
	for _, off := range []uint16{offUpper, offFUpper} {
		a := mem.Lin(dataSeg, off)
		m.W16(a, 128)
		for i := 0; i < 128; i++ {
			m.W8(a+2+uint32(i), e.CP.Upper(byte(0x80+i)))
		}
	}

	d.sft = []*openFile{{dev: devCON, refs: 3}, {dev: devNUL, name: "AUX", refs: 1}, {dev: devNUL, name: "PRN", refs: 1}}
	d.strategy = 0

	e.HookInt(0x20, "int20", d.int20)
	e.HookInt(0x21, "int21", d.int21)
	e.HookInt(0x22, "int22", func(e *hle.Env) error { return hle.Unsupported("INT 22h called directly") })
	e.HookInt(0x23, "int23", d.int23)
	e.HookInt(0x24, "int24", func(e *hle.Env) error { e.CPU.SetAL(3); return nil })
	e.HookInt(0x25, "int25", func(e *hle.Env) error { return hle.Unsupported("INT 25h absolute disk read") })
	e.HookInt(0x26, "int26", func(e *hle.Env) error { return hle.Unsupported("INT 26h absolute disk write") })
	e.HookInt(0x27, "int27", d.int27)
	e.HookInt(0x28, "int28", func(e *hle.Env) error { e.Idle(); return nil })
	e.HookInt(0x29, "int29", func(e *hle.Env) error { d.conWrite([]byte{e.CPU.AL()}); return nil })
	e.HookInt(0x2F, "int2F", d.int2F)
	iret := e.Emit([]byte{0xCF})
	e.SetVector(0x2A, hle.ROMSeg, iret)
	return d, nil
}

func (d *DOS) writeMCB(seg uint16, sig byte, owner, size uint16, name string) {
	a := mem.Lin(seg, 0)
	m := d.e.Mem
	m.W8(a, sig)
	m.W16(a+1, owner)
	m.W16(a+3, size)
	n := make([]byte, 8)
	copy(n, name)
	m.SetBytes(a+8, n)
}

func (d *DOS) writeCDS() {
	m := d.e.Mem
	for i := 0; i < lastDrive; i++ {
		a := mem.Lin(dataSeg, offCDS) + uint32(i*cdsSize)
		for j := 0; j < cdsSize; j++ {
			m.W8(a+uint32(j), 0)
		}
		p := fmt.Sprintf("%c:\\", 'A'+i)
		m.SetBytes(a, []byte(p))
		if d.fs.drives[i] != "" {
			m.W16(a+0x43, 0x4000) // physical drive
		}
		m.W32(a+0x45, 0xFFFFFFFF)
		m.W16(a+0x49, 0xFFFF)
		m.W32(a+0x4B, 0xFFFFFFFF)
		m.W16(a+0x4F, 2)
	}
}

// --- memory -----------------------------------------------------------------

type mcb struct {
	seg   uint16
	sig   byte
	owner uint16
	size  uint16
}

func (d *DOS) readMCB(seg uint16) mcb {
	a := mem.Lin(seg, 0)
	m := d.e.Mem
	return mcb{seg, m.R8(a), m.R16(a + 1), m.R16(a + 3)}
}

// chain walks the MCBs; it returns an error on a corrupted chain.
func (d *DOS) chain() ([]mcb, error) {
	var out []mcb
	seg := uint16(firstMCB)
	for i := 0; i < 0x10000; i++ {
		b := d.readMCB(seg)
		if b.sig != 'M' && b.sig != 'Z' {
			return out, fmt.Errorf("MCB chain corrupted at %04X", seg)
		}
		out = append(out, b)
		if b.sig == 'Z' {
			return out, nil
		}
		seg += b.size + 1
	}
	return out, fmt.Errorf("MCB chain loops")
}

// coalesce merges adjacent free blocks.
func (d *DOS) coalesce() error {
	for {
		blocks, err := d.chain()
		if err != nil {
			return err
		}
		merged := false
		for i := 0; i+1 < len(blocks); i++ {
			a, b := blocks[i], blocks[i+1]
			if a.owner == 0 && b.owner == 0 {
				d.writeMCB(a.seg, b.sig, 0, a.size+b.size+1, "")
				merged = true
				break
			}
		}
		if !merged {
			return nil
		}
	}
}

// alloc allocates paras paragraphs; on failure it returns the largest free block.
func (d *DOS) alloc(paras uint16, owner uint16) (seg uint16, largest uint16, errc uint16) {
	if err := d.coalesce(); err != nil {
		return 0, 0, 7
	}
	blocks, _ := d.chain()
	var best *mcb
	for i := range blocks {
		b := &blocks[i]
		if b.owner != 0 {
			continue
		}
		if b.size > largest {
			largest = b.size
		}
		if b.size >= paras {
			switch d.strategy & 3 {
			case 1: // best fit
				if best == nil || b.size < best.size {
					best = b
				}
			case 2: // last fit
				best = b
			default:
				if best == nil {
					best = b
				}
			}
		}
	}
	if best == nil {
		return 0, largest, 8
	}
	if best.size > paras {
		rest := best.seg + paras + 1
		d.writeMCB(rest, best.sig, 0, best.size-paras-1, "")
		d.writeMCB(best.seg, 'M', owner, paras, "")
	} else {
		d.writeMCB(best.seg, best.sig, owner, paras, "")
	}
	return best.seg + 1, 0, 0
}

func (d *DOS) free(seg uint16) uint16 {
	b := d.readMCB(seg - 1)
	if b.sig != 'M' && b.sig != 'Z' {
		return 9
	}
	d.writeMCB(seg-1, b.sig, 0, b.size, "")
	return 0
}

func (d *DOS) resize(seg, paras uint16) (largest uint16, errc uint16) {
	b := d.readMCB(seg - 1)
	if b.sig != 'M' && b.sig != 'Z' {
		return 0, 9
	}
	if paras <= b.size {
		if paras < b.size {
			rest := seg + paras
			d.writeMCB(rest, b.sig, 0, b.size-paras-1, "")
			d.writeMCB(seg-1, 'M', b.owner, paras, d.mcbName(seg-1))
		}
		return 0, d.coalesceErr()
	}
	// Grow: absorb the following free block if it is large enough.
	if err := d.coalesce(); err != nil {
		return 0, 7
	}
	b = d.readMCB(seg - 1)
	avail := b.size
	if b.sig == 'M' {
		n := d.readMCB(seg + b.size)
		if n.owner == 0 {
			avail = b.size + n.size + 1
			if avail >= paras {
				d.writeMCB(seg-1, n.sig, b.owner, avail, d.mcbName(seg-1))
				return d.resize(seg, paras)
			}
		}
	}
	return avail, 8
}

func (d *DOS) coalesceErr() uint16 {
	if d.coalesce() != nil {
		return 7
	}
	return 0
}

func (d *DOS) mcbName(seg uint16) string {
	return strings.TrimRight(string(d.e.Mem.Bytes(mem.Lin(seg, 8), 8)), "\x00")
}

func (d *DOS) freeOwnedBy(psp uint16) {
	blocks, err := d.chain()
	if err != nil {
		return
	}
	for _, b := range blocks {
		if b.owner == psp {
			d.writeMCB(b.seg, b.sig, 0, b.size, "")
		}
	}
	d.coalesce()
}

// --- loading ------------------------------------------------------------------

// Program describes a loaded program.
type Program struct {
	PSP   uint16
	Start [2]uint16 // CS:IP
	Stack [2]uint16 // SS:SP
}

// Load loads a program given as a DOS path (e.g. C:\VC.COM) with a command
// tail, and points the CPU at its entry.
func (d *DOS) Load(path string, tail string) (*Program, error) {
	drive, dpath, errc := d.fs.canon([]byte(path), false)
	if errc != 0 {
		return nil, fmt.Errorf("bad program path %q: %s", path, errText(errc))
	}
	host, _, errc := d.fs.resolve(drive, dpath, false)
	if errc != 0 {
		return nil, fmt.Errorf("program %s: %s", path, errText(errc))
	}
	image, err := d.fs.readFile(host)
	if err != nil {
		return nil, err
	}
	full := fmt.Sprintf("%c:%s", 'A'+drive, dpath)

	// Environment block.
	var env []byte
	for _, v := range d.env {
		b, _ := d.e.CP.Encode(v)
		env = append(env, b...)
		env = append(env, 0)
	}
	env = append(env, 0, 1, 0)
	env = append(env, []byte(full)...)
	env = append(env, 0)
	envSeg, _, errc := d.alloc(uint16((len(env)+15)/16), 0xFFFF)
	if errc != 0 {
		return nil, fmt.Errorf("no memory for the environment")
	}
	d.e.Mem.SetBytes(mem.Lin(envSeg, 0), env)

	isEXE := len(image) >= 0x1C && (string(image[:2]) == "MZ" || string(image[:2]) == "ZM")
	var hdr exeHeader
	need := uint16(0x1000) // COM: take everything
	if isEXE {
		hdr = parseEXE(image)
		need = hdr.loadParas() + 0x10 + hdr.MinAlloc
	}
	_, largest, _ := d.alloc(0xFFFF, 0)
	if largest < need {
		return nil, fmt.Errorf("not enough memory: need %d paragraphs, have %d", need, largest)
	}
	size := largest
	if isEXE && hdr.MaxAlloc != 0xFFFF {
		want := hdr.loadParas() + 0x10 + hdr.MaxAlloc
		if want < size && want >= need {
			size = want
		}
	}
	psp, _, errc := d.alloc(size, 0xFFFF)
	if errc != 0 {
		return nil, fmt.Errorf("allocation failed (%s)", errText(errc))
	}
	d.writeMCB(psp-1, d.readMCB(psp-1).sig, psp, size, baseName(dpath))
	d.writeMCB(envSeg-1, d.readMCB(envSeg-1).sig, psp, d.readMCB(envSeg-1).size, "")

	d.buildPSP(psp, psp+size, envSeg, psp, tail)
	d.psp = psp
	d.setDTA(psp, 0x80)
	prog := &Program{PSP: psp}
	c := d.e.CPU
	if isEXE {
		start := psp + 0x10
		body := image[hdr.HeaderParas*16:]
		if n := int(hdr.loadParas()) * 16; n < len(body) {
			body = body[:n]
		}
		d.e.Mem.SetBytes(mem.Lin(start, 0), body)
		for i := 0; i < int(hdr.NReloc); i++ {
			o := int(hdr.RelocOff) + i*4
			if o+4 > len(image) {
				return nil, fmt.Errorf("EXE relocation table truncated")
			}
			off := binary.LittleEndian.Uint16(image[o:])
			seg := binary.LittleEndian.Uint16(image[o+2:])
			a := mem.Lin(start+seg, off)
			d.e.Mem.W16(a, d.e.Mem.R16(a)+start)
		}
		prog.Start = [2]uint16{start + hdr.CS, hdr.IP}
		prog.Stack = [2]uint16{start + hdr.SS, hdr.SP}
	} else {
		if len(image) > 0xFF00 {
			return nil, fmt.Errorf("COM file too large (%d bytes)", len(image))
		}
		d.e.Mem.SetBytes(mem.Lin(psp, 0x100), image)
		sp := uint16(0xFFFE)
		if uint32(size)*16 < 0x10000 {
			sp = uint16(uint32(size)*16 - 2)
		}
		d.e.Mem.W16(mem.Lin(psp, sp), 0)
		prog.Start = [2]uint16{psp, 0x100}
		prog.Stack = [2]uint16{psp, sp}
	}
	c.SetSeg(cpu.CS, prog.Start[0])
	c.IP = prog.Start[1]
	c.SetSeg(cpu.SS, prog.Stack[0])
	c.R[cpu.SP] = prog.Stack[1]
	c.SetSeg(cpu.DS, psp)
	c.SetSeg(cpu.ES, psp)
	c.R[cpu.AX] = d.fcbDriveStatus(psp)
	c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX] = 0, uint16(len(image)), psp
	c.R[cpu.SI], c.R[cpu.DI], c.R[cpu.BP] = prog.Start[1], prog.Stack[1], 0x091C
	c.SetFlags(cpu.FlagIF)
	return prog, nil
}

func baseName(p string) string {
	if i := strings.LastIndexByte(p, '\\'); i >= 0 {
		p = p[i+1:]
	}
	if i := strings.IndexByte(p, '.'); i >= 0 {
		p = p[:i]
	}
	return p
}

type exeHeader struct {
	LastPage, Pages, NReloc, HeaderParas, MinAlloc, MaxAlloc uint16
	SS, SP, Checksum, IP, CS, RelocOff                       uint16
}

func parseEXE(b []byte) exeHeader {
	w := func(o int) uint16 { return binary.LittleEndian.Uint16(b[o:]) }
	return exeHeader{w(2), w(4), w(6), w(8), w(0x0A), w(0x0C), w(0x0E), w(0x10), w(0x12), w(0x14), w(0x16), w(0x18)}
}

func (h exeHeader) loadParas() uint16 {
	size := uint32(h.Pages) * 512
	if h.LastPage != 0 {
		size -= 512 - uint32(h.LastPage)
	}
	size -= uint32(h.HeaderParas) * 16
	return uint16((size + 15) / 16)
}

func (d *DOS) buildPSP(psp, top, envSeg, parent uint16, tail string) {
	m := d.e.Mem
	a := mem.Lin(psp, 0)
	for i := uint32(0); i < 0x100; i++ {
		m.W8(a+i, 0)
	}
	m.SetBytes(a, []byte{0xCD, 0x20})
	m.W16(a+2, top)
	m.SetBytes(a+5, []byte{0x9A, 0xF0, 0xFE, 0x1D, 0xF0})
	for i, v := range []byte{0x22, 0x23, 0x24} {
		seg, off := d.e.Vector(v)
		m.W16(a+0x0A+uint32(i*4), off)
		m.W16(a+0x0C+uint32(i*4), seg)
	}
	m.W16(a+0x16, parent)
	jft := []byte{0, 0, 0, 1, 2}
	for i := 0; i < 20; i++ {
		v := byte(0xFF)
		if i < len(jft) {
			v = jft[i]
		}
		m.W8(a+0x18+uint32(i), v)
	}
	m.W16(a+0x2C, envSeg)
	m.W16(a+0x32, 20)
	m.W32(a+0x34, uint32(psp)<<16|0x18)
	m.W32(a+0x38, 0xFFFFFFFF)
	m.SetBytes(a+0x50, []byte{0xCD, 0x21, 0xCB})
	t, _ := d.e.CP.Encode(tail)
	if len(t) > 126 {
		t = t[:126]
	}
	m.W8(a+0x80, byte(len(t)))
	m.SetBytes(a+0x81, t)
	m.W8(a+0x81+uint32(len(t)), 0x0D)
	// FCBs from the first two arguments.
	args := strings.Fields(tail)
	for i, off := range []uint32{0x5C, 0x6C} {
		for j := uint32(0); j < 16; j++ {
			m.W8(a+off+j, 0)
		}
		m.SetBytes(a+off+1, []byte("           "))
		if i < len(args) {
			b, _ := d.e.CP.Encode(args[i])
			d.parseFCB(b, a+off, 0)
		}
	}
}

func (d *DOS) fcbDriveStatus(psp uint16) uint16 {
	var r uint16
	for i, off := range []uint32{0x5C, 0x6C} {
		drv := d.e.Mem.R8(mem.Lin(psp, 0) + off)
		if drv != 0 && d.fs.drives[drv-1] == "" {
			r |= 0xFF << (8 * i)
		}
	}
	return r
}

// terminate ends the current program: a child returns to its parent, the
// top-level program stops the machine.
func (d *DOS) terminate(code byte, keep bool) {
	d.exit = code
	d.exitType = 0
	if keep {
		d.exitType = 3
	}
	if d.endChild(code, keep) {
		return
	}
	if !keep {
		d.closeAll(d.psp)
		d.freeOwnedBy(d.psp)
	}
	d.e.Stop = &hle.Exit{Code: code}
}

func (d *DOS) int20(e *hle.Env) error { d.terminate(0, false); return nil }

func (d *DOS) int23(e *hle.Env) error { d.terminate(0, false); return nil }

func (d *DOS) int27(e *hle.Env) error { d.terminate(0, true); return nil }

func (d *DOS) setDTA(seg, off uint16) {
	d.dtaSeg, d.dtaOff = seg, off
	d.dta = mem.Lin(seg, off)
}
