package dos

import (
	"bytes"
	"testing"
	"time"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// TestAMISInstallationCheck tests INT 2Dh AL=00h (installation check).
func TestAMISInstallationCheck(t *testing.T) {
	e, d := setupDOS(t)

	// Test free slot (multiplex 255 - should be free since we registered at 0)
	e.CPU.R[cpu.AX] = 0xFF00 // AH=FFh (unlikely multiplex), AL=00h
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	// Should return AL=00 (free), no change to other registers
	if al := e.CPU.AL(); al != 0x00 {
		t.Errorf("AL = %02Xh, want 00h for free slot", al)
	}

	// Test occupied slot (multiplex 0 - UTF8NAMES provider)
	e.CPU.R[cpu.AX] = 0x0000 // AH=00h, AL=00h
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	// Should return AL=FFh, CX=version
	if al := e.CPU.AL(); al != 0xFF {
		t.Errorf("AL = %02Xh, want FFh for occupied slot", al)
	}
	if cx := e.CPU.R[cpu.CX]; cx != 0x0100 {
		t.Errorf("CX = %04Xh, want 0100h (version 1.0)", cx)
	}
	// Check that DX:DI points to a valid signature
	di := e.CPU.R[cpu.DI]
	dx := e.CPU.R[cpu.DX]
	// Signature should be readable from memory
	sig := e.Mem.Bytes(mem.Lin(dx, di), 16)
	if !bytes.Equal(sig[0:8], []byte("DOS-UTF8")) {
		t.Errorf("signature mfr = %q, want 'DOS-UTF8'", sig[0:8])
	}
	if !bytes.Equal(sig[8:16], []byte("NAMES   ")) {
		t.Errorf("signature product = %q, want 'NAMES   '", sig[8:16])
	}
}

// TestAMISDirectEntry tests INT 2Dh AL=01h (direct entry, optional).
func TestAMISDirectEntry(t *testing.T) {
	e, d := setupDOS(t)

	// Direct entry for occupied slot should return AL=00h (not supported)
	e.CPU.R[cpu.AX] = 0x0001 // AH=00h, AL=01h
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	if al := e.CPU.AL(); al != 0x00 {
		t.Errorf("AL = %02Xh, want 00h for direct entry", al)
	}
}

// TestAMISUninstall tests INT 2Dh AL=02h (uninstall request).
func TestAMISUninstall(t *testing.T) {
	e, d := setupDOS(t)

	// Uninstall request for occupied slot should return AL=03h (cannot uninstall)
	e.CPU.R[cpu.AX] = 0x0002 // AH=00h, AL=02h
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	if al := e.CPU.AL(); al != 0x03 {
		t.Errorf("AL = %02Xh, want 03h (cannot uninstall)", al)
	}
}

// TestAMISInterruptList tests INT 2Dh AL=04h (list of intercepted interrupts).
func TestAMISInterruptList(t *testing.T) {
	e, d := setupDOS(t)

	// List request for occupied slot should return a list ending with 0x2Dh
	e.CPU.R[cpu.AX] = 0x0004     // AH=00h, AL=04h
	e.CPU.SetSeg(cpu.DS, 0x0070) // kernel data segment
	e.CPU.R[cpu.DX] = 0x0100     // offset in DS
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	if al := e.CPU.AL(); al != 0x04 {
		t.Errorf("AL = %02Xh, want 04h for list", al)
	}
	// Check that the list contains 2Dh at the start
	list := e.Mem.Bytes(mem.Lin(0x0070, 0x0100), 3)
	if list[0] != 0x2D {
		t.Errorf("list[0] = %02Xh, want 2Dh", list[0])
	}
}

// TestAMISMultipleProviders tests registering and finding multiple AMIS providers.
func TestAMISMultipleProviders(t *testing.T) {
	e, d := setupDOS(t)

	// Check the first provider (UTF8NAMES at slot 0)
	e.CPU.R[cpu.AX] = 0x0000 // AH=00h, AL=00h
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	if al := e.CPU.AL(); al != 0xFF {
		t.Errorf("provider 0: AL = %02Xh, want FFh", al)
	}

	// Try an unregistered slot
	e.CPU.R[cpu.AX] = 0x0500 // AH=05h, AL=00h
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	if al := e.CPU.AL(); al != 0x00 {
		t.Errorf("provider 5: AL = %02Xh, want 00h (free)", al)
	}

	// Register a new provider
	var hostmfr [8]byte
	var hostprod [8]byte
	copy(hostmfr[:], []byte("DOS-HOST"))
	copy(hostprod[:], []byte("HOSTEXEC"))
	slot := d.registerAMISProvider(hostmfr, hostprod, "Host command execution")
	if slot != 1 {
		t.Errorf("registerAMISProvider returned slot %d, want 1", slot)
	}

	// Now check that slot 1 is occupied
	e.CPU.R[cpu.AX] = 0x0100 // AH=01h, AL=00h
	if err := d.int2D(e); err != nil {
		t.Fatalf("int2D failed: %v", err)
	}
	if al := e.CPU.AL(); al != 0xFF {
		t.Errorf("provider 1: AL = %02Xh, want FFh", al)
	}
	// Check signature
	di := e.CPU.R[cpu.DI]
	dx := e.CPU.R[cpu.DX]
	sig := e.Mem.Bytes(mem.Lin(dx, di), 16)
	if !bytes.Equal(sig[0:8], []byte("DOS-HOST")) {
		t.Errorf("signature mfr = %q, want 'DOS-HOST'", sig[0:8])
	}
	if !bytes.Equal(sig[8:16], []byte("HOSTEXEC")) {
		t.Errorf("signature product = %q, want 'HOSTEXEC'", sig[8:16])
	}
}

// setupDOS creates a DOS kernel for testing.
func setupDOS(t *testing.T) (*hle.Env, *DOS) {
	t.Helper()
	dir := t.TempDir()

	m := mem.New()
	c := cpu.New(m, nil)
	page, err := cp.Get(437)
	if err != nil {
		t.Fatalf("failed to get codepage: %v", err)
	}
	e := hle.New(c, m, page, time.Now, hle.NewTracer(0, nil, nil))

	b := bios.New(e)
	d, err := New(e, b, Config{Drives: map[byte]string{'C': dir}})
	if err != nil {
		t.Fatalf("failed to create DOS: %v", err)
	}

	return e, d
}
