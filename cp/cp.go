// Package cp converts between the DOS single-byte (OEM) code page and
// Unicode, and picks the code page from the host locale.
package cp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/unxed/localecp"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// Codepage is one OEM code page.
type Codepage struct {
	Num int
	m   *charmap.Charmap
}

var tables = map[int]*charmap.Charmap{
	437: charmap.CodePage437,
	850: charmap.CodePage850,
	852: charmap.CodePage852,
	855: charmap.CodePage855,
	858: charmap.CodePage858,
	860: charmap.CodePage860,
	862: charmap.CodePage862,
	863: charmap.CodePage863,
	865: charmap.CodePage865,
	866: charmap.CodePage866,
}

// Supported lists the code page numbers with conversion tables.
func Supported() []int {
	var n []int
	for k := range tables {
		n = append(n, k)
	}
	sort.Ints(n)
	return n
}

// Get returns a supported code page.
func Get(num int) (*Codepage, error) {
	m, ok := tables[num]
	if !ok {
		return nil, fmt.Errorf("code page %d is not supported (supported: %v)", num, Supported())
	}
	return &Codepage{Num: num, m: m}, nil
}

// glyphs are the characters the video hardware shows for control bytes.
var glyphs = [32]rune{
	' ', '☺', '☻', '♥', '♦', '♣', '♠', '•', '◘', '○', '◙', '♂', '♀', '♪', '♫', '☼',
	'►', '◄', '↕', '‼', '¶', '§', '▬', '↨', '↑', '↓', '→', '←', '∟', '↔', '▲', '▼',
}

// ScreenRune converts a byte from video memory: 00-1F and 7F are glyphs.
func (c *Codepage) ScreenRune(b byte) rune {
	switch {
	case b < 0x20:
		return glyphs[b]
	case b == 0x7F:
		return '⌂'
	}
	return c.m.DecodeByte(b)
}

// Rune converts a byte of text (files, clipboard): control bytes stay controls.
func (c *Codepage) Rune(b byte) rune { return c.m.DecodeByte(b) }

// Byte converts a rune to the code page; ok is false if it has no mapping.
func (c *Codepage) Byte(r rune) (byte, bool) {
	if r < 0x80 {
		return byte(r), true
	}
	return c.m.EncodeRune(r)
}

// Decode converts DOS text to a Go string.
func (c *Codepage) Decode(b []byte) string {
	var s strings.Builder
	for _, v := range b {
		s.WriteRune(c.Rune(v))
	}
	return s.String()
}

// Encode converts a Go string to DOS text; ok is false if a rune has no mapping.
func (c *Codepage) Encode(s string) ([]byte, bool) {
	out := make([]byte, 0, len(s))
	ok := true
	for _, r := range s {
		b, good := c.Byte(r)
		if !good {
			b, ok = '?', false
		}
		out = append(out, b)
	}
	return out, ok
}

// Upper upper-cases one DOS character, using Unicode rules for 80-FF.
func (c *Codepage) Upper(b byte) byte {
	if b < 0x80 {
		if b >= 'a' && b <= 'z' {
			return b - 32
		}
		return b
	}
	u := []rune(strings.ToUpper(string(c.Rune(b))))
	if len(u) == 1 {
		if v, ok := c.Byte(u[0]); ok {
			return v
		}
	}
	return b
}

// Detection is the result of Detect.
type Detection struct {
	Num    int    // chosen code page (always supported)
	Source string // where it came from, for diagnostics
	Note   string // set when the host asked for something without a table
}

// Detect picks the code page: the explicit override if nonzero, else the
// host's OEM code page as deduced by localecp, else 437.
func Detect(override int) (Detection, error) {
	if override != 0 {
		if _, err := Get(override); err != nil {
			return Detection{}, err
		}
		return Detection{Num: override, Source: "override"}, nil
	}
	// On Windows localecp reports the number itself (GetOEMCP, with the
	// legacy code page on UTF-8 systems); elsewhere only the encoding.
	if n := localecp.OEMCodepage; n != 0 {
		if _, ok := tables[n]; ok {
			return Detection{Num: n, Source: "host OEM code page"}, nil
		}
		return Detection{Num: 437, Source: "host OEM code page",
			Note: fmt.Sprintf("host code page %d has no DOS table, using 437", n)}, nil
	}
	for n, m := range tables {
		if localecp.OEMEncoding == encoding.Encoding(m) {
			return Detection{Num: n, Source: "host locale"}, nil
		}
	}
	return Detection{Num: 437, Source: "host locale",
		Note: fmt.Sprintf("locale maps to %v, which is not a DOS code page; using 437", localecp.OEMEncoding)}, nil
}
