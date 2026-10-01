package ui

import (
	"fmt"
	"image"
)

// Buffer is a 2D grid of cells that represents the terminal screen.
type Buffer struct {
	cells  []Cell
	width  int
	height int
}

// NewBuffer creates a new buffer with the specified dimensions.
func NewBuffer(width, height int) *Buffer {
	return &Buffer{
		cells:  make([]Cell, width*height),
		width:  width,
		height: height,
	}
}

// Width returns the width of the buffer.
func (b *Buffer) Width() int {
	return b.width
}

// Height returns the height of the buffer.
func (b *Buffer) Height() int {
	return b.height
}

// SetCell sets a cell at the given position.
func (b *Buffer) SetCell(x, y int, cell Cell) {
	if x >= 0 && x < b.width && y >= 0 && y < b.height {
		b.cells[y*b.width+x] = cell
	}
}

// GetCell gets a cell at the given position.
func (b *Buffer) GetCell(x, y int) Cell {
	if x >= 0 && x < b.width && y >= 0 && y < b.height {
		return b.cells[y*b.width+x]
	}
	return Cell{Rune: ' ', Foreground: 7, Background: 0}
}

// SetText sets text in the buffer at the given position.
func (b *Buffer) SetText(x, y int, text string, fg, bg uint8) {
	for i, ch := range text {
		if x+i >= b.width {
			break
		}
		b.SetCell(x+i, y, Cell{Rune: ch, Foreground: fg, Background: bg})
	}
}

// FillRect fills a rectangle with a cell.
func (b *Buffer) FillRect(rect image.Rectangle, cell Cell) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			b.SetCell(x, y, cell)
		}
	}
}

// DrawBorder draws a border around a rectangle.
func (b *Buffer) DrawBorder(rect image.Rectangle, fg, bg uint8) {
	// Corners
	b.SetCell(rect.Min.X, rect.Min.Y, Cell{Rune: '┌', Foreground: fg, Background: bg})
	b.SetCell(rect.Max.X-1, rect.Min.Y, Cell{Rune: '┐', Foreground: fg, Background: bg})
	b.SetCell(rect.Min.X, rect.Max.Y-1, Cell{Rune: '└', Foreground: fg, Background: bg})
	b.SetCell(rect.Max.X-1, rect.Max.Y-1, Cell{Rune: '┘', Foreground: fg, Background: bg})

	// Top and bottom
	for x := rect.Min.X + 1; x < rect.Max.X-1; x++ {
		b.SetCell(x, rect.Min.Y, Cell{Rune: '─', Foreground: fg, Background: bg})
		b.SetCell(x, rect.Max.Y-1, Cell{Rune: '─', Foreground: fg, Background: bg})
	}

	// Left and right
	for y := rect.Min.Y + 1; y < rect.Max.Y-1; y++ {
		b.SetCell(rect.Min.X, y, Cell{Rune: '│', Foreground: fg, Background: bg})
		b.SetCell(rect.Max.X-1, y, Cell{Rune: '│', Foreground: fg, Background: bg})
	}
}

// String renders the buffer as a string for debugging.
func (b *Buffer) String() string {
	var result string
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			cell := b.GetCell(x, y)
			result += string(cell.Rune)
		}
		if y < b.height-1 {
			result += "\n"
		}
	}
	return result
}

// Render outputs the buffer to a terminal using ANSI escape codes.
func (b *Buffer) Render() string {
	var result string
	var lastFg, lastBg uint8 = 255, 255

	for y := 0; y < b.height; y++ {
		result += fmt.Sprintf("\033[%d;0H", y+1)
		for x := 0; x < b.width; x++ {
			cell := b.GetCell(x, y)

			if cell.Foreground != lastFg || cell.Background != lastBg {
				result += fmt.Sprintf("\033[%d;%dm", 30+cell.Foreground%8, 40+cell.Background%8)
				lastFg = cell.Foreground
				lastBg = cell.Background
			}

			if cell.Rune == 0 {
				result += " "
			} else {
				result += string(cell.Rune)
			}
		}
	}

	return result
}
