package frontend

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/machine"
)

// fakeHost запоминает вызовы Run: Start, Draw и restore.
type fakeHost struct {
	started  bool
	display  string
	drawn    int
	restored bool
}

func (h *fakeHost) Draw(*bios.Screen) { h.drawn++ }
func (h *fakeHost) Start(m *machine.Machine, display string, stop func(bool)) (func(), error) {
	h.started, h.display = true, display
	return func() { h.restored = true }, nil
}

// COM-программа: mov ax,4C03h; int 21h (выход с кодом 3).
var exit3 = []byte{0xB8, 0x03, 0x4C, 0xCD, 0x21}

func writeCom(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "EXIT3.COM")
	if err := os.WriteFile(p, exit3, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func parse(t *testing.T, terminal bool, args ...string) *Options {
	t.Helper()
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	o := RegisterFlags(fs, terminal)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestRegisterFlags(t *testing.T) {
	o := parse(t, true, "-drive", "c=/a", "-drive", "D=/b", "-cp", "866", "-lenient", "-nolfn", "-display", "grid", "-headless")
	if o.Drives['C'] != "/a" || o.Drives['D'] != "/b" || o.Codepage != 866 || !o.Lenient || !o.NoLFN || o.Display != "grid" || !o.Headless {
		t.Errorf("unexpected options: %+v", o)
	}
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	RegisterFlags(fs, false)
	if fs.Lookup("headless") != nil || fs.Lookup("display") != nil {
		t.Error("-headless/-display must exist only for the terminal front end")
	}
	if err := (driveFlags{}).Set("CD=x"); err == nil {
		t.Error("bad -drive accepted")
	}
}

func TestRunWithHost(t *testing.T) {
	o := parse(t, false, "-dump-dir", t.TempDir())
	h := &fakeHost{}
	if code := Run(o, []string{writeCom(t)}, h); code != 3 {
		t.Fatalf("exit code %d, want 3", code)
	}
	if !h.started || !h.restored {
		t.Errorf("Start/restore not called: %+v", h)
	}
	if h.display != "grid" {
		t.Errorf("display %q, want grid for a host without -display", h.display)
	}
}

func TestRunConsoleNeedsConsoleHost(t *testing.T) {
	o := parse(t, true, "-display", "console", "-dump-dir", t.TempDir())
	h := &fakeHost{}
	Run(o, []string{writeCom(t)}, h)
	if h.display != "grid" {
		t.Errorf("display %q, want grid for a host that is not a Console", h.display)
	}
}

func TestRunNeedsHost(t *testing.T) {
	o := parse(t, true)
	if code := Run(o, []string{writeCom(t)}, nil); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}

func TestMemoryClipboard(t *testing.T) {
	var c Clipboard = &MemoryClipboard{}
	if s, _ := c.GetText(); s != "" {
		t.Errorf("new clipboard holds %q", s)
	}
	c.SetText("привет\nмир")
	if s, err := c.GetText(); err != nil || s != "привет\nмир" {
		t.Errorf("got %q, %v", s, err)
	}
}
