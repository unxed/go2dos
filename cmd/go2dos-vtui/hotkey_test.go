package main

import (
	"testing"

	"github.com/unxed/vtinput"
)

func key(vk uint16, ch rune, down bool, mods vtinput.ControlKeyState) *vtinput.InputEvent {
	return &vtinput.InputEvent{Type: vtinput.KeyEventType, VirtualKeyCode: vk, Char: ch, KeyDown: down, ControlKeyState: mods, RepeatCount: 1}
}

func TestHotkey(t *testing.T) {
	ctrlBr := func(down bool) *vtinput.InputEvent {
		return key(vtinput.VK_OEM_6, 0x1D, down, vtinput.LeftCtrlPressed)
	}
	cases := []struct {
		name string
		evs  []*vtinput.InputEvent
		want []hotCmd
	}{
		{"plain key passes", []*vtinput.InputEvent{key(vtinput.VK_Q, 'q', true, 0)}, []hotCmd{hotNone}},
		{"quit", []*vtinput.InputEvent{ctrlBr(true), ctrlBr(false), key(vtinput.VK_Q, 'q', true, 0)}, []hotCmd{hotEaten, hotEaten, hotQuit}},
		{"dump", []*vtinput.InputEvent{ctrlBr(true), key(vtinput.VK_D, 'D', true, vtinput.ShiftPressed)}, []hotCmd{hotEaten, hotDump}},
		{"pass through", []*vtinput.InputEvent{ctrlBr(true), ctrlBr(true)}, []hotCmd{hotEaten, hotPass}},
		{"legacy byte 1Dh without VK", []*vtinput.InputEvent{{Type: vtinput.KeyEventType, KeyDown: true, Char: 0x1D, ControlKeyState: vtinput.RightCtrlPressed}, key(0, 'q', true, 0)}, []hotCmd{hotEaten, hotQuit}},
		{"modifier and release do not disarm", []*vtinput.InputEvent{ctrlBr(true), key(vtinput.VK_CONTROL, 0, false, 0), key(vtinput.VK_SHIFT, 0, true, vtinput.ShiftPressed), key(vtinput.VK_Q, 'q', false, 0), key(vtinput.VK_Q, 'q', true, 0)}, []hotCmd{hotEaten, hotEaten, hotEaten, hotEaten, hotQuit}},
		{"other key cancels", []*vtinput.InputEvent{ctrlBr(true), key(vtinput.VK_A, 'a', true, 0), key(vtinput.VK_Q, 'q', true, 0)}, []hotCmd{hotEaten, hotEaten, hotNone}},
		{"Ctrl without ] passes", []*vtinput.InputEvent{key(vtinput.VK_A, 1, true, vtinput.LeftCtrlPressed)}, []hotCmd{hotNone}},
	}
	for _, c := range cases {
		var h hotkey
		for i, ev := range c.evs {
			if got := h.feed(ev); got != c.want[i] {
				t.Errorf("%s: event %d: got %d, want %d", c.name, i, got, c.want[i])
			}
		}
	}
}
