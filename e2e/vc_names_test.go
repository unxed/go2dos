package e2e

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/dos"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
)

// vcRun copies VC into a fresh drive C:, lets setup add files, runs the key
// script and returns the machine, its result and the host directory of C:.
func vcRun(t *testing.T, version string, page int, script string, setup func(dir string)) (*machine.Machine, error, string) {
	t.Helper()
	src := vcDir(t, version)
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644)
	}
	setup(dir)
	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: page})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\VC.COM`, ""); err != nil {
		t.Fatal(err)
	}
	steps, err := keys.Parse(script, m.CP)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	scriptErr := make(chan error, 1)
	go func() { scriptErr <- m.RunScript(ctx, steps, machine.ScriptOptions{}) }()
	runErr := m.Run(ctx)
	if err := <-scriptErr; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("script: %v\nmachine: %v", err, runErr)
	}
	return m, runErr, dir
}

// File names with characters that the code page lacks (docs/NAMES.md): VC sees
// aliases, and every operation has to act on the host names with both
// alphabets intact.
type namesCase struct {
	version string
	page    int
	a, b    string // two file names that look alike in the code page (content "AAA", "BBB")
	dirName string // a directory name
	inDir   string // a file name inside it
}

var namesCases = []namesCase{
	{"4.05", 437, "Привет_äöü.txt", "Привіт_äöü.txt", "Папка_äöü", "Файл_äöü.txt"},
	{"4.05", 866, "Привет_äöü.txt", "Привет_öäü.txt", "Папка_äöü", "Файл_äöü.txt"},
	{"4.99.09", 437, "Привет_äöü.txt", "Привіт_äöü.txt", "Папка_äöü", "Файл_äöü.txt"},
	{"4.99.09", 866, "Привет_äöü.txt", "Привет_öäü.txt", "Папка_äöü", "Файл_äöü.txt"},
}

// asciiLower lower-cases ASCII letters only, as VC shows names.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// shown is how VC 4.05/4.99.09 shows an 8.3 name in a panel: base padded to
// eight characters, a blank and the extension, letters in lower case. The text
// is in Unicode (what Screen.Text returns) and keys.Parse types it back.
func (c namesCase) shown(page *cp.Codepage, short string) string {
	base, ext := short, ""
	if i := strings.IndexByte(short, '.'); i >= 0 {
		base, ext = short[:i], short[i+1:]
	}
	return page.Decode([]byte(asciiLower(base) + strings.Repeat(" ", 8-len(base)) + " " + asciiLower(ext)))
}

// shownDir is the same for a directory: VC shows directory names as they are.
func (c namesCase) shownDir(page *cp.Codepage, short string) string {
	return page.Decode([]byte(short + strings.Repeat(" ", 8-len(short)) + " "))
}

func hostEntries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range list {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func readHost(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%v", err)
	}
	return string(b)
}

func expectNames(t *testing.T, where string, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("%s: host names %q, want %q", where, got, want)
	}
}

// namesRun runs a VC session on the case: DATA holds the two files (or, with
// withDir, a directory with a file), DEST is empty. body is the key script
// after VC has entered DATA and put the cursor on the first entry below "..";
// the session ends with the quit sequence. It returns the host directory of C:.
func namesRun(t *testing.T, c namesCase, withDir bool, body func(page *cp.Codepage) string) string {
	t.Helper()
	page, err := cp.Get(c.page)
	if err != nil {
		t.Fatal(err)
	}
	script := `<waitfor:10Quit><Enter><waitfor:C:\DATA><Down>` + body(page) + `<F10><waitfor:Do you want to quit><Enter>`
	m, runErr, dir := vcRun(t, c.version, c.page, script, func(dir string) {
		data := filepath.Join(dir, "DATA")
		os.Mkdir(data, 0o755)
		os.Mkdir(filepath.Join(dir, "DEST"), 0o755)
		if withDir {
			os.Mkdir(filepath.Join(data, c.dirName), 0o755)
			if err := os.WriteFile(filepath.Join(data, c.dirName, c.inDir), []byte("IN"), 0o644); err != nil {
				t.Skipf("the host cannot create the test names: %v", err)
			}
			return
		}
		if err := os.WriteFile(filepath.Join(data, c.a), []byte("AAA"), 0o644); err != nil {
			t.Skipf("the host cannot create the test names: %v", err)
		}
		os.WriteFile(filepath.Join(data, c.b), []byte("BBB"), 0o644)
	})
	var ex *machine.ExitError
	if !errors.As(runErr, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", runErr, m.Screen().Text())
	}
	return dir
}

// first returns the file that VC lists first (sorted by alias) and the other.
func (c namesCase) order(page *cp.Codepage) (first, second string) {
	sa, _ := dos.AliasFor(page, c.a)
	sb, _ := dos.AliasFor(page, c.b)
	if strings.ToUpper(sb) < strings.ToUpper(sa) {
		return c.b, c.a
	}
	return c.a, c.b
}

func contentOf(c namesCase, name string) string {
	if name == c.a {
		return "AAA"
	}
	return "BBB"
}

func (c namesCase) alias(page *cp.Codepage, host string) string {
	s, _ := dos.AliasFor(page, host)
	return s
}

// typed is an 8.3 name as the script types it (Unicode of the code page).
func typed(page *cp.Codepage, short string) string { return page.Decode([]byte(short)) }

func forCases(t *testing.T, f func(t *testing.T, c namesCase)) {
	for _, c := range namesCases {
		c := c
		t.Run(c.version+"/cp"+strconv.Itoa(c.page), func(t *testing.T) { f(t, c) })
	}
}

// The panel lists both look-alike names under different aliases.
func TestVCNamesListed(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		var want []string
		dir := namesRun(t, c, false, func(page *cp.Codepage) string {
			f, s := c.order(page)
			want = []string{c.shown(page, c.alias(page, f)), c.shown(page, c.alias(page, s))}
			if want[0] == want[1] {
				t.Fatalf("both names are shown as %q", want[0])
			}
			return `<waitfor:` + want[0] + `><waitfor:` + want[1] + `>`
		})
		expectNames(t, "after listing", hostEntries(t, filepath.Join(dir, "DATA")), c.a, c.b)
	})
}

// Rename (F6, one name): only the extension of the first file changes; the base
// keeps both alphabets.
func TestVCNamesRenameExtension(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		var first, second, renamed string
		dir := namesRun(t, c, false, func(page *cp.Codepage) string {
			first, second = c.order(page)
			renamed = strings.TrimSuffix(first, ".txt") + ".doc"
			alias := c.alias(page, first)
			base := strings.TrimSuffix(alias, ".TXT")
			// VC reads the directory again and shows the alias of the new host name.
			return `<waitfor:` + c.shown(page, alias) + `><F6><waitfor:Rename or move>` +
				typed(page, base) + `.doc<Enter><waitfor:` + c.shown(page, c.alias(page, renamed)) + `>`
		})
		data := filepath.Join(dir, "DATA")
		expectNames(t, "after the rename", hostEntries(t, data), renamed, second)
		if got := readHost(t, filepath.Join(data, renamed)); got != contentOf(c, first) {
			t.Errorf("content of %q: %q", renamed, got)
		}
		if got := readHost(t, filepath.Join(data, second)); got != contentOf(c, second) {
			t.Errorf("content of %q: %q", second, got)
		}
	})
}

// Rename by mask (Shift-F6, *.txt to *.doc): both files change their extension
// (the dialog of 4.05 and 4.99.09 has a field for the files and one for the target).
func TestVCNamesRenameMask(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		dir := namesRun(t, c, false, func(page *cp.Codepage) string {
			return `<waitfor:` + c.shown(page, c.alias(page, c.a)) + `><Shift-F6><waitfor:Rename or move>*.txt<Tab>*.doc<Enter><waitfor:10Quit>` +
				// 4.99.09 does not show the new names by itself: re-read the panel.
				`<Ctrl-R><waitfor:` + c.shown(page, c.alias(page, strings.TrimSuffix(c.a, ".txt")+".doc")) + `>` +
				`<waitfor:` + c.shown(page, c.alias(page, strings.TrimSuffix(c.b, ".txt")+".doc")) + `>`
		})
		data := filepath.Join(dir, "DATA")
		ra, rb := strings.TrimSuffix(c.a, ".txt")+".doc", strings.TrimSuffix(c.b, ".txt")+".doc"
		expectNames(t, "after the rename", hostEntries(t, data), ra, rb)
		if got := readHost(t, filepath.Join(data, ra)); got != "AAA" {
			t.Errorf("content of %q: %q", ra, got)
		}
		if got := readHost(t, filepath.Join(data, rb)); got != "BBB" {
			t.Errorf("content of %q: %q", rb, got)
		}
	})
}

// Copy (F5) both files into DEST one after the other: the names there are the
// host names, the contents are not mixed up, the sources stay.
func TestVCNamesCopy(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		dir := namesRun(t, c, false, func(page *cp.Codepage) string {
			f, s := c.order(page)
			return `<waitfor:` + c.shown(page, c.alias(page, f)) + `><F5><waitfor:Copy ">C:\DEST<Enter><waitfor:10Quit>` +
				`<Down><waitfor:` + c.shown(page, c.alias(page, s)) + `><F5><waitfor:Copy ">C:\DEST<Enter><waitfor:10Quit>`
		})
		expectNames(t, "DATA", hostEntries(t, filepath.Join(dir, "DATA")), c.a, c.b)
		dest := filepath.Join(dir, "DEST")
		expectNames(t, "DEST", hostEntries(t, dest), c.a, c.b)
		for _, n := range []string{c.a, c.b} {
			if got := readHost(t, filepath.Join(dest, n)); got != contentOf(c, n) {
				t.Errorf("content of DEST/%q: %q", n, got)
			}
		}
	})
}

