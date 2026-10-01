package main

import "github.com/unxed/vtinput"

// Служебная клавиша фронтенда: Ctrl-] и затем q (выход), d (выход с дампом),
// ] (передать Ctrl-] программе) — как в cmd/go2dos.

type hotCmd int

const (
	hotNone  hotCmd = iota // событие не служебное: передать дальше
	hotEaten               // событие поглощено (служебная клавиша ждёт команду)
	hotQuit
	hotDump
	hotPass // Ctrl-] Ctrl-]: передать программе один Ctrl-]
)

type hotkey struct{ armed bool }

func isCtrlBracket(ev *vtinput.InputEvent) bool {
	ctrl := ev.ControlKeyState&(vtinput.LeftCtrlPressed|vtinput.RightCtrlPressed) != 0
	return ev.Type == vtinput.KeyEventType && ctrl && (ev.VirtualKeyCode == vtinput.VK_OEM_6 || ev.Char == 0x1D)
}

func isModifierKey(vk uint16) bool {
	return vk == vtinput.VK_SHIFT || vk == vtinput.VK_CONTROL || vk == vtinput.VK_MENU ||
		vk == vtinput.VK_LWIN || vk == vtinput.VK_RWIN
}

// feed разбирает одно событие. Отпускания и отдельные модификаторы
// служебную клавишу не сбрасывают.
func (h *hotkey) feed(ev *vtinput.InputEvent) hotCmd {
	if ev.Type != vtinput.KeyEventType {
		if h.armed {
			return hotEaten
		}
		return hotNone
	}
	if !h.armed {
		if ev.KeyDown && isCtrlBracket(ev) {
			h.armed = true
			return hotEaten
		}
		return hotNone
	}
	if !ev.KeyDown || isModifierKey(ev.VirtualKeyCode) {
		return hotEaten
	}
	h.armed = false
	switch {
	case isCtrlBracket(ev):
		return hotPass
	case ev.Char == 'q' || ev.Char == 'Q' || ev.VirtualKeyCode == vtinput.VK_Q:
		return hotQuit
	case ev.Char == 'd' || ev.Char == 'D' || ev.VirtualKeyCode == vtinput.VK_D:
		return hotDump
	}
	return hotEaten
}
