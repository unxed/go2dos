package wasidos

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// setupTestEnv creates a test HLE environment.
func setupTestEnv(t *testing.T) *hle.Env {
	memory := mem.New()
	cpuInst := cpu.New(memory, nil) // Use nil for Ports since we're just testing WASI
	e := &hle.Env{
		CPU: cpuInst,
		Mem: memory,
		Now: time.Now,
	}
	return e
}

// writeParamBlock writes a parameter block at DS:SI.
func writeParamBlock(e *hle.Env, seg uint16, off uint16, data []byte) {
	addr := mem.Lin(seg, off)
	e.Mem.SetBytes(addr, data)
}

// TestArgsSizesGet tests the args_sizes_get function.
func TestArgsSizesGet(t *testing.T) {
	e := setupTestEnv(t)
	w, err := NewWasiOS(e, nil, 0x2D)
	if err != nil {
		t.Fatalf("NewWasiOS failed: %v", err)
	}

	// Set up parameter block at DS:SI
	// Two i32 outputs: argc and argv_buf_size
	e.CPU.S[cpu.DS].Sel = 0x1000
	e.CPU.R[cpu.SI] = 0x0100

	// Call argsSizesGet
	err = w.argsSizesGet(e)
	if err != nil {
		t.Fatalf("argsSizesGet failed: %v", err)
	}

	// Check return value (should be ESUCCESS = 0)
	if e.CPU.R[cpu.AX] != ESUCCESS {
		t.Errorf("Expected AX=%d, got %d", ESUCCESS, e.CPU.R[cpu.AX])
	}

	// Read results from memory
	addr := mem.Lin(0x1000, 0x0100)
	argc := int32(binary.LittleEndian.Uint32(e.Mem.Bytes(addr, 4)))
	argvSize := int32(binary.LittleEndian.Uint32(e.Mem.Bytes(addr+4, 4)))

	if argc != 0 || argvSize != 0 {
		t.Errorf("Expected argc=0, argv_size=0, got argc=%d, argv_size=%d", argc, argvSize)
	}
}

// TestEnvironSizesGet tests the environ_sizes_get function.
func TestEnvironSizesGet(t *testing.T) {
	e := setupTestEnv(t)
	w, err := NewWasiOS(e, nil, 0x2D)
	if err != nil {
		t.Fatalf("NewWasiOS failed: %v", err)
	}

	e.CPU.S[cpu.DS].Sel = 0x1000
	e.CPU.R[cpu.SI] = 0x0100

	err = w.environSizesGet(e)
	if err != nil {
		t.Fatalf("environSizesGet failed: %v", err)
	}

	if e.CPU.R[cpu.AX] != ESUCCESS {
		t.Errorf("Expected AX=%d, got %d", ESUCCESS, e.CPU.R[cpu.AX])
	}

	// Read results from memory
	addr := mem.Lin(0x1000, 0x0100)
	envc := int32(binary.LittleEndian.Uint32(e.Mem.Bytes(addr, 4)))
	envSize := int32(binary.LittleEndian.Uint32(e.Mem.Bytes(addr+4, 4)))

	if envc != 0 || envSize != 0 {
		t.Errorf("Expected envc=0, env_size=0, got envc=%d, env_size=%d", envc, envSize)
	}
}

// TestClockTimeGet tests the clock_time_get function.
func TestClockTimeGet(t *testing.T) {
	e := setupTestEnv(t)
	w, err := NewWasiOS(e, nil, 0x2D)
	if err != nil {
		t.Fatalf("NewWasiOS failed: %v", err)
	}

	e.CPU.S[cpu.DS].Sel = 0x1000
	e.CPU.R[cpu.SI] = 0x0100

	// Parameter block: clock_id (i32) + precision (i64) + output pointer (4 bytes far ptr)
	// clock_id = 0 (CLOCK_REALTIME), precision = 1000000000 (1 second)
	params := make([]byte, 16)
	binary.LittleEndian.PutUint32(params[0:4], 0)          // clock_id
	binary.LittleEndian.PutUint64(params[4:12], 1000000000) // precision
	// Point to output buffer at 0x1000:0x0200 (far ptr: offset:segment)
	binary.LittleEndian.PutUint16(params[12:14], 0x0200)   // offset
	binary.LittleEndian.PutUint16(params[14:16], 0x1000)   // segment
	writeParamBlock(e, 0x1000, 0x0100, params)

	err = w.clockTimeGet(e)
	if err != nil {
		t.Fatalf("clockTimeGet failed: %v", err)
	}

	if e.CPU.R[cpu.AX] != ESUCCESS {
		t.Errorf("Expected AX=%d, got %d", ESUCCESS, e.CPU.R[cpu.AX])
	}

	// Read time from output
	outputAddr := mem.Lin(0x1000, 0x0200)
	timeBytes := e.Mem.Bytes(outputAddr, 8)
	timeValue := int64(binary.LittleEndian.Uint64(timeBytes))

	if timeValue == 0 {
		t.Error("Expected non-zero time value")
	}
}

