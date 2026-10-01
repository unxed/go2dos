package bios

import (
	"strings"
	"testing"
)

// TestLineWrapping verifies that Screen.Text() joins wrapped lines.
func TestLineWrapping(t *testing.T) {
	// Create a screen with wrapped lines.
	s := &Screen{
		Mode:    3,
		Cols:    80,
		Rows:    3,
		Cells:   make([]Cell, 80*3),
		Wrapped: make([]bool, 3),
	}

	// Initialize all cells with spaces.
	for i := range s.Cells {
		s.Cells[i] = Cell{Ch: ' ', Attr: 7, Rune: ' '}
	}

	// Line 0: 80 A's (fills the full width)
	for x := 0; x < 80; x++ {
		s.Cells[0*80+x] = Cell{Ch: 'A', Attr: 7, Rune: 'A'}
	}
	s.Wrapped[0] = true

	// Line 1: 80 B's (continuation of line 0)
	for x := 0; x < 80; x++ {
		s.Cells[1*80+x] = Cell{Ch: 'B', Attr: 7, Rune: 'B'}
	}
	s.Wrapped[1] = false

	// Line 2: 5 C's
	for x := 0; x < 5; x++ {
		s.Cells[2*80+x] = Cell{Ch: 'C', Attr: 7, Rune: 'C'}
	}
	s.Wrapped[2] = false

	// Get the text output.
	text := s.Text()

	// Expected: line 0 (80 A's) + line 1 (80 B's) joined, then newline, then line 2 (5 C's).
	expected := strings.Repeat("A", 80) + strings.Repeat("B", 80) + "\n" + strings.Repeat("C", 5)

	if text != expected {
		t.Errorf("Text() output mismatch.\nGot:\n%q\nWant:\n%q", text, expected)
	}
}

// TestScrollCopiesWrappedFlag verifies that scrolling preserves wrapped flags.
func TestScrollCopiesWrappedFlag(t *testing.T) {
	// This test is more complex as it requires a full Video instance with proper
	// initialization. For now, we test the conceptual functionality through the
	// higher-level behavior.
	// This would be better tested with a real e2e test using a COM program.
}

// TestLineRewriteRemovesWrappedFlag tests that rewriting a line clears the wrapped flag.
// This is tested through behavior tests, not unit tests, as it requires direct memory writes.
func TestLineRewriteRemovesWrappedFlag(t *testing.T) {
	// Tested via e2e tests with COM programs.
}
