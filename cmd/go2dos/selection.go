package main

import "github.com/unxed/go2dos/bios"

// selection is the keyboard selection mode of the terminal frontend
// (Ctrl-] s): the user moves a cursor over the screen, marks one corner of a
// rectangle, moves to the opposite corner and copies the text of the
// rectangle to the clipboards. The mouse is deliberately not captured: the
// terminal keeps its own selection (T06, docs/DOUBTS.md).
type selection struct {
	cols, rows int
	cx, cy     int // the cursor
	ax, ay     int // the marked corner, valid if anchored
	anchored   bool
}

// selResult is what a key press in the selection mode asks for.
type selResult int

const (
	selContinue selResult = iota // keep selecting
	selCopy                      // copy the rectangle and leave the mode
	selCancel                    // leave the mode
)

// Colors of the overlay (CGA attributes): the cursor cell, and the marked
// rectangle, which swaps foreground and background of the cells under it.
const (
	selCursorAttr = 0x4F
	selSwapMask   = 0x77
)

// newSelection starts the mode with the cursor on the program's text cursor.
func newSelection(s *bios.Screen) *selection {
	sl := &selection{cols: s.Cols, rows: s.Rows}
	sl.cx = min(max(s.CursorX, 0), s.Cols-1)
	sl.cy = min(max(s.CursorY, 0), s.Rows-1)
	return sl
}

// rect returns the selected rectangle, corners included: the marked corner
// and the cursor in any order, or just the cursor cell before a corner is marked.
func (sl *selection) rect() (x0, y0, x1, y1 int) {
	if !sl.anchored {
		return sl.cx, sl.cy, sl.cx, sl.cy
	}
	return min(sl.ax, sl.cx), min(sl.ay, sl.cy), max(sl.ax, sl.cx), max(sl.ay, sl.cy)
}

// attrAt is the attribute that cell (x, y) is drawn with; a nil selection
// changes nothing.
func (sl *selection) attrAt(x, y int, a byte) byte {
	if sl == nil {
		return a
	}
	if x == sl.cx && y == sl.cy {
		return selCursorAttr
	}
	x0, y0, x1, y1 := sl.rect()
	if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
		return a ^ selSwapMask
	}
	return a
}

// key handles one key press: arrows, Home/End (row ends), PgUp/PgDn (first
// and last row) move the cursor; Space or Enter marks the first corner and,
// pressed again, asks for the copy; Esc cancels. Other keys are ignored.
func (sl *selection) key(k bios.KeyEvent) selResult {
	ext := k.ASCII == 0 || k.ASCII == 0xE0 // not a numeric keypad digit
	switch {
	case k.Scan == 0x01:
		return selCancel
	case k.Scan == 0x1C || k.Scan == 0x39:
		if !sl.anchored {
			sl.ax, sl.ay, sl.anchored = sl.cx, sl.cy, true
			return selContinue
		}
		return selCopy
	case !ext:
	case k.Scan == 0x4B:
		sl.cx = max(sl.cx-1, 0)
	case k.Scan == 0x4D:
		sl.cx = min(sl.cx+1, sl.cols-1)
	case k.Scan == 0x48:
		sl.cy = max(sl.cy-1, 0)
	case k.Scan == 0x50:
		sl.cy = min(sl.cy+1, sl.rows-1)
	case k.Scan == 0x47:
		sl.cx = 0
	case k.Scan == 0x4F:
		sl.cx = sl.cols - 1
	case k.Scan == 0x49:
		sl.cy = 0
	case k.Scan == 0x51:
		sl.cy = sl.rows - 1
	}
	return selContinue
}
