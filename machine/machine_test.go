package machine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runProg(t *testing.T, name string) (*Machine, error) {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\`+strings.ToUpper(name), ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m, m.Run(ctx)
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	var ex *ExitError
	if !errors.As(err, &ex) {
		t.Fatalf("want a normal exit, got %v", err)
	}
	return ex.Code
}

func TestHello(t *testing.T) {
	m, err := runProg(t, "hello.com")
	if c := exitCode(t, err); c != 7 {
		t.Errorf("exit code %d, want 7", c)
	}
	if got := m.Screen().Line(0); got != "Hello from go2dos" {
		t.Errorf("screen line 0 = %q", got)
	}
}

func TestFiles(t *testing.T) {
	m, err := runProg(t, "files.com")
	if c := exitCode(t, err); c != 0 {
		t.Fatalf("exit code %d; screen:\n%s", c, m.Screen().Text())
	}
	if got := m.Screen().Line(0); got != "TEST.TXT: file I/O works" {
		t.Errorf("screen line 0 = %q", got)
	}
}
