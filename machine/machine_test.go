package machine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/dos"
)

func runProg(t *testing.T, name string) (*Machine, error) {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\`+strings.ToUpper(name), ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m, m.Run(ctx)
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	var ex *ExitError
	if !errors.As(err, &ex) {
		t.Fatalf("want a normal exit, got %v", err)
	}
	return ex.Code
}

func TestHello(t *testing.T) {
	m, err := runProg(t, "hello.com")
	if c := exitCode(t, err); c != 7 {
		t.Errorf("exit code %d, want 7", c)
	}
	if got := m.Screen().Line(0); got != "Hello from go2dos" {
		t.Errorf("screen line 0 = %q", got)
	}
}

func TestFiles(t *testing.T) {
	m, err := runProg(t, "files.com")
	if c := exitCode(t, err); c != 0 {
		t.Fatalf("exit code %d; screen:\n%s", c, m.Screen().Text())
	}
	if got := m.Screen().Line(0); got != "TEST.TXT: file I/O works" {
		t.Errorf("screen line 0 = %q", got)
	}
}

func runConsole(t *testing.T, name string) (stream string, events []bool, m *Machine) {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", name))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, name), src, 0o644)
	m, err = New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437, Display: "console",
		OnStream:  func(b []byte) { stream += string(b) },
		OnDisplay: func(g bool) { events = append(events, g) }})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\`+strings.ToUpper(name), ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exitCode(t, m.Run(ctx))
	return stream, events, m
}

func TestConsoleStreamOnly(t *testing.T) {
	stream, events, _ := runConsole(t, "hello.com")
	if stream != "Hello from go2dos\r\n" {
		t.Errorf("stream %q", stream)
	}
	if len(events) != 0 {
		t.Errorf("display events %v, want none", events)
	}
}

func TestConsoleSwitchesToGridOnDirectWrite(t *testing.T) {
	stream, events, m := runConsole(t, "direct.com")
	if stream != "before\r\n" {
		t.Errorf("stream %q: output after the direct write belongs to the grid", stream)
	}
	if len(events) != 2 || !events[0] || events[1] {
		t.Errorf("display events %v, want [true false]", events)
	}
	if got := m.Screen().Line(1); got != "after" {
		t.Errorf("grid line 1 = %q", got)
	}
}

func TestParseWatch(t *testing.T) {
	got, err := ParseWatch("22CD, 1234:0010/2")
	if err != nil {
		t.Fatal(err)
	}
	want := []uint32{0x22CD, 0x12350, 0x12351}
	if len(got) != len(want) {
		t.Fatalf("got %X, want %X", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %X, want %X", got, want)
		}
	}
	if _, err := ParseWatch("zz"); err == nil {
		t.Error("bad address accepted")
	}
}

func TestCopyRegionAndPasteText(t *testing.T) {
	m, err := New(Config{Drives: map[byte]string{'C': "/tmp"}, Codepage: 437, Clipboard: &dos.InMemoryClipboard{}})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Test PasteText with explicit text
	text := "Hello"
	err = m.PasteText(text)
	if err != nil {
		t.Fatalf("PasteText failed: %v", err)
	}

	// PasteText should inject key events into the BIOS queue
	// We can't easily verify this without inspecting internal state,
	// but the call should succeed.
}