// TestFdPrestatGet tests the fd_prestat_get function.
func TestFdPrestatGet(t *testing.T) {
	e := setupTestEnv(t)
	w, err := NewWasiOS(e, nil, 0x2D)
	if err != nil {
		t.Fatalf("NewWasiOS failed: %v", err)
	}

	e.CPU.S[cpu.DS].Sel = 0x1000
	e.CPU.R[cpu.SI] = 0x0100

	// Parameter block: fd (i32) + output buffer pointer (far ptr: offset:segment)
	params := make([]byte, 8)
	binary.LittleEndian.PutUint32(params[0:4], 3)     // fd for C: drive
	// Far pointer: offset 0x0200, segment 0x1000 (stored as offset:segment in little-endian)
	binary.LittleEndian.PutUint16(params[4:6], 0x0200)   // offset
	binary.LittleEndian.PutUint16(params[6:8], 0x1000)   // segment
	writeParamBlock(e, 0x1000, 0x0100, params)

	err = w.fdPrestatGet(e)
	if err != nil {
		t.Fatalf("fdPrestatGet failed: %v", err)
	}

	if e.CPU.R[cpu.AX] != ESUCCESS {
		t.Errorf("Expected AX=%d, got %d", ESUCCESS, e.CPU.R[cpu.AX])
	}

	// Read prestat structure from output
	outputAddr := mem.Lin(0x1000, 0x0200)
	tagBytes := e.Mem.Bytes(outputAddr, 1)
	tag := tagBytes[0]
	if tag != 3 {
		t.Errorf("Expected tag=3 (dir), got %d", tag)
	}

	nameLen := int32(binary.LittleEndian.Uint32(e.Mem.Bytes(outputAddr+4, 4)))
	if nameLen != 2 {
		t.Errorf("Expected name_len=2 (C:), got %d", nameLen)
	}
}

// TestFdWrite tests the fd_write function.
func TestFdWrite(t *testing.T) {
	e := setupTestEnv(t)
	w, err := NewWasiOS(e, nil, 0x2D)
	if err != nil {
		t.Fatalf("NewWasiOS failed: %v", err)
	}

	e.CPU.S[cpu.DS].Sel = 0x1000
	e.CPU.R[cpu.SI] = 0x0100

	// Parameter block: fd (i32), iovs_count (i32), iovs ptr (far), nwritten ptr (far)
	params := make([]byte, 16)
	binary.LittleEndian.PutUint32(params[0:4], 1) // fd = stdout
	binary.LittleEndian.PutUint32(params[4:8], 1) // iovs_count = 1

	// Far pointers to iovec and nwritten output
	// iovs at 0x1000:0x0200
	binary.LittleEndian.PutUint16(params[8:10], 0x0200)    // offset
	binary.LittleEndian.PutUint16(params[10:12], 0x1000)   // segment
	// nwritten at 0x1000:0x0210
	binary.LittleEndian.PutUint16(params[12:14], 0x0210)   // offset
	binary.LittleEndian.PutUint16(params[14:16], 0x1000)   // segment

	writeParamBlock(e, 0x1000, 0x0100, params)

	// Create iovec at 0x1000:0x0200
	iov := make([]byte, 8)
	// Data at 0x1000:0x0300
	binary.LittleEndian.PutUint16(iov[0:2], 0x0300)    // buf offset
	binary.LittleEndian.PutUint16(iov[2:4], 0x1000)    // buf segment
	binary.LittleEndian.PutUint32(iov[4:8], 5)         // buf_len = 5

	e.Mem.SetBytes(mem.Lin(0x1000, 0x0200), iov)

	// Write "hello" at data address
	e.Mem.SetBytes(mem.Lin(0x1000, 0x0300), []byte("hello"))

	err = w.fdWrite(e)
	if err != nil {
		t.Fatalf("fdWrite failed: %v", err)
	}

	if e.CPU.R[cpu.AX] != ESUCCESS {
		t.Errorf("Expected AX=%d, got %d", ESUCCESS, e.CPU.R[cpu.AX])
	}

	// Read nwritten from output at 0x1000:0x0210
	nwritten := int32(binary.LittleEndian.Uint32(e.Mem.Bytes(mem.Lin(0x1000, 0x0210), 4)))
	if nwritten != 5 {
		t.Errorf("Expected nwritten=5, got %d", nwritten)
	}
}

// TestSchedYield tests the sched_yield function.
func TestSchedYield(t *testing.T) {
	e := setupTestEnv(t)
	w, err := NewWasiOS(e, nil, 0x2D)
	if err != nil {
		t.Fatalf("NewWasiOS failed: %v", err)
	}

	idleCalled := false
	e.Idle = func() {
		idleCalled = true
	}

	err = w.schedYield(e)
	if err != nil {
		t.Fatalf("schedYield failed: %v", err)
	}

	if !idleCalled {
		t.Error("Expected Idle() to be called")
	}

	if e.CPU.R[cpu.AX] != ESUCCESS {
		t.Errorf("Expected AX=%d, got %d", ESUCCESS, e.CPU.R[cpu.AX])
	}
}
