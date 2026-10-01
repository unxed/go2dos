package dos

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

func testFS(t *testing.T, page int, names ...string) (*fsys, string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := mem.New()
	c, _ := cp.Get(page)
	e := hle.New(cpu.New(m, nil), m, c, time.Now, hle.NewTracer(4, nil, nil))
	f, err := newFS(e, Config{Drives: map[byte]string{'C': dir}})
	if err != nil {
		t.Fatal(err)
	}
	return f, dir
}

func TestShortNames(t *testing.T) {
	f, dir := testFS(t, 866, "readme.txt", "a b.txt", "очень длинное имя.txt", "доклад.doc", ".hidden", "Makefile")
	ix, errc := f.index(dir)
	if errc != 0 {
		t.Fatal(errc)
	}
	want := map[string]string{
		"README.TXT":   "readme.txt",
		"AB~1.TXT":     "a b.txt",
		"ДОКЛАД.DOC":   "доклад.doc",
		"HIDDEN~1":     ".hidden",
		"MAKEFILE":     "Makefile",
		"ОЧЕНЬД~1.TXT": "очень длинное имя.txt",
	}
	for dosName, host := range want {
		b, _ := f.e.CP.Encode(dosName)
		i, ok := ix.byDOS[string(b)]
		if !ok {
			t.Errorf("no DOS name %s (have %v)", dosName, names(f, ix))
			continue
		}
		if ix.entries[i].host != host {
			t.Errorf("%s -> %s, want %s", dosName, ix.entries[i].host, host)
		}
	}
}

func names(f *fsys, ix *dirIndex) []string {
	var out []string
	for _, e := range ix.entries {
		out = append(out, f.e.CP.Decode([]byte(e.dos)))
	}
	return out
}

func TestCaseCollision(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs a case-sensitive file system")
	}
	f, dir := testFS(t, 437, "FILE.TXT", "file.txt")
	ix, _ := f.index(dir)
	if len(ix.entries) != 2 {
		t.Fatalf("entries: %v", names(f, ix))
	}
	if _, ok := ix.byDOS["FILE.TXT"]; !ok {
		t.Error("FILE.TXT missing")
	}
	if _, ok := ix.byDOS["FILE~1.TXT"]; !ok {
		t.Errorf("alias missing: %v", names(f, ix))
	}
}

func TestCanon(t *testing.T) {
	f, _ := testFS(t, 437)
	f.cwd[2] = `\DIR`
	for in, want := range map[string]string{
		`foo.txt`:           `\DIR\FOO.TXT`,
		`..\x`:              `\X`,
		`C:\a\.\b\..\c`:     `\A\C`,
		`/unix/style`:       `\UNIX\STYLE`,
		`longfilename.text`: `\DIR\LONGFILE.TEX`,
	} {
		_, got, errc := f.canon([]byte(in), false)
		if errc != 0 || got != want {
			t.Errorf("canon(%q) = %q, %d; want %q", in, got, errc, want)
		}
	}
	if _, _, errc := f.canon([]byte(`Q:\x`), false); errc != errBadDrive {
		t.Errorf("unmapped drive: errc %d", errc)
	}
}
