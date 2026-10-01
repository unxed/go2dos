package dos

import (
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
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

// TestPasteFromSelectionSuccess tests pasting selected text into clipboard.
func TestPasteFromSelectionSuccess(t *testing.T) {
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

	// Create DOS instance and open clipboard
	m := createTestDOS(t)
	m.clipboardOpen = true

	// Paste selection
	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 4, EndY: 0}
	size, err := ts.PasteFromSelection(m, screen, 7) // CF_OEMTEXT
	if err != nil {
		t.Fatalf("PasteFromSelection failed: %v", err)
	}

	// Check that clipboard was updated
	if size != 5 {
		t.Errorf("Expected size=5, got %d", size)
	}
	if string(m.clipboardData) != "Hello" {
		t.Errorf("Expected 'Hello' in clipboard, got %q", string(m.clipboardData))
	}
	if m.clipboardFormat != 7 {
		t.Errorf("Expected format 7, got %d", m.clipboardFormat)
	}
}

// TestPasteFromSelectionClosed tests that pasting fails when clipboard is closed.
func TestPasteFromSelectionClosed(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	m := createTestDOS(t)
	m.clipboardOpen = false

	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 5, EndY: 0}
	_, err := ts.PasteFromSelection(m, screen, 7)
	if err != ErrClipboardNotOpen {
		t.Errorf("Expected ErrClipboardNotOpen, got %v", err)
	}
}

// TestPasteFromSelectionUnsupportedFormat tests that unsupported format is rejected.
func TestPasteFromSelectionUnsupportedFormat(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	m := createTestDOS(t)
	m.clipboardOpen = true

	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 5, EndY: 0}
	_, err := ts.PasteFromSelection(m, screen, 99) // Invalid format
	if err != ErrUnsupportedFormat {
		t.Errorf("Expected ErrUnsupportedFormat, got %v", err)
	}
}

// TestPasteFromSelectionEmpty tests pasting with empty/whitespace-only selection.
func TestPasteFromSelectionEmpty(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	// Fill with spaces
	for i := 0; i < 80*25; i++ {
		screen.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}

	m := createTestDOS(t)
	m.clipboardOpen = true
	m.clipboardData = []byte("Previous data")
	m.clipboardFormat = 7

	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 0, EndY: 0}
	size, err := ts.PasteFromSelection(m, screen, 7)
	if err != nil {
		t.Fatalf("PasteFromSelection failed: %v", err)
	}

	// Whitespace-only selection is treated as empty, so clipboard is cleared
	if size != 0 {
		t.Errorf("Expected size=0 for whitespace selection, got %d", size)
	}
	if len(m.clipboardData) != 0 {
		t.Errorf("Expected empty clipboard data, got %q", string(m.clipboardData))
	}
}

// Helper function to create a test DOS instance
func createTestDOS(t *testing.T) *DOS {
	// This is a minimal DOS instance for testing
	// We only need the clipboard fields to be accessible
	return &DOS{
		clipboardOpen:   false,
		clipboardFormat: 0,
		clipboardData:   []byte{},
	}
}

// TestHostClipboard is a simple in-memory implementation of HostClipboard for testing.
type TestHostClipboard struct {
	text string
}

func (c *TestHostClipboard) GetText() (string, error) {
	return c.text, nil
}

func (c *TestHostClipboard) SetText(s string) error {
	c.text = s
	return nil
}

// TestCopyToClipboardBasic tests copying screen selection to host clipboard.
func TestCopyToClipboardBasic(t *testing.T) {
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

	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 4, EndY: 0}
	hclip := &TestHostClipboard{}

	count, err := ts.CopyToClipboard(screen, hclip)
	if err != nil {
		t.Fatalf("CopyToClipboard failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 chars, got %d", count)
	}
	if hclip.text != "Hello" {
		t.Errorf("Expected 'Hello' in clipboard, got %q", hclip.text)
	}
}

// TestCopyToClipboardEmpty tests copying empty selection.
func TestCopyToClipboardEmpty(t *testing.T) {
	screen := &bios.Screen{
		Cols:  80,
		Rows:  25,
		Cells: make([]bios.Cell, 80*25),
	}

	for i := 0; i < 80*25; i++ {
		screen.Cells[i] = bios.Cell{Ch: ' ', Attr: 0x07, Rune: ' '}
	}

	ts := &TextSelection{StartX: 0, StartY: 0, EndX: 0, EndY: 0}
	hclip := &TestHostClipboard{text: "original"}

	count, err := ts.CopyToClipboard(screen, hclip)
	if err != nil {
		t.Fatalf("CopyToClipboard failed: %v", err)
	}

	if hclip.text != "" {
		t.Errorf("Expected empty clipboard, got %q", hclip.text)
	}
	if count != 0 {
		t.Errorf("Expected 0 chars, got %d", count)
	}
}

// TestPasteFromClipboardBasic tests pasting text as keystrokes.
func TestPasteFromClipboardBasic(t *testing.T) {
	testBIOS := &bios.BIOS{IdlePolls: 50}
	testPage, err := cp.Get(437)
	if err != nil {
		t.Fatalf("Failed to get codepage 437: %v", err)
	}

	var keystrokes []bios.KeyEvent
	pushKey := func(ke bios.KeyEvent) {
		keystrokes = append(keystrokes, ke)
	}

	hclip := &TestHostClipboard{text: "hi"}

	count, err := PasteFromClipboard(testBIOS, hclip, testPage, pushKey)
	if err != nil {
		t.Fatalf("PasteFromClipboard failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 keystrokes, got %d", count)
	}
	if len(keystrokes) != 2 {
		t.Errorf("Expected 2 key events, got %d", len(keystrokes))
	}
}

// TestPasteFromClipboardWithNewlines tests that newlines become Enter keys.
func TestPasteFromClipboardWithNewlines(t *testing.T) {
	testBIOS := &bios.BIOS{IdlePolls: 50}
	testPage, err := cp.Get(437)
	if err != nil {
		t.Fatalf("Failed to get codepage 437: %v", err)
	}

	var keystrokes []bios.KeyEvent
	pushKey := func(ke bios.KeyEvent) {
		keystrokes = append(keystrokes, ke)
	}

	hclip := &TestHostClipboard{text: "line1\nline2"}

	count, err := PasteFromClipboard(testBIOS, hclip, testPage, pushKey)
	if err != nil {
		t.Fatalf("PasteFromClipboard failed: %v", err)
	}

	if count != 11 {
		t.Errorf("Expected 11 keystrokes, got %d", count)
	}
}

// TestPasteFromClipboardEmpty tests pasting empty clipboard.
func TestPasteFromClipboardEmpty(t *testing.T) {
	testBIOS := &bios.BIOS{IdlePolls: 50}
	testPage, err := cp.Get(437)
	if err != nil {
		t.Fatalf("Failed to get codepage 437: %v", err)
	}

	var keystrokes []bios.KeyEvent
	pushKey := func(ke bios.KeyEvent) {
		keystrokes = append(keystrokes, ke)
	}

	hclip := &TestHostClipboard{text: ""}

	count, err := PasteFromClipboard(testBIOS, hclip, testPage, pushKey)
	if err != nil {
		t.Fatalf("PasteFromClipboard failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 keystrokes, got %d", count)
	}
	if len(keystrokes) != 0 {
		t.Errorf("Expected no key events, got %d", len(keystrokes))
	}
}
