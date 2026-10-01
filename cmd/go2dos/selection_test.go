package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
)

func selTestScreen(rows ...string) *bios.Screen {
	s := &bios.Screen{Mode: 3, Cols: len([]rune(rows[0])), Rows: len(rows), CursorVisible: true}
	for _, r := range rows {
		for _, c := range r {
			s.Cells = append(s.Cells, bios.Cell{Rune: c, Attr: 0x07})
		}
	}
	return s
}

func selKey(t *testing.T, name string) bios.KeyEvent {
	t.Helper()
	k, ok := keys.Named(name, 0)
	if !ok {
		t.Fatalf("no key %q", name)
	}
	return k
}

// Moving the cursor, marking a corner and copying: the rectangle runs from the
// marked corner to the cursor in any direction, and the text comes from
// Screen.Selection.
func TestSelectionKeys(t *testing.T) {
	s := selTestScreen("Hello world", "second line", "third      ")
	sl := newSelection(s) // text cursor at 0,0
	for _, k := range []string{"Right", "Right", "Down"} {
		if r := sl.key(selKey(t, k)); r != selContinue {
			t.Fatalf("%s: result %v", k, r)
		}
	}
	if x0, y0, x1, y1 := sl.rect(); x0 != 2 || y0 != 1 || x1 != 2 || y1 != 1 {
		t.Errorf("before marking the rectangle is the cursor cell, got %d,%d-%d,%d", x0, y0, x1, y1)
	}
	if r := sl.key(selKey(t, "Space")); r != selContinue {
		t.Fatalf("marking: result %v", r)
	}
	for _, k := range []string{"Up", "Left", "Left", "Home"} { // opposite corner: up and to the left
		sl.key(selKey(t, k))
	}
	if x0, y0, x1, y1 := sl.rect(); x0 != 0 || y0 != 0 || x1 != 2 || y1 != 1 {
		t.Errorf("rect %d,%d-%d,%d, want 0,0-2,1", x0, y0, x1, y1)
	}
	if r := sl.key(selKey(t, "Enter")); r != selCopy {
		t.Fatalf("second Enter: result %v, want copy", r)
	}
	x0, y0, x1, y1 := sl.rect()
	if got := s.Selection(x0, y0, x1, y1); got != "Hel\nsec" {
		t.Errorf("text %q", got)
	}
}

// The cursor stays on the screen; Esc cancels; keypad digits are not arrows.
func TestSelectionBoundsAndCancel(t *testing.T) {
	s := selTestScreen("abc", "def")
	s.CursorX, s.CursorY = 9, 9 // the program's cursor off the grid
	sl := newSelection(s)
	if sl.cx != 2 || sl.cy != 1 {
		t.Errorf("start %d,%d, want 2,1", sl.cx, sl.cy)
	}
	sl.key(selKey(t, "Right"))
	sl.key(selKey(t, "Down"))
	if sl.cx != 2 || sl.cy != 1 {
		t.Errorf("moved past the edge: %d,%d", sl.cx, sl.cy)
	}
	sl.key(selKey(t, "PgUp"))
	sl.key(selKey(t, "Home"))
	if sl.cx != 0 || sl.cy != 0 {
		t.Errorf("PgUp+Home: %d,%d, want 0,0", sl.cx, sl.cy)
	}
	sl.key(bios.KeyEvent{Scan: 0x50, ASCII: '2'}) // keypad 2 with NumLock
	if sl.cy != 0 {
		t.Errorf("a keypad digit moved the cursor")
	}
	if r := sl.key(selKey(t, "Esc")); r != selCancel {
		t.Errorf("Esc: result %v, want cancel", r)
	}
}

