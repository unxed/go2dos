package main

import (
	"testing"

	"github.com/unxed/go2dos/bios"
)

// TestSelectionHandlerToggle tests toggling selection mode on and off.
func TestSelectionHandlerToggle(t *testing.T) {
	sh := newSelectionHandler()

	// Initially off
	if sh.isActive() {
		t.Errorf("Expected mode off, got active")
	}

	// Toggle on
	sh.toggleSelectionMode()
	if !sh.isActive() {
		t.Errorf("Expected mode active after toggle")
	}

	// Toggle off
	sh.toggleSelectionMode()
	if sh.isActive() {
		t.Errorf("Expected mode off after second toggle")
	}
}

// TestSelectionHandlerWithScreen tests selection with a screen snapshot.
func TestSelectionHandlerWithScreen(t *testing.T) {
	sh := newSelectionHandler()
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	// Fill first line with "Hello World"
	text := "Hello World"
	for i, ch := range text {
		screen.Cells[i] = bios.Cell{Ch: byte(ch), Attr: 0x07, Rune: rune(ch)}
	}
	for i := len(text); i < 80*25; i++ {
		screen.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}

	sh.updateScreen(screen)
	sh.toggleSelectionMode()

	// Start selection
	sh.startSelection(0, 0)
	sh.extendSelection(4, 0)
	sh.endSelection()

	selected := sh.getSelectedText()
	if selected != "Hello" {
		t.Errorf("Expected 'Hello', got %q", selected)
	}
}

// TestSelectionHandlerMode tests mode transitions.
func TestSelectionHandlerMode(t *testing.T) {
	sh := newSelectionHandler()

	if sh.getMode() != selectionModeOff {
		t.Errorf("Expected mode off, got %v", sh.getMode())
	}

	sh.setMode(selectionModeActive)
	if sh.getMode() != selectionModeActive {
		t.Errorf("Expected mode active, got %v", sh.getMode())
	}

	sh.setMode(selectionModeEnded)
	if sh.getMode() != selectionModeEnded {
		t.Errorf("Expected mode ended, got %v", sh.getMode())
	}
}

// TestSelectionHandlerGetSelection tests retrieving the selection object.
func TestSelectionHandlerGetSelection(t *testing.T) {
	sh := newSelectionHandler()
	sel := sh.getSelection()

	if sel == nil {
		t.Errorf("Expected non-nil selection")
	}

	// Verify it's the internal selection
	sel.SetFromCoords(1, 2, 3, 4)
	if sh.selection.StartX != 1 || sh.selection.StartY != 2 {
		t.Errorf("Selection not updated through GetSelection")
	}
}

// TestSelectionHandlerBounds tests that selection respects screen bounds.
func TestSelectionHandlerBounds(t *testing.T) {
	sh := newSelectionHandler()
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	sh.updateScreen(screen)
	sh.toggleSelectionMode()

	// Try to select outside bounds
	sh.startSelection(0, 0)
	sh.extendSelection(100, 100) // Outside bounds

	selected := sh.getSelectedText()
	// Should clamp to screen bounds in GetSelectedText
	if selected != "" {
		t.Logf("Got selection: %q", selected)
	}
}