// Move (F6) the first file into DEST.
func TestVCNamesMove(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		var first, second string
		dir := namesRun(t, c, false, func(page *cp.Codepage) string {
			first, second = c.order(page)
			return `<waitfor:` + c.shown(page, c.alias(page, first)) + `><F6><waitfor:Rename or move>C:\DEST<Enter><waitfor:10Quit>`
		})
		expectNames(t, "DATA", hostEntries(t, filepath.Join(dir, "DATA")), second)
		dest := filepath.Join(dir, "DEST")
		expectNames(t, "DEST", hostEntries(t, dest), first)
		if got := readHost(t, filepath.Join(dest, first)); got != contentOf(c, first) {
			t.Errorf("content of DEST/%q: %q", first, got)
		}
	})
}

// Delete (F8) the first file: the other one stays.
func TestVCNamesDelete(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		var second string
		dir := namesRun(t, c, false, func(page *cp.Codepage) string {
			var first string
			first, second = c.order(page)
			return `<waitfor:` + c.shown(page, c.alias(page, first)) + `><F8><waitfor:Do you wish to delete><Enter><waitfor:` +
				page.Decode([]byte(asciiLower(c.alias(page, second)))) + `>`
		})
		expectNames(t, "DATA", hostEntries(t, filepath.Join(dir, "DATA")), second)
	})
}

