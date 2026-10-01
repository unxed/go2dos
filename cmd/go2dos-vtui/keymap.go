package main

import (
	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/vtinput"
)

// Таблица VK → имя клавиши в пакете keys. Скан-коды из события не берутся:
// vtinput заполняет VirtualScanCode только в режимах win32 и far2l, в kitty и
// на «старых» терминалах он нулевой (docs/FRONTEND.md, §2), поэтому скан-коды
// набора 1 даёт keys.Named / keys.Char / keys.Ctrl / keys.Alt.
var vkNames = map[uint16]string{
	vtinput.VK_ESCAPE: "Esc", vtinput.VK_BACK: "Backspace", vtinput.VK_TAB: "Tab", vtinput.VK_RETURN: "Enter",
	vtinput.VK_F1: "F1", vtinput.VK_F2: "F2", vtinput.VK_F3: "F3", vtinput.VK_F4: "F4", vtinput.VK_F5: "F5",
	vtinput.VK_F6: "F6", vtinput.VK_F7: "F7", vtinput.VK_F8: "F8", vtinput.VK_F9: "F9", vtinput.VK_F10: "F10",
	vtinput.VK_F11: "F11", vtinput.VK_F12: "F12",
	vtinput.VK_HOME: "Home", vtinput.VK_UP: "Up", vtinput.VK_PRIOR: "PgUp", vtinput.VK_LEFT: "Left",
	vtinput.VK_RIGHT: "Right", vtinput.VK_END: "End", vtinput.VK_DOWN: "Down", vtinput.VK_NEXT: "PgDn",
	vtinput.VK_INSERT: "Ins", vtinput.VK_DELETE: "Del",
	vtinput.VK_ADD: "GrayPlus", vtinput.VK_SUBTRACT: "GrayMinus", vtinput.VK_MULTIPLY: "GrayStar",
}

// Клавиши блока курсора: у них есть вариант с цифровой клавиатуры (без E0).
var navScan = map[string]uint16{"Home": 0x47, "Up": 0x48, "PgUp": 0x49, "Left": 0x4B, "Right": 0x4D,
	"End": 0x4F, "Down": 0x50, "PgDn": 0x51, "Ins": 0x52, "Del": 0x53}

// maxRepeat — предел повторов одного события (RepeatCount).
const maxRepeat = 32

// vkASCII возвращает букву, цифру, '-' или '=' клавиши — для Alt-сочетаний.
func vkASCII(ev *vtinput.InputEvent) (byte, bool) {
	vk := ev.VirtualKeyCode
	switch {
	case vk >= vtinput.VK_A && vk <= vtinput.VK_Z:
		return byte('a' + vk - vtinput.VK_A), true
	case vk >= vtinput.VK_0 && vk <= vtinput.VK_9:
		return byte('0' + vk - vtinput.VK_0), true
	case vk == vtinput.VK_OEM_MINUS:
		return '-', true
	case vk == vtinput.VK_OEM_PLUS:
		return '=', true
	}
	if vk == 0 { // kitty и другие: клавиша без VK — берём символ без Shift
		u := ev.UnshiftedChar
		if u >= 'a' && u <= 'z' || u >= '0' && u <= '9' {
			return byte(u), true
		}
	}
	return 0, false
}

func isModifierOnly(vk uint16) bool {
	switch vk {
	case vtinput.VK_SHIFT, vtinput.VK_LSHIFT, vtinput.VK_RSHIFT, vtinput.VK_CONTROL, vtinput.VK_LCONTROL,
		vtinput.VK_RCONTROL, vtinput.VK_MENU, vtinput.VK_LMENU, vtinput.VK_RMENU, vtinput.VK_LWIN,
		vtinput.VK_RWIN, vtinput.VK_CAPITAL, vtinput.VK_NUMLOCK, vtinput.VK_SCROLL:
		return true
	}
	return false
}

// toKeys переводит событие vtinput в нажатия BIOS. Отпускания, мышь, фокус,
// вставка и отдельные модификаторы нажатий не дают (отпускание машина
// синтезирует сама; раздельные make/break — шаг F5). Автоповтор
// (RepeatCount > 1) даёт столько же нажатий.
func toKeys(ev *vtinput.InputEvent, page *cp.Codepage) []bios.KeyEvent {
	if ev.Type != vtinput.KeyEventType || !ev.KeyDown || isModifierOnly(ev.VirtualKeyCode) {
		return nil
	}
	k, ok := toKey(ev, page)
	if !ok {
		return nil
	}
	n := int(ev.RepeatCount)
	if n < 1 {
		n = 1
	} else if n > maxRepeat {
		n = maxRepeat
	}
	out := make([]bios.KeyEvent, n)
	for i := range out {
		out[i] = k
	}
	return out
}

func toKey(ev *vtinput.InputEvent, page *cp.Codepage) (bios.KeyEvent, bool) {
	st := ev.ControlKeyState
	var mods byte
	if st&vtinput.ShiftPressed != 0 {
		mods |= bios.ModLShift
	}
	if st&(vtinput.LeftCtrlPressed|vtinput.RightCtrlPressed) != 0 {
		mods |= bios.ModCtrl
	}
	if st&(vtinput.LeftAltPressed|vtinput.RightAltPressed) != 0 {
		mods |= bios.ModAlt
	}
	// AltGr (правый Alt, на Windows вместе с левым Ctrl) вводит символ.
	if st&vtinput.RightAltPressed != 0 && ev.Char >= 0x20 && ev.Char != 0x7F {
		mods &^= bios.ModAlt | bios.ModCtrl
	}

	if name, ok := vkNames[ev.VirtualKeyCode]; ok {
		k, ok := keys.Named(name, mods)
		if !ok {
			return k, false
		}
		// Цифровая клавиатура без NumLock: тот же скан-код, но без E0 —
		// только когда событие несёт настоящий скан-код (режим win32).
		if sc, nav := navScan[name]; nav && !ev.IsLegacy && ev.VirtualScanCode == sc && st&vtinput.EnhancedKey == 0 {
			k.Gray, k.ASCII = false, 0
		}
		return k, true
	}

	ctrl, alt := mods&bios.ModCtrl != 0, mods&bios.ModAlt != 0
	shift := mods & bios.ModLShift
	switch {
	case alt:
		if c, ok := vkASCII(ev); ok {
			k := keys.Alt(c)
			k.Mods |= shift | mods&bios.ModCtrl
			return k, true
		}
		return bios.KeyEvent{}, false
	case ctrl:
		switch ev.VirtualKeyCode {
		case vtinput.VK_OEM_4:
			return keys.Ctrl('['), true
		case vtinput.VK_OEM_5:
			return keys.Ctrl('\\'), true
		case vtinput.VK_OEM_6:
			return keys.Ctrl(']'), true
		case vtinput.VK_SPACE:
			return keys.Named("Space", mods)
		case vtinput.VK_2: // Ctrl-2 / Ctrl-@
			return bios.KeyEvent{Scan: 0x03, Mods: mods}, true
		}
		if ev.VirtualKeyCode >= vtinput.VK_A && ev.VirtualKeyCode <= vtinput.VK_Z {
			k := keys.Ctrl(byte('a' + ev.VirtualKeyCode - vtinput.VK_A))
			k.Mods |= shift
			return k, true
		}
		return bios.KeyEvent{}, false
	}
	if ev.Char >= 0x20 && ev.Char != 0x7F {
		return keys.Char(ev.Char, page)
	}
	return bios.KeyEvent{}, false
}