// Ctrl-] s is the select command.
func TestInputParserSelectCommand(t *testing.T) {
	page, err := cp.Get(437)
	if err != nil {
		t.Fatal(err)
	}
	var cmds []termCmd
	p := &inputParser{page: page, push: func(bios.KeyEvent) {}, cmd: func(c termCmd) { cmds = append(cmds, c) }}
	p.run(strings.NewReader("\x1dsx"))
	if len(cmds) != 1 || cmds[0] != cmdSelect {
		t.Errorf("commands %v, want one select", cmds)
	}
}

// The renderer paints the overlay (marked cells swapped, the cursor cell
// distinct, the real cursor hidden) and repaints without it afterwards.
func TestRendererSelectionOverlay(t *testing.T) {
	var out bytes.Buffer
	r := newRenderer(&out)
	s := selTestScreen("abcd", "efgh")
	r.draw(s)
	if !r.setSelection(&selection{cols: 4, rows: 2}) {
		t.Fatal("the overlay needs a drawn frame")
	}
	out.Reset()
	sl := &selection{cols: 4, rows: 2, cx: 2, cy: 1, ax: 1, ay: 0, anchored: true}
	r.setSelection(sl)
	got := out.String()
	// 0x07 ^ 0x77 = 0x70: the marked cells; 0x4F: the cursor cell
	for _, want := range []string{machine.SGR(0x70), machine.SGR(selCursorAttr), "\x1b[?25l"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in the overlay frame %q", want, got)
		}
	}
	out.Reset()
	r.setSelection(nil)
	got = out.String()
	if strings.Contains(got, machine.SGR(0x70)) || strings.Contains(got, machine.SGR(selCursorAttr)) || !strings.Contains(got, "\x1b[?25h") {
		t.Errorf("the overlay did not go away: %q", got)
	}
}

func TestRendererSelectionNeedsFrame(t *testing.T) {
	r := newRenderer(&bytes.Buffer{})
	if r.setSelection(&selection{cols: 4, rows: 2}) {
		t.Error("overlay accepted before any frame was drawn (console mode)")
	}
}

// Ctrl-] y, p, x type Ctrl-Ins, Shift-Ins and Shift-Del: GNOME Terminal and xterm keep
// the first two for the terminal's own copy and paste.
func TestInputParserClipboardKeys(t *testing.T) {
	page, err := cp.Get(437)
	if err != nil {
		t.Fatal(err)
	}
	var got []bios.KeyEvent
	p := &inputParser{page: page, push: func(k bios.KeyEvent) { got = append(got, k) }, cmd: func(termCmd) {}}
	p.run(strings.NewReader("\x1dy\x1dp\x1dx"))
	want := []struct {
		name string
		mods byte
	}{{"Ins", bios.ModCtrl}, {"Ins", bios.ModLShift}, {"Del", bios.ModLShift}}
	if len(got) != len(want) {
		t.Fatalf("keys %v, want %d", got, len(want))
	}
	for i, w := range want {
		k, _ := keys.Named(w.name, w.mods)
		if got[i] != k {
			t.Errorf("key %d: %+v, want %+v", i, got[i], k)
		}
	}
}

func TestTermHostClear(t *testing.T) {
	var b bytes.Buffer
	newTermHost(&b).Clear()
	if got := b.String(); !strings.Contains(got, "\x1b[2J") || !strings.Contains(got, "\x1b[H") {
		t.Errorf("Clear wrote %q", got)
	}
}

// resize asks the terminal for the window size and forces a full repaint.
func TestRendererResize(t *testing.T) {
	var out bytes.Buffer
	r := newRenderer(&out)
	r.draw(selTestScreen("abcd", "efgh"))
	out.Reset()
	r.resize(100, 40)
	if got := out.String(); got != "\x1b[8;40;100t" {
		t.Errorf("request %q", got)
	}
	out.Reset()
	r.draw(selTestScreen("abcd", "efgh")) // the same cells: a full repaint all the same
	if !strings.Contains(out.String(), "\x1b[2J") {
		t.Errorf("no full repaint after the resize: %q", out.String())
	}
}
