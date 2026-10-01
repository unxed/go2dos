package dos

import (
	"errors"
	"strings"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/keys"
)

// Clipboard-related errors
var (
	ErrClipboardNotOpen    = errors.New("clipboard is not open")
	ErrUnsupportedFormat   = errors.New("unsupported clipboard format")
	ErrPasteInvalidCoords  = errors.New("invalid paste coordinates")
	ErrPasteOutOfBounds    = errors.New("paste operation out of screen bounds")
)

// TextSelection represents a text selection on the screen with start and end coordinates.
// Coordinates are (column, row) where (0,0) is top-left.
type TextSelection struct {
	StartX, StartY int // selection start
	EndX, EndY     int // selection end
}

// GetSelectedText returns the text selected by the TextSelection from the given screen snapshot.
// If the selection is empty or invalid, returns an empty string.
// The selection spans from (StartX, StartY) to (EndX, EndY) inclusive.
// If start is after end, they are swapped.
func (ts *TextSelection) GetSelectedText(screen *bios.Screen) string {
	if !screen.TextMode() {
		return ""
	}

	// Ensure start <= end
	startX, startY := ts.StartX, ts.StartY
	endX, endY := ts.EndX, ts.EndY

	if startY > endY {
		startY, endY = endY, startY
		startX, endX = endX, startX
	} else if startY == endY && startX > endX {
		startX, endX = endX, startX
	}

	// Clamp coordinates to screen bounds
	if startY < 0 {
		startY = 0
		startX = 0
	}
	if startY >= screen.Rows {
		return ""
	}
	if endY >= screen.Rows {
		endY = screen.Rows - 1
	}
	if endX >= screen.Cols {
		endX = screen.Cols - 1
	}
	if startX < 0 {
		startX = 0
	}
	if startX >= screen.Cols {
		startX = screen.Cols - 1
	}

	var lines []string

	// Single line selection
	if startY == endY {
		var result []rune
		for x := startX; x <= endX; x++ {
			cell := screen.Cells[startY*screen.Cols+x]
			result = append(result, cell.Rune)
		}
		return strings.TrimRight(string(result), " ")
	}

	// Multi-line selection
	// First line: from startX to end of line
	var firstLine []rune
	for x := startX; x < screen.Cols; x++ {
		cell := screen.Cells[startY*screen.Cols+x]
		firstLine = append(firstLine, cell.Rune)
	}
	lines = append(lines, strings.TrimRight(string(firstLine), " "))

	// Middle lines: entire lines
	for y := startY + 1; y < endY; y++ {
		var middleLine []rune
		for x := 0; x < screen.Cols; x++ {
			cell := screen.Cells[y*screen.Cols+x]
			middleLine = append(middleLine, cell.Rune)
		}
		lines = append(lines, strings.TrimRight(string(middleLine), " "))
	}

	// Last line: from start of line to endX
	var lastLine []rune
	for x := 0; x <= endX; x++ {
		cell := screen.Cells[endY*screen.Cols+x]
		lastLine = append(lastLine, cell.Rune)
	}
	lines = append(lines, strings.TrimRight(string(lastLine), " "))

	return strings.Join(lines, "\n")
}

// IsValid reports whether the selection has at least one character.
func (ts *TextSelection) IsValid() bool {
	return ts.StartX != ts.EndX || ts.StartY != ts.EndY
}

// IsEmpty reports whether the selection has zero area.
func (ts *TextSelection) IsEmpty() bool {
	return ts.StartX == ts.EndX && ts.StartY == ts.EndY
}

// SetFromCoords sets the selection from start and end coordinates.
// This is a convenience method for setting both start and end at once.
func (ts *TextSelection) SetFromCoords(startX, startY, endX, endY int) {
	ts.StartX = startX
	ts.StartY = startY
	ts.EndX = endX
	ts.EndY = endY
}

// PasteFromSelection puts the selected text into the DOS clipboard buffer.
// Returns the number of bytes written to the clipboard, or an error if the
// clipboard is not open or the selection is invalid.
func (ts *TextSelection) PasteFromSelection(d *DOS, screen *bios.Screen, format uint16) (int, error) {
	// Validate format
	if format != 1 && format != 7 {
		return 0, ErrUnsupportedFormat
	}

	// Check if clipboard is open
	if !d.clipboardOpen {
		return 0, ErrClipboardNotOpen
	}

	// Get selected text from screen
	text := ts.GetSelectedText(screen)
	if text == "" {
		// Empty selection - clear the clipboard
		d.clipboardData = []byte{}
		d.clipboardFormat = 0
		return 0, nil
	}

	// Convert text to clipboard format
	data := []byte(text)

	// Store in clipboard
	d.clipboardData = make([]byte, len(data))
	copy(d.clipboardData, data)
	d.clipboardFormat = format

	return len(data), nil
}

// HostClipboard is the interface for the host's clipboard (for copy/paste operations).
type HostClipboard interface {
	GetText() (string, error)
	SetText(string) error
}

// CopyToClipboard copies the selected text from the screen to the host clipboard.
// Returns the number of characters copied, or an error if the operation fails.
func (ts *TextSelection) CopyToClipboard(screen *bios.Screen, hclip HostClipboard) (int, error) {
	if hclip == nil {
		return 0, errors.New("host clipboard is nil")
	}

	text := ts.GetSelectedText(screen)
	if text == "" {
		// Empty selection - set empty text
		if err := hclip.SetText(""); err != nil {
			return 0, err
		}
		return 0, nil
	}

	if err := hclip.SetText(text); err != nil {
		return 0, err
	}

	return len([]rune(text)), nil
}

// PasteFromClipboard pastes text from the host clipboard as keystrokes into the
// BIOS keyboard buffer, in chunks of at most 15 keystrokes (the BIOS buffer size).
// The text is converted to the given code page, and newlines become Enter keystrokes.
// It returns the number of keystrokes injected, or an error if the operation fails.
func PasteFromClipboard(b *bios.BIOS, hclip HostClipboard, page *cp.Codepage, pushKey func(bios.KeyEvent)) (int, error) {
	if b == nil {
		return 0, errors.New("BIOS is nil")
	}
	if hclip == nil {
		return 0, errors.New("host clipboard is nil")
	}
	if page == nil {
		return 0, errors.New("code page is nil")
	}

	text, err := hclip.GetText()
	if err != nil {
		return 0, err
	}

	if text == "" {
		return 0, nil
	}

	count := 0
	chunk := 0
	const chunkSize = 15

	for _, r := range text {
		if chunk >= chunkSize {
			chunk = 0
		}

		if r == '\n' {
			if ke, ok := keys.Named("Enter", 0); ok {
				pushKey(ke)
				count++
				chunk++
			}
		} else if r == '\r' {
			continue
		} else {
			if ke, ok := keys.Char(r, page); ok {
				pushKey(ke)
				count++
				chunk++
			}
		}
	}

	return count, nil
}
