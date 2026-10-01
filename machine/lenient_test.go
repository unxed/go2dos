package machine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/hle"
)

// A COM program that makes two unsupported calls, INT 21h AH=5Ah and
// INT 10h AH=FDh, and checks the INT 21h answer ("invalid function").
var lenientProg = []byte{
	0xB4, 0x5A, 0xCD, 0x21, // mov ah,5Ah; int 21h
	0x73, 0x0E, // jnc bad
	0x3D, 0x01, 0x00, // cmp ax,1
	0x75, 0x09, // jne bad
	0xB4, 0xFD, 0xCD, 0x10, // mov ah,0FDh; int 10h
	0xB8, 0x00, 0x4C, 0xCD, 0x21, // mov ax,4C00h; int 21h
	0xB8, 0x01, 0x4C, 0xCD, 0x21, // bad: mov ax,4C01h; int 21h
}

func runLenient(t *testing.T, lenient bool) (*Machine, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "LENIENT.COM"), lenientProg, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437, Lenient: lenient})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\LENIENT.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m, m.Run(ctx)
}

// By default an unsupported call stops the machine (fail fast).
func TestUnsupportedFailsFast(t *testing.T) {
	_, err := runLenient(t, false)
	var f *FaultError
	// The CPU turns a trap error into a fault with its text only.
	if !errors.As(err, &f) || !strings.Contains(err.Error(), "unsupported: INT 21h AH=5Ah") {
		t.Fatalf("want a fault on the unsupported INT 21h AH=5Ah, got %v", err)
	}
}

// In lenient mode the calls are answered, counted and the program goes on.
func TestLenient(t *testing.T) {
	m, err := runLenient(t, true)
	if c := exitCode(t, err); c != 0 {
		t.Fatalf("exit code %d, want 0 (INT 21h must answer CF=1 AX=1)", c)
	}
	list := m.Unsupported()
	if len(list) != 2 {
		t.Fatalf("unsupported calls: %+v", list)
	}
	if list[0].Handler != "int21" || !strings.Contains(list[0].What, "AH=5Ah") || list[0].Count != 1 {
		t.Errorf("first: %+v", list[0])
	}
	if list[1].Handler != "int10" || !strings.Contains(list[1].What, "AH=FDh") {
		t.Errorf("second: %+v", list[1])
	}
	if s := hle.FormatUnsupported(list); !strings.Contains(s, "int21") || !strings.Contains(s, "int10") {
		t.Errorf("summary:\n%s", s)
	}
	rep := m.report("test")
	if !strings.Contains(rep, "unsupported calls answered in lenient mode") || !strings.Contains(rep, "AH=5Ah") {
		t.Errorf("the dump report lacks the summary")
	}
}

// INT 10h AH=FFh (TopView/DESQview "update screen from shadow buffer") is not
// an unsupported call: without a multitasker (AH=FEh leaves ES:DI unchanged)
// there is no shadow buffer and nothing to update; the machine must not stop.
func TestTopViewUpdateIsNoop(t *testing.T) {
	prog := []byte{
		0xB4, 0xFE, 0xCD, 0x10, // mov ah,0FEh; int 10h (get shadow buffer)
		0xB9, 0x10, 0x00, // mov cx,16
		0xB4, 0xFF, 0xCD, 0x10, // mov ah,0FFh; int 10h (update)
		0xB8, 0x00, 0x4C, 0xCD, 0x21, // mov ax,4C00h; int 21h
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TV.COM"), prog, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\TV.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d, want 0", c)
	}
}

// INT 15h AX=1022h BX=0000h (TopView GETVER, RBIL): BX stays 0, "TopView is not
// loaded"; DN 1.51 asks this at start. Runs without -lenient.
func TestTopViewGetVerNotLoaded(t *testing.T) {
	prog := []byte{
		0xB8, 0x22, 0x10, // mov ax,1022h
		0xBB, 0x00, 0x00, // mov bx,0
		0xCD, 0x15, // int 15h
		0x85, 0xDB, // test bx,bx
		0x75, 0x05, // jnz bad
		0xB8, 0x00, 0x4C, 0xCD, 0x21, // mov ax,4C00h; int 21h
		0xB8, 0x01, 0x4C, 0xCD, 0x21, // bad: mov ax,4C01h; int 21h
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TV.COM"), prog, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\TV.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d, want 0", c)
	}
}
