package dos

import (
	"testing"
	"time"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// TestWinOldApClipboard tests the WinOldAp clipboard API (INT 2Fh/17xx).
func TestWinOldApClipboard(t *testing.T) {
	m := mem.New()
	c := cpu.New(m, nil)
	page, err := cp.Get(437)
	if err != nil {
		t.Fatal(err)
	}
	tr := hle.NewTracer(0, nil, nil)
	env := hle.New(c, m, page, time.Now, tr)
	b := bios.New(env)
	d, err := New(env, b, Config{
		Drives: map[byte]string{'C': "."},
		Env:    []string{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test 1: Check installation (0x1700)
	c.R[cpu.AX] = 0x1700
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1700): %v", err)
	}
	if c.R[cpu.AX] != 0x1700 {
		t.Errorf("AX after 0x1700: want 0x1700, got 0x%04X", c.R[cpu.AX])
	}

	// Test 2: Open clipboard (0x1701)
	c.R[cpu.AX] = 0x1701
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1701): %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("AX after open: want 1, got %d", c.R[cpu.AX])
	}

	// Test 3: Set clipboard data (0x1703) with CF_OEMTEXT (7)
	msg := []byte("Hello clipboard\r\n\x00") // null-terminated
	msgData := msg[:len(msg)-1]                 // without null for verification
	// Write message to memory at DS:SI
	dssi := mem.Lin(0x200, 0x100)
	m.SetBytes(dssi, msg)

	c.R[cpu.AX] = 0x1703
	c.R[cpu.CX] = 7 // CF_OEMTEXT
	c.S[cpu.DS].Sel = 0x200
	c.R[cpu.SI] = 0x100
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1703): %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("AX after set: want 1, got %d", c.R[cpu.AX])
	}

	// Test 4: Get clipboard size (0x1704)
	c.R[cpu.AX] = 0x1704
	c.R[cpu.CX] = 7 // CF_OEMTEXT
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1704): %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("AX after size: want 1, got %d", c.R[cpu.AX])
	}
	if c.R[cpu.DX] != uint16(len(msgData)) {
		t.Errorf("DX (size): want %d, got %d", len(msgData), c.R[cpu.DX])
	}

	// Test 5: Get clipboard data (0x1705)
	c.R[cpu.AX] = 0x1705
	c.R[cpu.CX] = 7 // CF_OEMTEXT
	c.S[cpu.ES].Sel = 0x300
	c.R[cpu.DI] = 0x100
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1705): %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("AX after get: want 1, got %d", c.R[cpu.AX])
	}

	// Verify the data was copied correctly
	endi := mem.Lin(0x300, 0x100)
	retrieved := m.Bytes(endi, len(msgData))
	if string(retrieved) != string(msgData) {
		t.Errorf("Retrieved data: want %q, got %q", msgData, retrieved)
	}

	// Test 6: Close clipboard (0x1708)
	c.R[cpu.AX] = 0x1708
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1708): %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("AX after close: want 1, got %d", c.R[cpu.AX])
	}

	// Test 7: Operations on closed clipboard should fail
	c.R[cpu.AX] = 0x1704
	c.R[cpu.CX] = 7
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1704 when closed): %v", err)
	}
	if c.R[cpu.AX] != 0 {
		t.Errorf("AX when closed: want 0, got %d", c.R[cpu.AX])
	}

	// Test 8: Open again and test empty (0x1702)
	c.R[cpu.AX] = 0x1701
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1701) again: %v", err)
	}

	c.R[cpu.AX] = 0x1702 // Empty
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1702): %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("AX after empty: want 1, got %d", c.R[cpu.AX])
	}

	// Verify clipboard is empty
	c.R[cpu.AX] = 0x1704
	c.R[cpu.CX] = 7
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1704) after empty: %v", err)
	}
	if c.R[cpu.DX] != 0 {
		t.Errorf("DX (size after empty): want 0, got %d", c.R[cpu.DX])
	}

	// Test 9: Compact (0x1709, no-op)
	c.R[cpu.AX] = 0x1709
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1709): %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("AX after compact: want 1, got %d", c.R[cpu.AX])
	}

	// Test 10: Unsupported format should fail
	c.R[cpu.AX] = 0x1704
	c.R[cpu.CX] = 99 // Unsupported format
	if err := d.int2F(env); err != nil {
		t.Fatalf("int2F(0x1704) with unsupported format: %v", err)
	}
	if c.R[cpu.AX] != 0 {
		t.Errorf("AX for unsupported format: want 0, got %d", c.R[cpu.AX])
	}
}
