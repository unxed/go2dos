package bios

import "testing"

func testScreen(rows ...string) *Screen {
	s := &Screen{Mode: 3, Cols: len([]rune(rows[0])), Rows: len(rows)}
	for _, r := range rows {
		for _, c := range r {
			s.Cells = append(s.Cells, Cell{Rune: c})
		}
	}
	return s
}

// A rectangle of the snapshot becomes text: corners in any order, clamped to
// the screen, trailing blanks of every row removed.
func TestSelection(t *testing.T) {
	s := testScreen(
		"Hello world   ",
		"second line   ",
		"third         ",
	)
	cases := []struct {
		x0, y0, x1, y1 int
		want           string
	}{
		{6, 0, 10, 0, "world"},
		{0, 0, 5, 1, "Hello\nsecond"},
		{5, 1, 0, 0, "Hello\nsecond"}, // corners the other way round
		{7, 1, 20, 5, "line\n"},       // clamped; the third row is empty in that range
		{0, 2, 13, 2, "third"},        // trailing blanks go
		{-5, -5, 1, 0, "He"},          // clamped at the top left
		{20, 0, 25, 1, ""},            // outside
	}
	for _, c := range cases {
		if got := s.Selection(c.x0, c.y0, c.x1, c.y1); got != c.want {
			t.Errorf("Selection(%d,%d,%d,%d) = %q, want %q", c.x0, c.y0, c.x1, c.y1, got, c.want)
		}
	}
	if got := (&Screen{}).Selection(0, 0, 1, 1); got != "" {
		t.Errorf("a graphics screen: %q", got)
	}
}
