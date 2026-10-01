package dos

import (
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

func TestInMemoryClipboard(t *testing.T) {
	c := NewInMemoryClipboard()

	// Initially empty
	text, err := c.GetText()
	if err != nil {
		t.Fatalf("GetText failed: %v", err)
	}
	if text != "" {
		t.Errorf("expected empty clipboard, got %q", text)
	}

	// Set text
	err = c.SetText("Hello, World!")
	if err != nil {
		t.Fatalf("SetText failed: %v", err)
	}

	// Get text back
	text, err = c.GetText()
	if err != nil {
		t.Fatalf("GetText failed: %v", err)
	}
	if text != "Hello, World!" {
		t.Errorf("expected %q, got %q", "Hello, World!", text)
	}

	// Overwrite
	err = c.SetText("New text")
	if err != nil {
		t.Fatalf("SetText failed: %v", err)
	}
	text, err = c.GetText()
	if err != nil {
		t.Fatalf("GetText failed: %v", err)
	}
	if text != "New text" {
		t.Errorf("expected %q, got %q", "New text", text)
	}
}

func TestWinOldApBasic(t *testing.T) {
	// Create a minimal DOS environment for testing.
	m := mem.New()
	c := cpu.New(m, nil)
	e := hle.New(c, m, nil, nil, nil)
	b := bios.New(e)

	// Create DOS with in-memory clipboard.
	clipboard := NewInMemoryClipboard()
	cfg := Config{
		Drives:    map[byte]string{'C': "/tmp"},
		Current:   'C' - 'A',
		Clipboard: clipboard,
	}
	d, err := New(e, b, cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Test: Check installed version (INT 2Fh AX=1700h)
	c.R[cpu.AX] = 0x1700
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F 1700h failed: %v", err)
	}
	if c.R[cpu.AX] != 0x1703 {
		t.Errorf("expected AX=0x1703, got 0x%04X", c.R[cpu.AX])
	}

	// Test: Open clipboard (INT 2Fh AX=1701h)
	c.R[cpu.AX] = 0x1701
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F 1701h failed: %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("expected AX=1, got %d", c.R[cpu.AX])
	}

	// Test: Set clipboard text (INT 2Fh AX=1703h)
	testText := "Hello"
	addr := uint32(0x2000) // arbitrary address
	m.Poke(addr, []byte(testText)...)
	c.R[cpu.AX] = 0x1703
	c.R[cpu.BX] = uint16(addr & 0xFFFF)
	c.SetSeg(cpu.ES, uint16(addr>>4))
	c.R[cpu.CX] = cfText
	c.R[cpu.SI] = uint16(len(testText))
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F 1703h failed: %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("expected AX=1 (success), got %d", c.R[cpu.AX])
	}

	// Verify clipboard was set
	clipText, _ := clipboard.GetText()
	if clipText != testText {
		t.Errorf("expected clipboard %q, got %q", testText, clipText)
	}

	// Test: Query clipboard size (INT 2Fh AX=1704h)
	c.R[cpu.AX] = 0x1704
	c.R[cpu.CX] = cfText
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F 1704h failed: %v", err)
	}
	size := uint32(c.R[cpu.DX])<<16 | uint32(c.R[cpu.AX])
	// Size should be len(testText) + 1 (null terminator)
	expectedSize := uint32(len(testText)) + 1
	if size != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, size)
	}

	// Test: Get clipboard text (INT 2Fh AX=1705h)
	bufAddr := uint32(0x3000)
	c.R[cpu.AX] = 0x1705
	c.R[cpu.BX] = uint16(bufAddr & 0xFFFF)
	c.SetSeg(cpu.ES, uint16(bufAddr>>4))
	c.R[cpu.CX] = cfText
	c.R[cpu.SI] = 64 // buffer size
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F 1705h failed: %v", err)
	}
	returnedSize := c.R[cpu.AX]
	if returnedSize != uint16(len(testText)) {
		t.Errorf("expected returned size %d, got %d", len(testText), returnedSize)
	}

	// Verify buffer contents
	bufData := m.Bytes(bufAddr, int(returnedSize)+1)
	if string(bufData[:returnedSize]) != testText {
		t.Errorf("expected buffer %q, got %q", testText, string(bufData[:returnedSize]))
	}

	// Test: Close clipboard (INT 2Fh AX=1708h)
	c.R[cpu.AX] = 0x1708
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F 1708h failed: %v", err)
	}
}

func TestWinOldApEmptyClipboard(t *testing.T) {
	// Test empty clipboard operation.
	m := mem.New()
	c := cpu.New(m, nil)
	e := hle.New(c, m, nil, nil, nil)
	b := bios.New(e)

	clipboard := NewInMemoryClipboard()
	clipboard.SetText("initial text")

	cfg := Config{
		Drives:    map[byte]string{'C': "/tmp"},
		Current:   'C' - 'A',
		Clipboard: clipboard,
	}
	d, err := New(e, b, cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Empty the clipboard (INT 2Fh AX=1702h)
	c.R[cpu.AX] = 0x1702
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F 1702h failed: %v", err)
	}
	if c.R[cpu.AX] != 1 {
		t.Errorf("expected AX=1 (success), got %d", c.R[cpu.AX])
	}

	// Verify clipboard is empty
	text, _ := clipboard.GetText()
	if text != "" {
		t.Errorf("expected empty clipboard, got %q", text)
	}
}

func TestWinOldApNoClipboard(t *testing.T) {
	// Test when no clipboard is configured.
	m := mem.New()
	c := cpu.New(m, nil)
	e := hle.New(c, m, nil, nil, nil)
	b := bios.New(e)

	cfg := Config{
		Drives:    map[byte]string{'C': "/tmp"},
		Current:   'C' - 'A',
		Clipboard: nil, // No clipboard
	}
	d, err := New(e, b, cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Any WinOldAp call should fail.
	c.R[cpu.AX] = 0x1703
	c.R[cpu.SI] = 5
	err = d.int2F(e)
	if err != nil {
		t.Errorf("int2F should not error, got %v", err)
	}
	if c.R[cpu.AX] != 0 {
		t.Errorf("expected AX=0 (failure), got %d", c.R[cpu.AX])
	}
}
