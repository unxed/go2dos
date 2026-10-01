package main

import (
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/vtinput"
)

func page866(t *testing.T) *cp.Codepage {
	t.Helper()
	p, err := cp.Get(866)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// named — эталон: то, что даёт скрипт <Name> в пакете keys.
func named(t *testing.T, name string, mods byte) bios.KeyEvent {
	t.Helper()
	k, ok := keys.Named(name, mods)
	if !ok {
		t.Fatalf("keys.Named(%q) failed", name)
	}
	return k
}

func TestToKeys(t *testing.T) {
	p := page866(t)
	const (
		shift = vtinput.ShiftPressed
		lctrl = vtinput.LeftCtrlPressed
		lalt  = vtinput.LeftAltPressed
		ralt  = vtinput.RightAltPressed
	)
	ch := func(r rune) bios.KeyEvent {
		k, ok := keys.Char(r, p)
		if !ok {
			t.Fatalf("keys.Char(%q) failed", r)
		}
		return k
	}
	cases := []struct {
		name string
		ev   vtinput.InputEvent
		want []bios.KeyEvent
	}{
		{"letter", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_A, Char: 'a'}, []bios.KeyEvent{ch('a')}},
		{"shifted letter", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_A, Char: 'A', ControlKeyState: shift}, []bios.KeyEvent{ch('A')}},
		{"cyrillic letter -> code page byte", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_F, Char: 'а'}, []bios.KeyEvent{ch('а')}},
		{"space", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_SPACE, Char: ' '}, []bios.KeyEvent{ch(' ')}},
		{"Enter", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_RETURN, Char: '\r'}, []bios.KeyEvent{named(t, "Enter", 0)}},
		{"Ctrl-Enter", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_RETURN, Char: '\n', ControlKeyState: lctrl}, []bios.KeyEvent{named(t, "Enter", bios.ModCtrl)}},
		{"Esc", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_ESCAPE}, []bios.KeyEvent{named(t, "Esc", 0)}},
		{"Shift-Tab", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_TAB, ControlKeyState: shift}, []bios.KeyEvent{named(t, "Tab", bios.ModLShift)}},
		{"F10", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_F10}, []bios.KeyEvent{named(t, "F10", 0)}},
		{"Alt-F5", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_F5, ControlKeyState: lalt}, []bios.KeyEvent{named(t, "F5", bios.ModAlt)}},
		{"Ctrl-F1", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_F1, ControlKeyState: lctrl}, []bios.KeyEvent{named(t, "F1", bios.ModCtrl)}},
		{"F12", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_F12}, []bios.KeyEvent{named(t, "F12", 0)}},
		{"Up (no scan code, kitty/legacy)", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_UP}, []bios.KeyEvent{named(t, "Up", 0)}},
		{"Up (win32 mode, enhanced)", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_UP, VirtualScanCode: 0x48, ControlKeyState: vtinput.EnhancedKey}, []bios.KeyEvent{named(t, "Up", 0)}},
		{"numpad 8 without NumLock (win32 mode)", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_UP, VirtualScanCode: 0x48}, []bios.KeyEvent{{Scan: 0x48}}},
		{"PgDn", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_NEXT}, []bios.KeyEvent{named(t, "PgDn", 0)}},
		{"Ctrl-Home", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_HOME, ControlKeyState: lctrl}, []bios.KeyEvent{named(t, "Home", bios.ModCtrl)}},
		{"Ins", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_INSERT}, []bios.KeyEvent{named(t, "Ins", 0)}},
		{"Del", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_DELETE}, []bios.KeyEvent{named(t, "Del", 0)}},
		{"Backspace", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_BACK, Char: 0x7F}, []bios.KeyEvent{named(t, "Backspace", 0)}},
		{"gray plus", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_ADD, Char: '+'}, []bios.KeyEvent{named(t, "GrayPlus", 0)}},
		{"gray star", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_MULTIPLY, Char: '*'}, []bios.KeyEvent{named(t, "GrayStar", 0)}},
		{"numpad digit with NumLock", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_NUMPAD5, Char: '5'}, []bios.KeyEvent{ch('5')}},
		{"Ctrl-O", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_O, Char: 0x0F, ControlKeyState: lctrl}, []bios.KeyEvent{keys.Ctrl('o')}},
		{"Ctrl-R legacy", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_R, ControlKeyState: lctrl, IsLegacy: true}, []bios.KeyEvent{keys.Ctrl('r')}},
		{"Ctrl-]", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_OEM_6, ControlKeyState: lctrl}, []bios.KeyEvent{keys.Ctrl(']')}},
		{"Ctrl-Space", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_SPACE, Char: ' ', ControlKeyState: lctrl}, []bios.KeyEvent{named(t, "Space", bios.ModCtrl)}},
		{"Alt-X", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_X, Char: 'x', ControlKeyState: lalt}, []bios.KeyEvent{keys.Alt('x')}},
		{"Alt-3", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_3, Char: '3', ControlKeyState: lalt}, []bios.KeyEvent{keys.Alt('3')}},
		{"Alt without VK, kitty unshifted char", vtinput.InputEvent{Char: 'q', UnshiftedChar: 'q', ControlKeyState: lalt}, []bios.KeyEvent{keys.Alt('q')}},
		{"AltGr character is typed", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_Q, Char: '@', ControlKeyState: lctrl | ralt}, []bios.KeyEvent{ch('@')}},
		{"auto-repeat", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_DOWN, RepeatCount: 3}, []bios.KeyEvent{named(t, "Down", 0), named(t, "Down", 0), named(t, "Down", 0)}},
		{"repeat is capped", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_DOWN, RepeatCount: 60000}, nil},
		{"key up", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_A, Char: 'a'}, nil},
		{"modifier alone", vtinput.InputEvent{VirtualKeyCode: vtinput.VK_SHIFT, ControlKeyState: shift}, nil},
		{"character outside the code page", vtinput.InputEvent{Char: '€'}, nil},
		{"mouse", vtinput.InputEvent{Type: vtinput.MouseEventType, MouseX: 3}, nil},
		{"focus", vtinput.InputEvent{Type: vtinput.FocusEventType, SetFocus: true}, nil},
		{"paste marker", vtinput.InputEvent{Type: vtinput.PasteEventType, PasteStart: true}, nil},
	}
	for _, c := range cases {
		ev := c.ev
		if ev.Type == 0 {
			ev.Type = vtinput.KeyEventType
			ev.KeyDown = c.name != "key up"
		}
		got := toKeys(&ev, p)
		if c.name == "repeat is capped" {
			c.want = make([]bios.KeyEvent, maxRepeat)
			for i := range c.want {
				c.want[i] = named(t, "Down", 0)
			}
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d keys %+v, want %d %+v", c.name, len(got), got, len(c.want), c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: key %d: got %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestPumpEvents(t *testing.T) {
	p := page866(t)
	ctrlBr := &vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_OEM_6, ControlKeyState: vtinput.LeftCtrlPressed}
	down := func(vk uint16, r rune) *vtinput.InputEvent {
		return &vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vk, Char: r}
	}
	events := make(chan *vtinput.InputEvent, 16)
	for _, ev := range []*vtinput.InputEvent{
		down(vtinput.VK_L, 'l'), down(vtinput.VK_F10, 0), // обычные клавиши
		ctrlBr, ctrlBr, // Ctrl-] Ctrl-] — программе уходит один Ctrl-]
		ctrlBr, down(vtinput.VK_D, 'd'), // выход с дампом
		down(vtinput.VK_Z, 'z'), // после выхода событий ещё может быть
	} {
		events <- ev
	}
	close(events)
	var got []bios.KeyEvent
	var stops []bool
	pumpEvents(events, func(k bios.KeyEvent) { got = append(got, k) }, p, func(dump bool) { stops = append(stops, dump) })
	l, _ := keys.Char('l', p)
	z, _ := keys.Char('z', p)
	want := []bios.KeyEvent{l, named(t, "F10", 0), keys.Ctrl(']'), z}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("key %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(stops) != 1 || !stops[0] {
		t.Errorf("stop calls %v, want one with dump", stops)
	}
}
