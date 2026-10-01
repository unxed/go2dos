package machine

import (
	"strings"
	"testing"
)

func wrapMachine(t *testing.T) *Machine {
	t.Helper()
	m, err := New(Config{Drives: map[byte]string{'C': t.TempDir()}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func teletype(m *Machine, s string) {
	for i := 0; i < len(s); i++ {
		m.BIOS.Video.Teletype(s[i], 0)
	}
}

func longLine(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(byte('a' + i%26))
	}
	return b.String()
}

// A line that the teletype wrapped at the right edge comes out as one line;
// Line and TextRows still see two rows.
func TestWrapJoined(t *testing.T) {
	m := wrapMachine(t)
	long := longLine(100)
	teletype(m, long+"\r\nnext")
	s := m.BIOS.Video.Snapshot()
	lines := strings.Split(s.Text(), "\n")
	if lines[0] != long || lines[1] != "next" {
		t.Fatalf("text:\n%s", s.Text())
	}
	if !s.Wrapped[0] || s.Wrapped[1] {
		t.Errorf("Wrapped = %v", s.Wrapped[:3])
	}
	if s.Line(0) != long[:80] || s.Line(1) != long[80:] {
		t.Errorf("rows: %q, %q", s.Line(0), s.Line(1))
	}
	if rows := strings.Split(s.TextRows(), "\n"); len(rows) != 25 || rows[0] != long[:80] {
		t.Errorf("TextRows: %d rows, first %q", len(rows), rows[0])
	}
}

// The marks move with their rows when the screen scrolls.
func TestWrapScroll(t *testing.T) {
	m := wrapMachine(t)
	long := longLine(100)
	teletype(m, strings.Repeat("\r\n", 10)+long) // rows 10 and 11
	teletype(m, strings.Repeat("\r\n", 16))      // 13 lines down to the last row, then 3 scrolls
	s := m.BIOS.Video.Snapshot()
	if !s.Wrapped[7] || s.Line(7) != long[:80] {
		t.Fatalf("the wrapped line is not at row 7:\n%s", s.TextRows())
	}
	found := false
	for _, l := range strings.Split(s.Text(), "\n") {
		found = found || l == long
	}
	if !found {
		t.Errorf("no joined line after the scroll:\n%s", s.Text())
	}
}

// A row that is written to after the wrap is no longer a continuation.
func TestWrapOverwritten(t *testing.T) {
	m := wrapMachine(t)
	long := longLine(100)
	teletype(m, long)
	m.Mem.W8(0xB8000+10, 'Z') // a program draws on row 0, column 5
	s := m.BIOS.Video.Snapshot()
	if s.Wrapped[0] {
		t.Fatal("row 0 still marked as continued")
	}
	lines := strings.Split(s.Text(), "\n")
	if lines[0] != long[:5]+"Z"+long[6:80] || lines[1] != long[80:] {
		t.Fatalf("text:\n%s", s.Text())
	}
}

// Clearing the screen (a mode set) forgets the marks.
func TestWrapCleared(t *testing.T) {
	m := wrapMachine(t)
	teletype(m, longLine(100))
	m.BIOS.Video.SetTextSize(80, 25) // sets the mode again and clears the screen
	if s := m.BIOS.Video.Snapshot(); s.Wrapped[0] || s.Text() != strings.Repeat("\n", 24) {
		t.Errorf("screen not cleared: %v", s.Wrapped[:2])
	}
}
