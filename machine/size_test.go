package machine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runFrame runs frame.com, which draws a frame by the size in the BDA.
func runFrame(t *testing.T, cols, rows int) *Machine {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "frame.com"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FRAME.COM"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437, Cols: cols, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\FRAME.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d", c)
	}
	return m
}

func checkFrame(t *testing.T, m *Machine, cols, rows int) {
	t.Helper()
	s := m.Screen()
	if s.Cols != cols || s.Rows != rows {
		t.Fatalf("snapshot %dx%d, want %dx%d", s.Cols, s.Rows, cols, rows)
	}
	edge := "+" + strings.Repeat("-", cols-2) + "+"
	mid := "|" + strings.Repeat(" ", cols-2) + "|"
	for y := 0; y < rows; y++ {
		want := mid
		if y == 0 || y == rows-1 {
			want = edge
		}
		// Line trims trailing blanks only; both patterns end in a visible character.
		if got := s.Line(y); got != want {
			t.Fatalf("row %d = %q, want %q", y, got, want)
		}
	}
	// What the BIOS and the CRTC tell a program.
	if got := m.Mem.R16(0x44A); int(got) != cols {
		t.Errorf("0040:004A = %d, want %d", got, cols)
	}
	if got := m.Mem.R8(0x484); int(got) != rows-1 {
		t.Errorf("0040:0084 = %d, want %d", got, rows-1)
	}
	crtc := func(i byte) byte {
		m.BIOS.Video.Out(0x3D4, i)
		b, _ := m.BIOS.Video.In(0x3D5)
		return b
	}
	if got := crtc(0x01); int(got) != cols-1 {
		t.Errorf("CRTC horizontal displayed = %d, want %d", got, cols-1)
	}
	if got := crtc(0x13); int(got) != cols/2 {
		t.Errorf("CRTC offset = %d, want %d", got, cols/2)
	}
	height := 16
	if rows > 25 {
		height = 8
	}
	lines := rows*height - 1
	if got := int(crtc(0x12)) | int(crtc(0x07)>>1&1)<<8 | int(crtc(0x07)>>6&1)<<9; got != lines {
		t.Errorf("CRTC vertical display end = %d, want %d", got, lines)
	}
	if want := (cols*rows*2 + 0xFF) &^ 0xFF; int(m.Mem.R16(0x44C)) != want {
		t.Errorf("0040:004C = %04X, want %04X", m.Mem.R16(0x44C), want)
	}
}

func TestSize80x25(t *testing.T) { checkFrame(t, runFrame(t, 0, 0), 80, 25) }

func TestSize100x40(t *testing.T) { checkFrame(t, runFrame(t, 100, 40), 100, 40) }

func TestSize132x43(t *testing.T) { checkFrame(t, runFrame(t, 132, 43), 132, 43) }

// 250x100 cells take 50000 bytes: the last rows are in C000h-C7FFFh, the
// part of the video window above the old 32 KiB.
func TestSizeBeyond32K(t *testing.T) { checkFrame(t, runFrame(t, 250, 100), 250, 100) }

func TestSizeTooBig(t *testing.T) {
	if _, err := New(Config{Drives: map[byte]string{'C': t.TempDir()}, Codepage: 437, Cols: 255, Rows: 255}); err == nil {
		t.Error("255x255 (65025 cells) accepted")
	}
	if _, err := New(Config{Drives: map[byte]string{'C': t.TempDir()}, Codepage: 437, Cols: 79, Rows: 25}); err == nil {
		t.Error("79 columns accepted")
	}
}
