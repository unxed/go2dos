package machine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer that the test can read while the machine writes.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// twMachine starts a machine with a hand-assembled program in which the
// multiplex number of DOS-HOST/TEXTWIN replaces every MUX byte (0xEE here).
func twMachine(t *testing.T, cfg Config, prog []byte) *Machine {
	t.Helper()
	dir := t.TempDir()
	cfg.Drives, cfg.Codepage = map[byte]string{'C': dir}, 437
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux, ok := m.DOS.AMISMux("DOS-HOST", "TEXTWIN")
	if !ok {
		t.Fatal("no DOS-HOST/TEXTWIN provider")
	}
	p := bytes.ReplaceAll(prog, []byte{0xB4, 0xEE}, []byte{0xB4, mux}) // mov ah,MUX
	if err := os.WriteFile(filepath.Join(dir, "P.COM"), p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\P.COM`, ""); err != nil {
		t.Fatal(err)
	}
	return m
}

// The program asks for the window size (AL=10h) and writes CX and DX to
// standard output (pipe mode).
//
//	100: mov ah,MUX / mov al,10h / int 2Dh
//	106: mov [200h],cx / mov [202h],dx
//	110: mov ah,40h / mov bx,1 / mov cx,4 / mov dx,200h / int 21h
//	11C: mov ax,4C00h / int 21h
func TestTextWinSize(t *testing.T) {
	prog := []byte{
		0xB4, 0xEE, 0xB0, 0x10, 0xCD, 0x2D,
		0x89, 0x0E, 0x00, 0x02, 0x89, 0x16, 0x02, 0x02,
		0xB4, 0x40, 0xBB, 0x01, 0x00, 0xB9, 0x04, 0x00, 0xBA, 0x00, 0x02, 0xCD, 0x21,
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
	}
	var out syncBuffer
	m := twMachine(t, Config{Cols: 100, Rows: 40, Stdout: &out}, prog)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d", c)
	}
	if got, want := out.String(), "\x64\x00\x28\x00"; got != want { // 100 and 40
		t.Errorf("size words %q, want %q", got, want)
	}
}

// The program asks to be told of size changes (AL=11h), prints R, waits for a
// key with INT 16h/00h and writes it, then asks for the size again and writes
// CX and DX. The test resizes the window after the R.
//
//	100: mov ah,MUX / mov al,11h / mov bl,1 / int 2Dh
//	108: mov dl,'R' / mov ah,2 / int 21h
//	10E: mov ah,0 / int 16h / mov [200h],ax
//	115: mov ah,MUX / mov al,10h / int 2Dh / mov [202h],cx / mov [204h],dx
//	125: mov ah,40h / mov bx,1 / mov cx,6 / mov dx,200h / int 21h
//	131: mov ax,4C00h / int 21h
func TestTextWinResizeEvent(t *testing.T) {
	prog := []byte{
		0xB4, 0xEE, 0xB0, 0x11, 0xB3, 0x01, 0xCD, 0x2D,
		0xB2, 0x52, 0xB4, 0x02, 0xCD, 0x21,
		0xB4, 0x00, 0xCD, 0x16, 0xA3, 0x00, 0x02,
		0xB4, 0xEE, 0xB0, 0x10, 0xCD, 0x2D, 0x89, 0x0E, 0x02, 0x02, 0x89, 0x16, 0x04, 0x02,
		0xB4, 0x40, 0xBB, 0x01, 0x00, 0xB9, 0x06, 0x00, 0xBA, 0x00, 0x02, 0xCD, 0x21,
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
	}
	var out syncBuffer
	m := twMachine(t, Config{Stdout: &out}, prog)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() {
		for out.String() == "" { // wait for the R: the program is watching now
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
		if err := m.Resize(120, 50); err != nil {
			t.Error(err)
		}
	}()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d", c)
	}
	// R, then AX=FF00h (AL=00h, AH=FFh is U+00A0 in CP437), then 120 and 50.
	if got, want := out.String(), "R\x00\u00a0x\x002\x00"; got != want {
		t.Errorf("output %q, want %q", got, want)
	}
	if s := m.Screen(); s.Cols != 120 || s.Rows != 50 {
		t.Errorf("screen %dx%d, want 120x50", s.Cols, s.Rows)
	}
	if err := m.Resize(79, 25); err == nil {
		t.Error("Resize accepted 79 columns")
	}
}

// Without AL=11h a resize changes the size and puts nothing in the buffer.
func TestTextWinResizeQuiet(t *testing.T) {
	m := twMachine(t, Config{}, []byte{0xEB, 0xFE}) // jmp $
	if err := m.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	m.Run(ctx)
	if s := m.Screen(); s.Cols != 100 || s.Rows != 30 {
		t.Errorf("screen %dx%d, want 100x30", s.Cols, s.Rows)
	}
	if m.BIOS.KeysPending() || m.BIOS.BufferedKeys() != 0 {
		t.Error("a key event without AL=11h")
	}
}

// AL=12h: the full-screen role shows the grid, the console role the stream.
//
//	100: mov ah,MUX / mov al,12h / mov bl,1 / int 2Dh
//	108: mov ah,MUX / mov al,12h / mov bl,0 / int 2Dh
//	110: mov ax,4C00h / int 21h
func TestTextWinRole(t *testing.T) {
	prog := []byte{
		0xB4, 0xEE, 0xB0, 0x12, 0xB3, 0x01, 0xCD, 0x2D,
		0xB4, 0xEE, 0xB0, 0x12, 0xB3, 0x00, 0xCD, 0x2D,
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
	}
	var events []bool
	m := twMachine(t, Config{Display: "console", OnStream: func([]byte) {}, OnDisplay: func(g bool) { events = append(events, g) }}, prog)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d", c)
	}
	if len(events) != 2 || !events[0] || events[1] {
		t.Errorf("display events %v, want [true false]", events)
	}
}
