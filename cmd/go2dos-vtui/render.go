package main

import (
	"github.com/unxed/go2dos/bios"
	"github.com/unxed/vtui"
)

// cgaRGB — цвета текстового режима VGA (палитра по умолчанию), 24 бита.
var cgaRGB = [16]uint32{
	0x000000, 0x0000AA, 0x00AA00, 0x00AAAA, 0xAA0000, 0xAA00AA, 0xAA5500, 0xAAAAAA,
	0x555555, 0x5555FF, 0x55FF55, 0x55FFFF, 0xFF5555, 0xFF55FF, 0xFFFF55, 0xFFFFFF,
}

// attrColors переводит атрибут знакоместа в цвета текста и фона. Бит 7
// считается яркостью фона (мерцание выключено), как делает machine.SGR.
func attrColors(attr byte) (fg, bg uint32) {
	return cgaRGB[attr&0x0F], cgaRGB[attr>>4]
}

// renderScreen переносит снимок экрана машины в ScreenBuf: размер, ячейки,
// курсор. Графические режимы пока не показываются (снимок пуст).
func renderScreen(scr *vtui.ScreenBuf, s *bios.Screen) {
	if !s.TextMode() {
		return
	}
	scr.AllocBuf(s.Cols, s.Rows)
	row := make([]vtui.CharInfo, s.Cols)
	for y := 0; y < s.Rows; y++ {
		for x := range row {
			c := s.Cells[y*s.Cols+x]
			fg, bg := attrColors(c.Attr)
			ch := c.Rune
			if ch == 0 {
				ch = ' '
			}
			row[x] = vtui.CharInfo{Char: uint64(ch), Attributes: vtui.SetRGBBoth(0, fg, bg)}
		}
		scr.Write(0, y, row)
	}
	scr.SetCursorPos(s.CursorX, s.CursorY)
	scr.SetCursorVisible(s.CursorVisible)
	scr.Flush()
}
