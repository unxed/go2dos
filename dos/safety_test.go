package dos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unxed/go2dos/cpu"
)

// Data-safety probes of the host file system pass-through (docs/DATA-SAFETY.md).

// The 8.3 name of a long name must not change when other files appear: a program (or a
// panel of a file manager) that holds ALONGN~1.TXT would otherwise delete another file.
func TestShortNameStableWhenFilesAppear(t *testing.T) {
	f, dir := testFS(t, 437, "long name a.txt", "long name b.txt")
	before := map[string]string{}
	ix, _ := f.index(dir)
	for _, e := range ix.entries {
		before[e.host] = e.dos
	}
	for _, n := range []string{"long name 0.txt", "long name aa.txt", "LONG NAME A2.TXT"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	f.invalidate()
	ix, _ = f.index(dir)
	for _, e := range ix.entries {
		if old, ok := before[e.host]; ok && old != e.dos {
			t.Errorf("%q: short name %s became %s", e.host, old, e.dos)
		}
	}
}

// Paths with ".." never leave the root of the drive.
func TestDotDotStaysInRoot(t *testing.T) {
	f, dir := testFS(t, 437, "in.txt")
	outside := filepath.Join(filepath.Dir(dir), "outside.txt")
	os.WriteFile(outside, []byte("x"), 0o644)
	for _, p := range []string{`..\outside.txt`, `\..\..\outside.txt`, `SUB\..\..\outside.txt`, `C:..\outside.txt`} {
		drive, dp, errc := f.canon([]byte(p), false)
		if errc != 0 {
			continue
		}
		host, _, errc := f.resolve(drive, dp, false)
		if errc == 0 && !strings.HasPrefix(host, dir) {
			t.Errorf("%q resolved outside the root: %s", p, host)
		}
		if errc == 0 && host == outside {
			t.Errorf("%q reached %s", p, host)
		}
	}
	// the long-name resolver too
	for _, p := range []string{`..\outside.txt`, `\..\..\outside.txt`} {
		r, errc := f.lfnResolve([]byte(p))
		if errc == 0 && !strings.HasPrefix(r.host, dir) {
			t.Errorf("lfnResolve(%q) outside the root: %s", p, r.host)
		}
	}
}

func TestParseReadOnly(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
	}{{"C", "C"}, {"cd", "CD"}, {"C,D", "CD"}, {"c: d:", "CD"}} {
		m, err := ParseReadOnly(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		var got string
		for l := byte('A'); l <= 'Z'; l++ {
			if m[l] {
				got += string(l)
			}
		}
		if got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
	if m, _ := ParseReadOnly("all"); len(m) != 26 {
		t.Errorf("all: %d drives", len(m))
	}
	if _, err := ParseReadOnly("C1"); err == nil {
		t.Errorf("C1 accepted")
	}
}

func TestWriteProtectedHostPaths(t *testing.T) {
	f, dir := testFS(t, 437)
	if f.wp(filepath.Join(dir, "x")) {
		t.Fatal("drive is writable by default")
	}
	f.ro['C'-'A'] = true
	for _, p := range []string{dir, filepath.Join(dir, "x"), filepath.Join(dir, "a", "b")} {
		if !f.wp(p) {
			t.Errorf("%s not protected", p)
		}
	}
	if f.wp(dir + "-sibling") {
		t.Errorf("a sibling directory with the same prefix is protected")
	}
}

// -confine: links that lead out of the drive disappear; links inside stay.
func TestConfineHidesEscapingLinks(t *testing.T) {
	f, dir := testFS(t, 437, "real.txt")
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("s"), 0o644)
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	for name, target := range map[string]string{
		"out-file": filepath.Join(outside, "secret.txt"),
		"out-dir":  outside,
		"in-file":  filepath.Join(dir, "real.txt"),
		"in-dir":   filepath.Join(dir, "sub"),
		"broken":   filepath.Join(dir, "nothing"),
	} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skipf("no symbolic links: %v", err)
		}
	}
	names := func() map[string]bool {
		f.invalidate()
		ix, _ := f.index(dir)
		m := map[string]bool{}
		for _, e := range ix.entries {
			m[e.host] = true
		}
		return m
	}
	all := names()
	for _, n := range []string{"out-file", "out-dir", "in-file", "in-dir", "broken"} {
		if !all[n] {
			t.Errorf("without -confine %s is missing", n)
		}
	}
	f.confine = true
	got := names()
	for _, n := range []string{"out-file", "out-dir", "broken"} {
		if got[n] {
			t.Errorf("-confine: %s is visible", n)
		}
	}
	for _, n := range []string{"real.txt", "sub", "in-file", "in-dir"} {
		if !got[n] {
			t.Errorf("-confine: %s is hidden", n)
		}
	}
}

