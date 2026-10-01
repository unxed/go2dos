package dos

import (
	"strings"
	"unicode"

	"github.com/unxed/go2dos/bios"
)

// Real names on the screen (T23, docs/NAMES.md "Показ настоящих имён").
//
// A program that does not know UTF-8 shows a name that the code page cannot hold
// under its alias ("___~A1B2 txt"). The screen the hosts get is a snapshot of video
// memory; DisplayNames replaces the cells of such an alias by the real name (only the
// Rune of the cell: the bytes in video memory, and so the program, do not change). The
// alias is recognised by the registry (names.go), so only names that were really
// handed out are replaced; a real file called "ABC~A1B2.TXT" is not touched.
//
// Layouts: the dotted name "STEM~HHHH.EXT" (status lines, dialogs), the panel columns of
// Volkov Commander "STEM~HHHH ext" (the base field of 8 cells, a blank, the extension
// field of 3 cells), and the name without extension. A long real name does not fit the
// cells of its alias: it is cut and ends with "…". Characters that take other than one
// cell on a terminal (wide, combining, unprintable) are shown as '?'.

// DisplayNames replaces aliases in the snapshot by the real names. Call it from the
// goroutine that runs the machine (it reads the registry).
func (d *DOS) DisplayNames(s *bios.Screen) {
	if d.fs.names == nil || d.fs.utf8() || !s.TextMode() {
		return
	}
	for y := 0; y < s.Rows; y++ {
		d.displayRow(s.Cells[y*s.Cols : (y+1)*s.Cols])
	}
}

func isHexByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'F' || c >= 'a' && c <= 'f'
}

func (d *DOS) displayRow(row []bios.Cell) {
	n := len(row)
	for x := 0; x+5 <= n; x++ {
		if row[x].Ch != '~' || !isHexByte(row[x+1].Ch) || !isHexByte(row[x+2].Ch) ||
			!isHexByte(row[x+3].Ch) || !isHexByte(row[x+4].Ch) {
			continue
		}
		start, end, rec, panel := d.matchAlias(row, x)
		if rec == nil {
			continue
		}
		if panel {
			d.putPanel(row[start:end], rec)
		} else {
			putFit(row[start:end], rec.host)
		}
		x = end - 1
	}
}

func cellBytes(row []bios.Cell, from, to int) string {
	b := make([]byte, 0, to-from)
	for i := from; i < to; i++ {
		b = append(b, row[i].Ch)
	}
	return string(b)
}

// matchAlias looks for a registered alias whose "~HHHH" starts at row[x].
func (d *DOS) matchAlias(row []bios.Cell, x int) (start, end int, rec *nameRec, panel bool) {
	reg := d.fs.names
	hash := cellBytes(row, x, x+5)
	p := x + 5
	for k := 3; k >= 0; k-- { // the stem: up to 3 cells before the tilde, the longest first
		start = x - k
		if start < 0 {
			continue
		}
		r := reg.byShort[d.fs.key(cellBytes(row, start, x)+hash)]
		if r == nil {
			continue
		}
		// Dotted: STEM~HHHH.EXT
		if p < len(row) && row[p].Ch == '.' {
			for l := 3; l >= 0; l-- {
				if p+1+l <= len(row) && d.fs.key(cellBytes(row, p+1, p+1+l)) == d.fs.key(r.shortExt) &&
					(p+1+l == len(row) || row[p+1+l].Ch == ' ' || l == 3) {
					return start, p + 1 + l, r, false
				}
			}
			continue
		}
		// Panel columns: base field of 8, a blank, the extension field of 3.
		if start+12 <= len(row) && row[start+8].Ch == ' ' &&
			strings.TrimRight(cellBytes(row, p, start+8), " ") == "" {
			ext := strings.TrimRight(cellBytes(row, start+9, start+12), " ")
			if d.fs.key(ext) == d.fs.key(r.shortExt) {
				return start, start + 12, r, true
			}
		}
		// A name without extension.
		if r.shortExt == "" {
			return start, p, r, false
		}
	}
	return 0, 0, nil, false
}

// narrow is the rune a terminal shows in one cell, or '?'.
func narrow(r rune) rune {
	switch {
	case r < 0x20 || r == 0x7F || !unicode.IsPrint(r),
		unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc),
		r >= 0x1100 && r <= 0x115F, r >= 0x2E80 && r <= 0xA4CF, r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF, r >= 0xFE30 && r <= 0xFE6F, r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6, r >= 0x1F300 && r <= 0x1FAFF, r >= 0x20000:
		return '?'
	}
	return r
}

// fitRunes returns s cut to w cells (the last one is "…" if it did not fit) and
// padded with blanks.
func fitRunes(s string, w int) []rune {
	rs := []rune(s)
	if len(rs) > w {
		rs = append(rs[:max(w-1, 0)], '…')
	}
	for i := range rs {
		rs[i] = narrow(rs[i])
	}
	for len(rs) < w {
		rs = append(rs, ' ')
	}
	return rs
}

func putFit(cells []bios.Cell, name string) {
	for i, r := range fitRunes(name, len(cells)) {
		cells[i].Rune = r
	}
}

func (d *DOS) putPanel(cells []bios.Cell, r *nameRec) {
	base := fitRunes(r.hostBase, 8)
	ext := fitRunes(r.hostExt, 3)
	for i, c := range base {
		cells[i].Rune = c
	}
	cells[8].Rune = ' '
	for i, c := range ext {
		cells[9+i].Rune = c
	}
}
