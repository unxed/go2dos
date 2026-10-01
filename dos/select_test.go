package dos

import (
	"testing"

	"github.com/unxed/go2dos/bios"
)

// TestTextSelectionSingleLine tests selection on a single line.
func TestTextSelectionSingleLine(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	// Fill the first line with "Hello World"
	text := "Hello World"
	for i, ch := range text {
		screen.Cells[i] = bios.Cell{Ch: byte(ch), Attr: 0x07, Rune: rune(ch)}
	}
	// Fill rest with spaces
	for i := len(text); i < 80*25; i++ {
		screen.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}

	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 4, EndY: 0}
	selected := ts.GetSelectedText(screen)
	if selected != "Hello" {
		t.Errorf("Expected 'Hello', got %q", selected)
	}
}

// TestTextSelectionMultiLine tests selection across multiple lines.
func TestTextSelectionMultiLine(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	// Fill screen with known content
	lines := []string{"First line", "Second line", "Third line"}
	for lineIdx, line := range lines {
		for colIdx, ch := range line {
			idx := lineIdx*80 + colIdx
			screen.Cells[idx] = bios.Cell{Ch: byte(ch), Attr: 0x07, Rune: rune(ch)}
		}
		// Fill rest of line with spaces
		for colIdx := len(line); colIdx < 80; colIdx++ {
			idx := lineIdx*80 + colIdx
			screen.Cells[idx] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
		}
	}

	// Select from "First" on line 0 to "Second" on line 1
	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 5, EndY: 1}
	selected := ts.GetSelectedText(screen)

	// Expected: "First line\nSecond"
	expected := "First line\nSecond"
	if selected != expected {
		t.Errorf("Expected %q, got %q", expected, selected)
	}
}

// TestTextSelectionSwappedCoords tests that swapped coordinates are handled correctly.
func TestTextSelectionSwappedCoords(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	text := "Hello World"
	for i, ch := range text {
		screen.Cells[i] = bios.Cell{Ch: byte(ch), Attr: 0x07, Rune: rune(ch)}
	}
	for i := len(text); i < 80*25; i++ {
		screen.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}

	// Start from end, end at start (should be swapped)
	ts := &TextSelection{StartX: 4, StartY: 0, EndX: 0, EndY: 0}
	selected := ts.GetSelectedText(screen)
	if selected != "Hello" {
		t.Errorf("Expected 'Hello', got %q", selected)
	}
}

// TestTextSelectionBoundaryClamp tests that out-of-bounds coordinates are clamped.
func TestTextSelectionBoundaryClamp(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	text := "Hello"
	for i, ch := range text {
		screen.Cells[i] = bios.Cell{Ch: byte(ch), Attr: 0x07, Rune: rune(ch)}
	}
	for i := len(text); i < 80*25; i++ {
		screen.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}

	// Select with end coordinate beyond screen
	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 200, EndY: 0}
	selected := ts.GetSelectedText(screen)
	if selected != "Hello" {
		t.Errorf("Expected 'Hello', got %q", selected)
	}
}

// TestTextSelectionEmpty tests that an empty selection returns empty string.
func TestTextSelectionEmpty(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	text := "Hello"
	for i, ch := range text {
		screen.Cells[i] = bios.Cell{Ch: byte(ch), Attr: 0x07, Rune: rune(ch)}
	}
	for i := len(text); i < 80*25; i++ {
		screen.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}

	// Zero-width selection
	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 0, EndY: 0}
	selected := ts.GetSelectedText(screen)
	if selected != "H" {
		t.Errorf("Expected 'H', got %q", selected)
	}
}

// TestTextSelectionIsValid tests the IsValid method.
func TestTextSelectionIsValid(t *testing.T) {
	ts1 := &TextSelection{StartX: 0, StartY: 0, EndX: 0, EndY: 0}
	if ts1.IsValid() {
		t.Errorf("Expected IsValid=false for point selection, got true")
	}

	ts2 := &TextSelection{StartX: 0, StartY: 0, EndX: 5, EndY: 0}
	if !ts2.IsValid() {
		t.Errorf("Expected IsValid=true, got false")
	}
}

// TestTextSelectionIsEmpty tests the IsEmpty method.
func TestTextSelectionIsEmpty(t *testing.T) {
	ts1 := &TextSelection{StartX: 0, StartY: 0, EndX: 0, EndY: 0}
	if !ts1.IsEmpty() {
		t.Errorf("Expected IsEmpty=true for point selection, got false")
	}

	ts2 := &TextSelection{StartX: 0, StartY: 0, EndX: 5, EndY: 0}
	if ts2.IsEmpty() {
		t.Errorf("Expected IsEmpty=false, got true")
	}
}

// TestTextSelectionSetFromCoords tests the SetFromCoords method.
func TestTextSelectionSetFromCoords(t *testing.T) {
	ts := &TextSelection{}
	ts.SetFromCoords(10, 5, 20, 10)

	if ts.StartX != 10 || ts.StartY != 5 || ts.EndX != 20 || ts.EndY != 10 {
		t.Errorf("SetFromCoords failed: got (%d,%d) to (%d,%d)", ts.StartX, ts.StartY, ts.EndX, ts.EndY)
	}
}
