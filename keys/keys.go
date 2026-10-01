// Package keys maps key names and characters to BIOS keystrokes and
// implements the key script format used for automation and session replay.
//
// Script syntax: plain characters are typed as is; <Name> presses a named
// key, optionally with modifiers (<Ctrl-F5>, <Alt-X>, <Shift-Tab>); << types
// a literal '<'. Control tokens: <wait:500ms>, <waitfor:text> (until the
// text appears on screen), <screen> (log the screen).
package keys

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
)

type named struct {
	scan  byte
	ascii byte
	gray  bool
}

var namedKeys = map[string]named{
	"Esc": {0x01, 0x1B, false}, "Backspace": {0x0E, 0x08, false}, "Tab": {0x0F, 0x09, false},
	"Enter": {0x1C, 0x0D, false}, "Space": {0x39, ' ', false},
	"F1": {0x3B, 0, false}, "F2": {0x3C, 0, false}, "F3": {0x3D, 0, false}, "F4": {0x3E, 0, false},
	"F5": {0x3F, 0, false}, "F6": {0x40, 0, false}, "F7": {0x41, 0, false}, "F8": {0x42, 0, false},
	"F9": {0x43, 0, false}, "F10": {0x44, 0, false}, "F11": {0x85, 0, false}, "F12": {0x86, 0, false},
	"Home": {0x47, 0xE0, true}, "Up": {0x48, 0xE0, true}, "PgUp": {0x49, 0xE0, true},
	"Left": {0x4B, 0xE0, true}, "Right": {0x4D, 0xE0, true}, "End": {0x4F, 0xE0, true},
	"Down": {0x50, 0xE0, true}, "PgDn": {0x51, 0xE0, true}, "Ins": {0x52, 0xE0, true}, "Del": {0x53, 0xE0, true},
	"GrayPlus": {0x4E, '+', false}, "GrayMinus": {0x4A, '-', false}, "GrayStar": {0x37, '*', false},
}

// US layout: scan codes of printable ASCII characters and whether Shift is needed.
var asciiScan [128]byte
var asciiShift [128]bool

func init() {
	rows := []struct {
		scan        byte
		plain, shft string
	}{
		{0x02, "1234567890-=", "!@#$%^&*()_+"},
		{0x10, "qwertyuiop[]", "QWERTYUIOP{}"},
		{0x1E, "asdfghjkl;'`", "ASDFGHJKL:\"~"},
		{0x2B, "\\zxcvbnm,./", "|ZXCVBNM<>?"},
	}
	for _, r := range rows {
		for i := 0; i < len(r.plain); i++ {
			asciiScan[r.plain[i]] = r.scan + byte(i)
			asciiScan[r.shft[i]] = r.scan + byte(i)
			asciiShift[r.shft[i]] = true
		}
	}
	asciiScan[' '] = 0x39
	asciiScan[0x0D] = 0x1C
	asciiScan[0x1B] = 0x01
	asciiScan[0x08] = 0x0E
	asciiScan[0x09] = 0x0F
}

// Char returns the keystroke for typing rune r (US layout; characters
// outside ASCII get scan code 0, as when typed with Alt+numpad).
func Char(r rune, page *cp.Codepage) (bios.KeyEvent, bool) {
	if r < 0x80 {
		k := bios.KeyEvent{Scan: asciiScan[r], ASCII: byte(r)}
		if asciiShift[r] {
			k.Mods = bios.ModLShift
		}
		return k, true
	}
	b, ok := page.Byte(r)
	if !ok {
		return bios.KeyEvent{}, false
	}
	return bios.KeyEvent{ASCII: b}, true
}

// Ctrl returns Ctrl+letter (letter is 'a'..'z' or '[', '\\', ']').
func Ctrl(letter byte) bios.KeyEvent {
	if letter >= 'A' && letter <= 'Z' {
		letter += 32
	}
	return bios.KeyEvent{Scan: asciiScan[letter], ASCII: letter & 0x1F, Mods: bios.ModCtrl}
}

// Alt returns Alt+key for a letter or digit.
func Alt(ch byte) bios.KeyEvent {
	if ch >= 'A' && ch <= 'Z' {
		ch += 32
	}
	scan := asciiScan[ch]
	if ch >= '1' && ch <= '9' {
		scan = 0x78 + ch - '1'
	} else if ch == '0' {
		scan = 0x81
	} else if ch == '-' {
		scan = 0x82
	} else if ch == '=' {
		scan = 0x83
	}
	return bios.KeyEvent{Scan: scan, Mods: bios.ModAlt}
}

// Named returns a named key with modifiers applied, following the BIOS
// conventions for extended scan codes.
func Named(name string, mods byte) (bios.KeyEvent, bool) {
	n, ok := namedKeys[name]
	if !ok {
		return bios.KeyEvent{}, false
	}
	k := bios.KeyEvent{Scan: n.scan, ASCII: n.ascii, Gray: n.gray, Mods: mods}
	shift := mods&(bios.ModLShift|bios.ModRShift) != 0
	ctrl, alt := mods&bios.ModCtrl != 0, mods&bios.ModAlt != 0
	switch {
	case n.scan >= 0x3B && n.scan <= 0x44: // F1-F10
		switch {
		case alt:
			k.Scan = n.scan - 0x3B + 0x68
		case ctrl:
			k.Scan = n.scan - 0x3B + 0x5E
		case shift:
			k.Scan = n.scan - 0x3B + 0x54
		}
	case n.scan == 0x85 || n.scan == 0x86: // F11, F12
		switch {
		case alt:
			k.Scan += 6
		case ctrl:
			k.Scan += 4
		case shift:
			k.Scan += 2
		}
	case n.gray:
		if ctrl {
			k.Scan = map[byte]byte{0x47: 0x77, 0x48: 0x8D, 0x49: 0x84, 0x4B: 0x73, 0x4D: 0x74,
				0x4F: 0x75, 0x50: 0x91, 0x51: 0x76, 0x52: 0x92, 0x53: 0x93}[n.scan]
		} else if alt {
			k.Scan = n.scan + 0x50
			k.ASCII = 0
		}
	case name == "Tab" && shift:
		k.ASCII = 0
	case name == "Enter" && ctrl:
		k.ASCII = 0x0A
	case name == "Backspace" && ctrl:
		k.ASCII = 0x7F
	}
	if alt && !n.gray && k.Scan < 0x54 {
		k.ASCII = 0
	}
	return k, true
}