// -trash: a deleted file is moved to the trash directory, a taken name gets ".~N", the move
// is logged; a file inside the trash is really removed; if the trash cannot be made, the
// file stays.
func TestTrash(t *testing.T) {
	trash := filepath.Join(t.TempDir(), "bin")
	h := newHarness(t, 437, Config{TrashDir: trash}, "A.TXT", "B.TXT")
	del := func(name string) uint16 { return h.d.unlink([]byte(`C:\` + name)) }
	if errc := del("A.TXT"); errc != 0 {
		t.Fatal("delete A.TXT:", errc)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "A.TXT")); err == nil {
		t.Error("A.TXT is still in the drive")
	}
	if b, err := os.ReadFile(filepath.Join(trash, "A.TXT")); err != nil || string(b) != "x" {
		t.Errorf("trash/A.TXT: %q %v", b, err)
	}
	// The same name again: nothing in the trash is overwritten.
	os.WriteFile(filepath.Join(h.dir, "A.TXT"), []byte("second"), 0o644)
	h.d.fs.invalidate()
	if errc := del("A.TXT"); errc != 0 {
		t.Fatal("delete A.TXT again:", errc)
	}
	if b, _ := os.ReadFile(filepath.Join(trash, "A.TXT.~1")); string(b) != "second" {
		t.Errorf("trash/A.TXT.~1: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(trash, "A.TXT")); string(b) != "x" {
		t.Errorf("the first file in the trash was overwritten: %q", b)
	}
	lines := strings.Split(strings.TrimSpace(readFile(t, filepath.Join(trash, trashLog))), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "\t"+filepath.Join(h.dir, "A.TXT")+"\tA.TXT") ||
		!strings.HasSuffix(lines[1], "\tA.TXT.~1") {
		t.Errorf("log:\n%s", strings.Join(lines, "\n"))
	}
	// A file that lies in the trash is removed for real.
	if err := h.d.fs.remove(filepath.Join(trash, "A.TXT.~1")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(trash, "A.TXT.~1")); err == nil {
		t.Error("a file in the trash was not removed")
	}
}

func TestTrashUnavailableKeepsFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	os.WriteFile(blocker, nil, 0o644)
	h := newHarness(t, 437, Config{TrashDir: filepath.Join(blocker, "bin")}, "A.TXT") // a file where a directory is needed
	if errc := h.d.unlink([]byte(`C:\A.TXT`)); errc == 0 {
		t.Error("the delete succeeded without a trash")
	}
	if _, err := os.Stat(filepath.Join(h.dir, "A.TXT")); err != nil {
		t.Error("the file is gone though it could not be trashed:", err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The LFN delete (7141h), with and without wildcards, goes to the trash too.
func TestTrashLFNDelete(t *testing.T) {
	trash := filepath.Join(t.TempDir(), "bin")
	h := newHarness(t, 437, Config{TrashDir: trash}, "One Long Name.txt", "Two Long Name.txt", "keep.dat")
	h.put(nameOff, `C:\One Long Name.txt`)
	if h.call6(0x7141, 0, 0, nameOff, 0, 0) {
		t.Fatalf("7141h failed: AX=%04X", h.reg(cpu.AX))
	}
	h.put(nameOff, `C:\*.txt`)
	if h.call6(0x7141, 0, 0x0027, nameOff, 1, 0) {
		t.Fatalf("7141h with wildcards failed: AX=%04X", h.reg(cpu.AX))
	}
	for _, n := range []string{"One Long Name.txt", "Two Long Name.txt"} {
		if _, err := os.Stat(filepath.Join(trash, n)); err != nil {
			t.Errorf("%s is not in the trash: %v", n, err)
		}
		if _, err := os.Stat(filepath.Join(h.dir, n)); err == nil {
			t.Errorf("%s is still in the drive", n)
		}
	}
	if _, err := os.Stat(filepath.Join(h.dir, "keep.dat")); err != nil {
		t.Error("keep.dat is gone")
	}
}
