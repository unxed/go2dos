package machine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// In UTF-8 mode 714Eh returns the long name in UTF-8 and the short name as an
// ASCII alias; the mode is per process, and invalid UTF-8 is an error.
func TestUTF8Names(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "utf8names.com"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UTF8.COM"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "дом 世界.txt"), []byte("x"), 0o644); err != nil {
		t.Skipf("the host cannot create the file name: %v", err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\UTF8.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d; screen:\n%s", c, m.Screen().Text())
	}
	want := []string{
		"SET=FF PREV=0000",
		"NAME=D0B4D0BED0BC20E4B896E7958C2E747874", // дом 世界.txt in UTF-8
		"SHORT=_~1.TXT",
		"MODE=FDE9",
		"NAME0=5F5F5F205F5F2E747874", // "___ __.txt": unmappable characters are '_'
		"BAD=010003",                 // CF set, AX=3
	}
	for i, w := range want {
		if got := m.Screen().Line(i); got != w {
			t.Fatalf("line %d = %q, want %q\nscreen:\n%s", i, got, w, m.Screen().Text())
		}
	}
}