// Step is one parsed script element.
type Step struct {
	Key     *bios.KeyEvent
	Wait    time.Duration
	WaitFor string
	Screen  bool
	Text    string // original token, for diagnostics
}

var modNames = map[string]byte{"Ctrl": bios.ModCtrl, "Alt": bios.ModAlt, "Shift": bios.ModLShift}

// Parse parses a key script.
func Parse(s string, page *cp.Codepage) ([]Step, error) {
	var out []Step
	for len(s) > 0 {
		if strings.HasPrefix(s, "<<") {
			k, _ := Char('<', page)
			out = append(out, Step{Key: &k, Text: "<"})
			s = s[2:]
			continue
		}
		if s[0] == '<' {
			end := strings.IndexByte(s, '>')
			if end < 0 {
				return nil, fmt.Errorf("unterminated token in %q", s)
			}
			tok := s[1:end]
			s = s[end+1:]
			st, err := parseToken(tok)
			if err != nil {
				return nil, err
			}
			st.Text = "<" + tok + ">"
			out = append(out, st)
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == '\n' {
			r = '\r'
		}
		k, ok := Char(r, page)
		if !ok {
			return nil, fmt.Errorf("character %q is not in code page %d", r, page.Num)
		}
		out = append(out, Step{Key: &k, Text: string(r)})
	}
	return out, nil
}

func parseToken(tok string) (Step, error) {
	switch {
	case strings.HasPrefix(tok, "wait:"):
		d, err := time.ParseDuration(tok[5:])
		return Step{Wait: d}, err
	case strings.HasPrefix(tok, "waitfor:"):
		return Step{WaitFor: tok[8:]}, nil
	case tok == "screen":
		return Step{Screen: true}, nil
	case strings.HasPrefix(tok, "raw:") && len(tok) == 8:
		var w uint16
		if _, err := fmt.Sscanf(tok[4:], "%04X", &w); err != nil {
			return Step{}, fmt.Errorf("bad raw key <%s>", tok)
		}
		return Step{Key: &bios.KeyEvent{Scan: byte(w >> 8), ASCII: byte(w)}}, nil
	}
	var mods byte
	parts := strings.Split(tok, "-")
	for len(parts) > 1 {
		m, ok := modNames[parts[0]]
		if !ok {
			break
		}
		mods |= m
		parts = parts[1:]
	}
	name := strings.Join(parts, "-")
	if k, ok := Named(name, mods); ok {
		return Step{Key: &k}, nil
	}
	if len(name) == 1 {
		ch := name[0]
		switch {
		case mods == bios.ModCtrl && (ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch == '[' || ch == '\\' || ch == ']'):
			k := Ctrl(ch)
			return Step{Key: &k}, nil
		case mods == bios.ModAlt:
			k := Alt(ch)
			return Step{Key: &k}, nil
		}
	}
	return Step{}, fmt.Errorf("unknown key <%s>", tok)
}

// Format renders a keystroke as a script token (for session recording).
func Format(k bios.KeyEvent, page *cp.Codepage) string {
	for name, n := range namedKeys {
		for _, mods := range []byte{0, bios.ModLShift, bios.ModCtrl, bios.ModAlt} {
			if n.scan == 0x39 {
				continue
			}
			if w, _ := Named(name, mods); w.Scan == k.Scan && w.ASCII == k.ASCII {
				prefix := map[byte]string{0: "", bios.ModLShift: "Shift-", bios.ModCtrl: "Ctrl-", bios.ModAlt: "Alt-"}[mods]
				return "<" + prefix + name + ">"
			}
		}
	}
	switch {
	case k.ASCII == '<':
		return "<<"
	case k.ASCII >= 0x20 && k.ASCII < 0x7F:
		return string(rune(k.ASCII))
	case k.ASCII >= 0x80:
		return string(page.Rune(k.ASCII))
	case k.ASCII >= 1 && k.ASCII <= 26 && k.Mods&bios.ModCtrl != 0:
		return fmt.Sprintf("<Ctrl-%c>", 'A'+k.ASCII-1)
	case k.ASCII == 0 && k.Mods&bios.ModAlt != 0:
		for ch := byte('a'); ch <= 'z'; ch++ {
			if Alt(ch).Scan == k.Scan {
				return fmt.Sprintf("<Alt-%c>", ch-32)
			}
		}
		for _, ch := range []byte("1234567890-=") {
			if Alt(ch).Scan == k.Scan {
				return fmt.Sprintf("<Alt-%c>", ch)
			}
		}
	}
	return fmt.Sprintf("<raw:%02X%02X>", k.Scan, k.ASCII)
}
