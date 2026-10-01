package main

import (
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/vtui"
)

func TestAttrColors(t *testing.T) {
	cases := []struct {
		attr   byte
		fg, bg uint32
	}{
		{0x07, 0xAAAAAA, 0x000000}, // серый на чёрном
		{0x1F, 0xFFFFFF, 0x0000AA}, // белый на синем
		{0x8E, 0xFFFF55, 0x555555}, // бит 7: яркий фон
		{0x6C, 0xFF5555, 0xAA5500}, // коричневый фон
	}
	for _, c := range cases {
		fg, bg := attrColors(c.attr)
		if fg != c.fg || bg != c.bg {
			t.Errorf("attr %02X: got %06X/%06X, want %06X/%06X", c.attr, fg, bg, c.fg, c.bg)
		}
	}
}

func TestRenderScreen(t *testing.T) {
	s := &bios.Screen{Mode: 3, Cols: 4, Rows: 2, CursorX: 2, CursorY: 1, CursorVisible: true}
	s.Cells = make([]bios.Cell, 8)
	for i := range s.Cells {
		s.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}
	s.Cells[1] = bios.Cell{Ch: 0xB0, Attr: 0x1F, Rune: '░'}
	s.Cells[4] = bios.Cell{Ch: 1, Attr: 0x4E, Rune: '☺'}
	s.Cells[5] = bios.Cell{Attr: 0x07} // нулевая руна показывается пробелом

	scr := vtui.NewSilentScreenBuf()
	renderScreen(scr, s)
	if scr.Width() != 4 || scr.Height() != 2 {
		t.Fatalf("size %dx%d, want 4x2", scr.Width(), scr.Height())
	}
	for _, c := range []struct {
		x, y   int
		r      rune
		fg, bg uint32
	}{
		{1, 0, '░', 0xFFFFFF, 0x0000AA},
		{0, 1, '☺', 0xFFFF55, 0xAA0000},
		{1, 1, ' ', 0xAAAAAA, 0x000000},
		{3, 1, ' ', 0xAAAAAA, 0x000000},
	} {
		ci := scr.GetCell(c.x, c.y)
		if rune(ci.Char) != c.r || vtui.GetRGBFore(ci.Attributes) != c.fg || vtui.GetRGBBack(ci.Attributes) != c.bg {
			t.Errorf("cell (%d,%d): rune %q fg %06X bg %06X, want %q %06X %06X",
				c.x, c.y, rune(ci.Char), vtui.GetRGBFore(ci.Attributes), vtui.GetRGBBack(ci.Attributes), c.r, c.fg, c.bg)
		}
	}
	if x, y, vis, _ := scr.GetCursorStateForTesting(); x != 2 || y != 1 || !vis {
		t.Errorf("cursor (%d,%d) visible=%v, want (2,1) true", x, y, vis)
	}

	// Смена размера снимка меняет размер буфера.
	renderScreen(scr, &bios.Screen{Mode: 3, Cols: 2, Rows: 1, Cells: make([]bios.Cell, 2)})
	if scr.Width() != 2 || scr.Height() != 1 {
		t.Errorf("after resize: %dx%d, want 2x1", scr.Width(), scr.Height())
	}
	// Графический режим (Cols == 0) буфер не трогает.
	renderScreen(scr, &bios.Screen{Mode: 0x13})
	if scr.Width() != 2 {
		t.Errorf("a graphics snapshot changed the buffer")
	}
}