// A directory with such a name: enter it, copy the file inside to DEST; then
// copy and move the directory itself.
func TestVCNamesDirectory(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		dir := namesRun(t, c, true, func(page *cp.Codepage) string {
			return `<waitfor:` + c.shownDir(page, c.alias(page, c.dirName)) + `><Enter><waitfor:` + c.shown(page, c.alias(page, c.inDir)) + `>` +
				`<Down><F5><waitfor:Copy ">C:\DEST<Enter><waitfor:10Quit>`
		})
		expectNames(t, "DEST", hostEntries(t, filepath.Join(dir, "DEST")), c.inDir)
		if got := readHost(t, filepath.Join(dir, "DEST", c.inDir)); got != "IN" {
			t.Errorf("content: %q", got)
		}
		expectNames(t, "DATA", hostEntries(t, filepath.Join(dir, "DATA")), c.dirName)
	})
}

func TestVCNamesDirectoryCopy(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		dir := namesRun(t, c, true, func(page *cp.Codepage) string {
			return `<waitfor:` + c.shownDir(page, c.alias(page, c.dirName)) + `><F5><waitfor:Copy ">C:\DEST<Enter><waitfor:10Quit>`
		})
		expectNames(t, "DEST", hostEntries(t, filepath.Join(dir, "DEST")), c.dirName)
		expectNames(t, "DEST/dir", hostEntries(t, filepath.Join(dir, "DEST", c.dirName)), c.inDir)
		if got := readHost(t, filepath.Join(dir, "DEST", c.dirName, c.inDir)); got != "IN" {
			t.Errorf("content: %q", got)
		}
		expectNames(t, "DATA", hostEntries(t, filepath.Join(dir, "DATA")), c.dirName)
	})
}

