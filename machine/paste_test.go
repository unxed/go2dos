package machine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func runBytes(t *testing.T, prog []byte, setup func(m *Machine), timeout time.Duration) (*Machine, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "P.COM"), prog, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\P.COM`, ""); err != nil {
		t.Fatal(err)
	}
	setup(m)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return m, m.Run(ctx)
}

// A program that does not read the keyboard (jmp $) gets 15 pasted keys into
// its type-ahead buffer; the rest wait in the keyboard controller instead of
// being dropped.
func TestPasteWaitsForBufferSpace(t *testing.T) {
	m, err := runBytes(t, []byte{0xEB, 0xFE}, func(m *Machine) {
		m.PasteText("0123456789abcdefghijklmnopqrstuvwxyzABCD") // 40 keys
	}, 500*time.Millisecond)
	if err == nil {
		t.Fatal("the program ended; want it still spinning")
	}
	if n := m.BIOS.BufferedKeys(); n != 15 {
		t.Errorf("%d keys in the buffer, want 15", n)
	}
	if !m.BIOS.KeysPending() {
		t.Error("the keys beyond 15 were dropped, not held back")
	}
}

// A program that reads 40 keys with INT 21h/08h and prints them with 02h gets
// the whole text in order.
//
//	100: mov cx,40
//	103: mov ah,8 / int 21h / mov dl,al / mov ah,2 / int 21h / loop 103
//	10F: mov ax,4C00h / int 21h
func TestPasteDelivered(t *testing.T) {
	prog := []byte{
		0xB9, 0x28, 0x00,
		0xB4, 0x08, 0xCD, 0x21, 0x88, 0xC2, 0xB4, 0x02, 0xCD, 0x21, 0xE2, 0xF4,
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
	}
	const text = "0123456789abcdefghijklmnopqrstuvwxyzABCD"
	m, err := runBytes(t, prog, func(m *Machine) { m.PasteText(text) }, 10*time.Second)
	if c := exitCode(t, err); c != 0 {
		t.Fatalf("exit code %d", c)
	}
	if got := m.Screen().Line(0); got != text {
		t.Errorf("screen line 0 = %q, want %q", got, text)
	}
}
