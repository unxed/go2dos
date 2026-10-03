package machine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unxed/go2dos/dos"
)

func runUTF8Clip(t *testing.T, clip Clipboard) *Machine {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "utf8clip.com"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UCLIP.COM"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437, Clipboard: clip})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\UCLIP.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m.Run(ctx)
	return m
}

// In UTF-8 mode the text of WinOldAp (formats 1 and 7) is UTF-8: the size is in
// UTF-8 bytes, 1705h gives UTF-8 with CR LF, 1703h takes UTF-8; after the mode
// is turned off the same calls speak OEM again (docs/UTF8CLIPBOARD.md).
func TestUTF8Clipboard(t *testing.T) {
	clip := &dos.MemClipboard{Text: "дом\n世界"}
	m := runUTF8Clip(t, clip)
	want := []string{
		"SET=FF PREV=0000",
		"SIZE=0000000F", // 6 + 2 + 6 bytes and the final 0
		"TEXT=D0B4D0BED0BC0D0AE4B896E7958C",
		"SETC=0001",
		"SIZE2=00000015", // "Привет" CR LF "мир": 12 + 2 + 6 bytes and the final 0
		"MODE=FDE9",
		"OEM=0000000C", // in code page 437 every character that it lacks is one "?"
	}
	for i, w := range want {
		if got := m.Screen().Line(i); got != w {
			t.Fatalf("line %d = %q, want %q\nscreen:\n%s", i, got, w, m.Screen().Text())
		}
	}
	if clip.Text != "Привет\nмир" {
		t.Errorf("clipboard after the client: %q", clip.Text)
	}
}

// Without a clipboard there is no provider (as there is no WinOldAp).
func TestUTF8ClipboardAbsent(t *testing.T) {
	m := runUTF8Clip(t, nil)
	if got := m.Screen().Line(0); got != "NOT FOUND" {
		t.Errorf("line 0 = %q", got)
	}
}