func TestVCNamesDirectoryMove(t *testing.T) {
	forCases(t, func(t *testing.T, c namesCase) {
		dir := namesRun(t, c, true, func(page *cp.Codepage) string {
			return `<waitfor:` + c.shownDir(page, c.alias(page, c.dirName)) + `><F6><waitfor:Rename or move>C:\DEST<Enter><waitfor:10Quit>`
		})
		expectNames(t, "DEST", hostEntries(t, filepath.Join(dir, "DEST")), c.dirName)
		expectNames(t, "DEST/dir", hostEntries(t, filepath.Join(dir, "DEST", c.dirName)), c.inDir)
		expectNames(t, "DATA", hostEntries(t, filepath.Join(dir, "DATA")))
	})
}

// VC 4.99.09 in long-name mode (Ctrl-N): the long forms are what it shows and
// passes to INT 21h/71xx; rename and copy lead back to the host names too.
func TestVCNamesLongMode(t *testing.T) {
	c := namesCases[2]
	var first, second string
	dir := namesRun(t, c, false, func(page *cp.Codepage) string {
		first, second = c.order(page)
		alias := c.alias(page, first)
		_, long := dos.AliasFor(page, first)
		_, longSecond := dos.AliasFor(page, second)
		// In long-name mode the panel cuts a name to 12 characters; the status
		// line shows a "←" and the last 11.
		status := func(l string) string { return "←" + page.Decode([]byte(l[len(l)-11:])) }
		return `<waitfor:` + c.shown(page, alias) + `><Ctrl-N><waitfor:` + status(long) + `>` +
			`<Down><waitfor:` + status(longSecond) + `><F5><waitfor:Copy ">C:\DEST<Enter><waitfor:10Quit>` +
			`<Up><waitfor:` + status(long) + `><F6><waitfor:Rename or move>` +
			typed(page, strings.TrimSuffix(long, ".txt")) + `.doc<Enter><waitfor:10Quit>`
	})
	renamed := strings.TrimSuffix(first, ".txt") + ".doc"
	expectNames(t, "DATA", hostEntries(t, filepath.Join(dir, "DATA")), renamed, second)
	expectNames(t, "DEST", hostEntries(t, filepath.Join(dir, "DEST")), second)
	if got := readHost(t, filepath.Join(dir, "DATA", renamed)); got != contentOf(c, first) {
		t.Errorf("content of %q: %q", renamed, got)
	}
	if got := readHost(t, filepath.Join(dir, "DEST", second)); got != contentOf(c, second) {
		t.Errorf("content of DEST/%q: %q", second, got)
	}
}
