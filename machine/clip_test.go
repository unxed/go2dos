package machine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unxed/go2dos/dos"
)

func runClip(t *testing.T, clip Clipboard) *Machine {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "clip.com"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CLIP.COM"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437, Clipboard: clip})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\CLIP.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d; screen:\n%s", c, m.Screen().Text())
	}
	return m
}

// The client reads the host text (OEM, LF -> CR LF, NUL-terminated: 7 bytes
// with the NUL), asks for a format that is not there, and replaces the text.
func TestClipboardServer(t *testing.T) {
	clip := &dos.MemClipboard{Text: "é\nabc"}
	m := runClip(t, clip)
	want := []string{"VER=0A03", "SIZE=00000007", "é", "abc", "BM=00000000", "SET=0001"}
	for i, w := range want {
		if got := m.Screen().Line(i); got != w {
			t.Fatalf("line %d = %q, want %q\nscreen:\n%s", i, got, w, m.Screen().Text())
		}
	}
	if clip.Text != "dos é\nsecond" {
		t.Errorf("clipboard after the client: %q", clip.Text)
	}
}

// Without a clipboard the API is "not installed": 1700h comes back unchanged.
func TestClipboardAbsent(t *testing.T) {
	m := runClip(t, nil)
	if got := m.Screen().Line(0); got != "NO CLIP" {
		t.Errorf("line 0 = %q", got)
	}
}

// flipClip changes its text after the first read: the system clipboard being
// changed between the size (1704h) and the data (1705h) calls.
type flipClip struct {
	reads int
	set   string
}

func (c *flipClip) GetText() (string, error) {
	c.reads++
	if c.reads == 1 {
		return "é\nabc", nil
	}
	return "a much longer text than the client has room for", nil
}
func (c *flipClip) SetText(s string) error { c.set = s; return nil }

// The data the client gets is the text whose size 1704h reported (T15b).
func TestClipboardSnapshotBetweenSizeAndData(t *testing.T) {
	clip := &flipClip{}
	m := runClip(t, clip)
	want := []string{"VER=0A03", "SIZE=00000007", "é", "abc"}
	for i, w := range want {
		if got := m.Screen().Line(i); got != w {
			t.Fatalf("line %d = %q, want %q\nscreen:\n%s", i, got, w, m.Screen().Text())
		}
	}
}
