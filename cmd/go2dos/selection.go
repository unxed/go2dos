package main

import (
	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/dos"
)

// selectionMode represents the current text selection mode.
type selectionMode int

const (
	selectionModeOff selectionMode = iota
	selectionModeActive
	selectionModeEnded
)

// selectionHandler manages text selection on the screen.
type selectionHandler struct {
	mode     selectionMode
	selection *dos.TextSelection
	screen   *bios.Screen // last screen snapshot
}

// newSelectionHandler creates a new selection handler.
func newSelectionHandler() *selectionHandler {
	return &selectionHandler{
		mode:      selectionModeOff,
		selection: &dos.TextSelection{},
	}
}

// startSelection begins a text selection at the given screen coordinates.
func (sh *selectionHandler) startSelection(x, y int) {
	if sh.mode == selectionModeOff || sh.screen == nil {
		return
	}
	if y >= sh.screen.Rows || x >= sh.screen.Cols {
		return
	}
	sh.selection.SetFromCoords(x, y, x, y)
}

// extendSelection extends the current selection to the given coordinates.
func (sh *selectionHandler) extendSelection(x, y int) {
	if sh.mode != selectionModeActive || sh.screen == nil {
		return
	}
	if y >= sh.screen.Rows || x >= sh.screen.Cols {
		return
	}
	sh.selection.EndX = x
	sh.selection.EndY = y
}

// endSelection stops the current selection.
func (sh *selectionHandler) endSelection() {
	if sh.mode == selectionModeActive {
		sh.mode = selectionModeEnded
	}
}

// toggleSelectionMode toggles between selection on and off.
func (sh *selectionHandler) toggleSelectionMode() {
	if sh.mode == selectionModeOff {
		sh.mode = selectionModeActive
	} else {
		sh.mode = selectionModeOff
		sh.selection = &dos.TextSelection{}
	}
}

// getSelectedText returns the currently selected text, or empty string if no selection.
func (sh *selectionHandler) getSelectedText() string {
	if sh.screen == nil {
		return ""
	}
	return sh.selection.GetSelectedText(sh.screen)
}

// updateScreen updates the stored screen snapshot.
func (sh *selectionHandler) updateScreen(screen *bios.Screen) {
	sh.screen = screen
}

// isActive reports whether selection mode is currently active.
func (sh *selectionHandler) isActive() bool {
	return sh.mode == selectionModeActive
}

// setMode sets the selection mode directly.
func (sh *selectionHandler) setMode(mode selectionMode) {
	sh.mode = mode
}

// getMode returns the current selection mode.
func (sh *selectionHandler) getMode() selectionMode {
	return sh.mode
}

// getSelection returns the current TextSelection object.
func (sh *selectionHandler) getSelection() *dos.TextSelection {
	return sh.selection
}
