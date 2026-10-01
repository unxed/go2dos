package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/keys"
)

// fakeClip is a clipboard tool and a terminal in one.
type fakeClip struct {
	sys     string // the "system clipboard"
	osc     []string
	failW   bool
	failR   bool
	readN   int
	writeN  int
	hasTool bool
}

func (f *fakeClip) mk(mode string) *sysClip {
	c := &sysClip{mode: mode, osc: func(s string) { f.osc = append(f.osc, s) },
		read: func([]string) (string, error) {
			f.readN++
			return f.sys, map[bool]error{true: errors.New("x")}[f.failR]
		},
		write: func(_ []string, s string) error {
			f.writeN++
			f.sys = s
			return map[bool]error{true: errors.New("x")}[f.failW]
		}}
	if f.hasTool && (mode == "auto" || mode == "tool") {
		c.rd, c.wr = []string{"r"}, []string{"w"}
	}
	return c
}

func TestSysClipToolRoundTrip(t *testing.T) {
	f := &fakeClip{hasTool: true}
	c := f.mk("auto")
	c.SetText("hello")
	if f.sys != "hello" || len(f.osc) != 0 {
		t.Fatalf("sys %q osc %v: tool only expected", f.sys, f.osc)
	}
	f.sys = "from outside" // somebody else copied
	if s, _ := c.GetText(); s != "from outside" {
		t.Fatalf("GetText %q", s)
	}
	f.failR = true // the tool broke: we fall back to what we held
	if s, _ := c.GetText(); s != "from outside" {
		t.Fatalf("fallback %q", s)
	}
}

func TestSysClipFallbackToOSC(t *testing.T) {
	f := &fakeClip{hasTool: true, failW: true}
	c := f.mk("auto")
	c.SetText("a")
	if len(f.osc) != 1 || f.osc[0] != "a" {
		t.Fatalf("osc %v", f.osc)
	}
	g := &fakeClip{}
	c = g.mk("auto") // no tool at all
	c.SetText("b")
	if len(g.osc) != 1 || g.writeN != 0 {
		t.Fatalf("osc %v writes %d", g.osc, g.writeN)
	}
	if s, _ := c.GetText(); s != "b" || g.readN != 0 {
		t.Fatalf("GetText %q reads %d", s, g.readN)
	}
	c.remember("pasted") // terminal paste becomes what DOS reads
	if s, _ := c.GetText(); s != "pasted" {
		t.Fatalf("after remember %q", s)
	}
}

func TestSysClipModes(t *testing.T) {
	f := &fakeClip{hasTool: true}
	c := f.mk("off")
	c.SetText("x")
	c.remember("y")
	if f.writeN+len(f.osc) != 0 {
		t.Fatalf("off: writes %d osc %v", f.writeN, f.osc)
	}
	if s, _ := c.GetText(); s != "x" { // remember is ignored too
		t.Fatalf("off: %q", s)
	}
	f = &fakeClip{hasTool: true, failW: true}
	f.mk("tool").SetText("z")
	if len(f.osc) != 0 {
		t.Fatalf("tool: OSC sent %v", f.osc)
	}
	f = &fakeClip{hasTool: true}
	f.mk("osc52").SetText("w")
	if f.writeN != 0 || len(f.osc) != 1 {
		t.Fatalf("osc52: writes %d osc %v", f.writeN, f.osc)
	}
}

func TestFindClipTool(t *testing.T) {
	have := map[string]bool{"xclip": true, "xsel": true, "wl-paste": true, "wl-copy": true}
	look := func(n string) (string, error) {
		if have[n] {
			return n, nil
		}
		return "", errors.New("no")
	}
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	rd, wr := findClipTool(env(map[string]string{"WAYLAND_DISPLAY": "w", "DISPLAY": ":0"}), look, "linux")
	if rd[0] != "wl-paste" || wr[0] != "wl-copy" {
		t.Fatalf("wayland: %v %v", rd, wr)
	}
	rd, _ = findClipTool(env(map[string]string{"DISPLAY": ":0"}), look, "linux")
	if rd[0] != "xclip" {
		t.Fatalf("x11: %v", rd)
	}
	delete(have, "xclip")
	rd, _ = findClipTool(env(map[string]string{"DISPLAY": ":0"}), look, "linux")
	if rd[0] != "xsel" {
		t.Fatalf("xsel: %v", rd)
	}
	if rd, _ = findClipTool(env(nil), look, "linux"); rd != nil {
		t.Fatalf("no display: %v", rd)
	}
	have["pbpaste"], have["pbcopy"] = true, true
	if rd, _ = findClipTool(env(nil), look, "darwin"); rd == nil || rd[0] != "pbpaste" {
		t.Fatalf("darwin: %v", rd)
	}
}

// TestSysClipRealTool runs the real exec path against fake xclip/xsel-style scripts.
func TestSysClipRealTool(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh scripts")
	}
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	script := "#!/bin/sh\ncase \"$*\" in *-o*) cat '" + store + "';; *) cat > '" + store + "';; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "xclip"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
	c := newSysClip("auto", nil)
	if c.rd == nil || c.rd[0] != "xclip" {
		t.Fatalf("tool not found: %v", c.rd)
	}
	c.SetText("привет\nмир")
	if b, _ := os.ReadFile(store); string(b) != "привет\nмир" {
		t.Fatalf("stored %q", b)
	}
	os.WriteFile(store, []byte("outer"), 0o644)
	if s, err := c.GetText(); err != nil || s != "outer" {
		t.Fatalf("GetText %q %v", s, err)
	}
}

func TestInputParserCtrlShiftIns(t *testing.T) {
	page, _ := cp.Get(437)
	var got []bios.KeyEvent
	p := &inputParser{page: page, push: func(k bios.KeyEvent) { got = append(got, k) }, cmd: func(termCmd) {}}
	p.run(strings.NewReader("\x1b[2;6~\x1b[2;5~"))
	a, _ := keys.Named("Ins", bios.ModLShift) // Ctrl-Shift-Ins is Shift-Ins
	b, _ := keys.Named("Ins", bios.ModCtrl)   // Ctrl-Ins stays copy
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("keys %+v", got)
	}
}
