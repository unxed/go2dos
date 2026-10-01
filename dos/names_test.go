package dos

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/mem"
)

// classicList lists a directory with FindFirst/FindNext (INT 21h AH=4Eh/4Fh),
// files and directories, and returns the names in DOS bytes.
func classicList(h *harness, pattern string) []string {
	h.t.Helper()
	h.put(nameOff, pattern)
	var out []string
	cf := h.call(0x4E00, 0, 0x10, nameOff)
	for !cf {
		a := mem.Lin(tDTA, 0)
		n := string(h.e.Mem.ASCIIZ(a+0x1E, 13))
		if n != "." && n != ".." {
			out = append(out, n)
		}
		cf = h.call(0x4F00, 0, 0, 0)
	}
	sort.Strings(out)
	return out
}

// hostNames lists the names in a host directory.
func hostNames(t *testing.T, dir string) []string {
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

func sameNames(a, b []string) bool { return strings.Join(a, "\x00") == strings.Join(b, "\x00") }

func skipIfNoUnicodeNames(t *testing.T, dir string) {
	t.Helper()
	probe := filepath.Join(dir, "Проба_äöü")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Skipf("the host file system cannot hold the test names: %v", err)
	}
	os.Remove(probe)
}

// Names that CP437 cannot hold (Cyrillic) but with characters it can (umlauts):
// the program sees unique aliases, and changing only the extension of an alias,
// copying it into another directory, deleting it and entering a directory with
// such a name all act on the right host names, with both alphabets intact.
func TestLossyNamesClassic(t *testing.T) {
	const a, b, dirName = "Привет_äöü.txt", "Привіт_äöü.txt", "Папка_äöü"
	h := newHarness(t, 437, Config{})
	skipIfNoUnicodeNames(t, h.dir)
	os.WriteFile(filepath.Join(h.dir, a), []byte("AAA"), 0o644)
	os.WriteFile(filepath.Join(h.dir, b), []byte("BBB"), 0o644)
	os.WriteFile(filepath.Join(h.dir, "plain.txt"), []byte("P"), 0o644)
	os.Mkdir(filepath.Join(h.dir, dirName), 0o755)
	page := h.d.fs.e.CP
	aliasA, _ := AliasFor(page, a)
	aliasB, _ := AliasFor(page, b)
	aliasDir, _ := AliasFor(page, dirName)

	// 1. The listing: three files and a directory; the two look-alike names differ.
	want := []string{"PLAIN.TXT", aliasA, aliasB, aliasDir}
	sort.Strings(want)
	got := classicList(h, `C:\*.*`)
	if !sameNames(got, want) {
		t.Fatalf("listing %q, want %q", got, want)
	}
	if aliasA == aliasB {
		t.Fatalf("different names share the alias %q", aliasA)
	}

	// 2. Only the extension changes: the original base stays (both alphabets).
	base := strings.TrimSuffix(aliasA, ".TXT")
	h.put(nameOff, `C:\`+aliasA)
	h.put(nameOf2, `C:\`+base+".DOC")
	if h.call6(0x5600, 0, 0, nameOff, 0, nameOf2) {
		t.Fatalf("rename: AX=%04X", h.reg(cpu.AX))
	}
	wantHost := []string{"Привет_äöü.doc", b, dirName, "plain.txt"}
	sort.Strings(wantHost)
	if got := hostNames(t, h.dir); !sameNames(got, wantHost) {
		t.Fatalf("host names after the rename: %q, want %q", got, wantHost)
	}
	if c, _ := os.ReadFile(filepath.Join(h.dir, "Привет_äöü.doc")); string(c) != "AAA" {
		t.Errorf("content after the rename: %q", c)
	}

	// 3. Copy into another directory: the program creates the alias there.
	h.put(nameOff, `C:\DEST`)
	if h.call(0x3900, 0, 0, nameOff) {
		t.Fatalf("mkdir: AX=%04X", h.reg(cpu.AX))
	}
	h.put(nameOff, `C:\DEST\`+aliasB)
	if h.call(0x3C00, 0, 0, nameOff) {
		t.Fatalf("create: AX=%04X", h.reg(cpu.AX))
	}
	fh := h.reg(cpu.AX)
	h.put(bufOff, "BBB")
	if h.call(0x4000, fh, 3, bufOff) {
		t.Fatalf("write: AX=%04X", h.reg(cpu.AX))
	}
	h.call(0x3E00, fh, 0, 0)
	if got := hostNames(t, filepath.Join(h.dir, "DEST")); !sameNames(got, []string{b}) {
		t.Fatalf("host names in DEST: %q, want %q", got, []string{b})
	}
	// Listing DEST gives the same alias again.
	if got := classicList(h, `C:\DEST\*.*`); !sameNames(got, []string{aliasB}) {
		t.Errorf("DEST listing %q, want %q", got, []string{aliasB})
	}

	// 4. A directory with such a name: enter it by the alias, create a file in it.
	h.put(nameOff, `C:\`+aliasDir)
	if h.call(0x3B00, 0, 0, nameOff) {
		t.Fatalf("chdir: AX=%04X", h.reg(cpu.AX))
	}
	h.put(nameOff, `new.txt`)
	if h.call(0x3C00, 0, 0, nameOff) {
		t.Fatalf("create in the directory: AX=%04X", h.reg(cpu.AX))
	}
	h.call(0x3E00, h.reg(cpu.AX), 0, 0)
	if got := hostNames(t, filepath.Join(h.dir, dirName)); !sameNames(got, []string{"NEW.TXT"}) {
		t.Errorf("host names in the directory: %q", got)
	}
	h.put(nameOff, `C:\`)
	h.call(0x3B00, 0, 0, nameOff)

	// 5. Delete by the alias.
	h.put(nameOff, `C:\`+aliasB)
	if h.call(0x4100, 0, 0, nameOff) {
		t.Fatalf("delete: AX=%04X", h.reg(cpu.AX))
	}
	wantHost = []string{"Привет_äöü.doc", dirName, "DEST", "plain.txt"}
	sort.Strings(wantHost)
	if got := hostNames(t, h.dir); !sameNames(got, wantHost) {
		t.Errorf("host names after the delete: %q, want %q", got, wantHost)
	}

	// 6. A rename onto an existing host name does not overwrite it.
	os.WriteFile(filepath.Join(h.dir, "Привет_äöü.txt"), []byte("NEW"), 0o644)
	h.d.fs.invalidate()
	aliasA2, _ := AliasFor(page, "Привет_äöü.txt")
	h.put(nameOff, `C:\`+aliasA2)
	h.put(nameOf2, `C:\`+base+".DOC") // the alias of Привет_äöü.doc
	if !h.call6(0x5600, 0, 0, nameOff, 0, nameOf2) {
		t.Error("the rename onto an existing file succeeded")
	}
	if c, _ := os.ReadFile(filepath.Join(h.dir, "Привет_äöü.doc")); string(c) != "AAA" {
		t.Errorf("the existing file was overwritten: %q", c)
	}
}

// The same with CP866 and umlauts, which the page lacks; and a name with an
// extension that the page lacks.
func TestLossyNamesCP866(t *testing.T) {
	h := newHarness(t, 866, Config{})
	skipIfNoUnicodeNames(t, h.dir)
	const a = "Привет_äöü.txt"
	os.WriteFile(filepath.Join(h.dir, a), []byte("X"), 0o644)
	os.WriteFile(filepath.Join(h.dir, "äöü.txt"), []byte("Y"), 0o644)
	os.WriteFile(filepath.Join(h.dir, "üöä.txt"), []byte("Z"), 0o644)
	aliasA, _ := AliasFor(h.d.fs.e.CP, a)
	got := classicList(h, `C:\*.*`)
	if len(got) != 3 {
		t.Fatalf("listing %q", got)
	}
	seen := map[string]bool{}
	for _, n := range got {
		if seen[n] {
			t.Errorf("name %q listed twice: %q", n, got)
		}
		seen[n] = true
	}
	if !seen[aliasA] {
		t.Errorf("no alias %q for %q in %q", aliasA, a, got)
	}
	// The Cyrillic name is representable except the umlauts: renaming the
	// extension keeps the Cyrillic and the umlauts.
	base := strings.TrimSuffix(aliasA, ".TXT")
	h.put(nameOff, `C:\`+aliasA)
	h.put(nameOf2, `C:\`+base+".BAK")
	if h.call6(0x5600, 0, 0, nameOff, 0, nameOf2) {
		t.Fatalf("rename: AX=%04X", h.reg(cpu.AX))
	}
	if c, err := os.ReadFile(filepath.Join(h.dir, "Привет_äöü.bak")); err != nil || string(c) != "X" {
		t.Errorf("Привет_äöü.bak: %q, %v; host names %q", c, err, hostNames(t, h.dir))
	}
}

// A name whose extension the page lacks: the alias has no extension, and a
// copy keeps the host extension.
func TestLossyExtension(t *testing.T) {
	h := newHarness(t, 437, Config{})
	skipIfNoUnicodeNames(t, h.dir)
	const name = "doc.текст"
	os.WriteFile(filepath.Join(h.dir, name), []byte("T"), 0o644)
	alias, _ := AliasFor(h.d.fs.e.CP, name)
	if got := classicList(h, `C:\*.*`); !sameNames(got, []string{alias}) {
		t.Fatalf("listing %q, want %q", got, alias)
	}
	h.put(nameOff, `C:\COPY`)
	h.call(0x3900, 0, 0, nameOff)
	h.put(nameOff, `C:\COPY\`+alias)
	if h.call(0x3C00, 0, 0, nameOff) {
		t.Fatalf("create: AX=%04X", h.reg(cpu.AX))
	}
	h.call(0x3E00, h.reg(cpu.AX), 0, 0)
	if got := hostNames(t, filepath.Join(h.dir, "COPY")); !sameNames(got, []string{name}) {
		t.Errorf("host names in COPY: %q, want %q", got, name)
	}
}

// Long names (INT 21h/71xx): the long forms of look-alike names differ, the
// long form leads back to the host name for rename, with a changed extension
// too, and the short name of the same entry works as well.
func TestLossyNamesLong(t *testing.T) {
	const a, b = "Привет_äöü.txt", "Привіт_äöü.txt"
	h := newHarness(t, 437, Config{})
	skipIfNoUnicodeNames(t, h.dir)
	os.WriteFile(filepath.Join(h.dir, a), []byte("A"), 0o644)
	os.WriteFile(filepath.Join(h.dir, b), []byte("B"), 0o644)
	page := h.d.fs.e.CP
	shortA, longA := AliasFor(page, a)
	shortB, longB := AliasFor(page, b)
	if longA == longB {
		t.Fatalf("the long forms of different names are equal: %q", longA)
	}
	h.put(nameOff, `C:\*`)
	if h.call6(0x714E, 0, 0x0016, nameOff, 1, findOff) {
		t.Fatalf("714Eh failed: AX=%04X", h.reg(cpu.AX))
	}
	handle := h.reg(cpu.AX)
	got := map[string]string{}
	for {
		long, short, _ := h.findData()
		got[long] = short
		if h.call6(0x714F, handle, 0, 0, 1, findOff) {
			break
		}
	}
	if len(got) != 2 || got[longA] != shortA || got[longB] != shortB {
		t.Fatalf("found %q, want %q=%q and %q=%q", got, longA, shortA, longB, shortB)
	}
	// Rename by the long form, only the extension changes.
	baseLong := strings.TrimSuffix(longA, ".txt")
	h.put(nameOff, `C:\`+longA)
	h.put(nameOf2, `C:\`+baseLong+".doc")
	if h.call6(0x7156, 0, 0, nameOff, 0, nameOf2) {
		t.Fatalf("7156h by the long form: AX=%04X", h.reg(cpu.AX))
	}
	wantHost := []string{"Привет_äöü.doc", b}
	sort.Strings(wantHost)
	if got := hostNames(t, h.dir); !sameNames(got, wantHost) {
		t.Fatalf("host names: %q, want %q", got, wantHost)
	}
	// Rename by the short name, only the extension changes again.
	newShort, _ := AliasFor(page, "Привет_äöü.doc")
	h.d.fs.invalidate()
	h.put(nameOff, `C:\`+newShort)
	h.put(nameOf2, `C:\`+strings.TrimSuffix(newShort, ".DOC")+".TXT")
	if h.call6(0x7156, 0, 0, nameOff, 0, nameOf2) {
		t.Fatalf("7156h by the short name: AX=%04X", h.reg(cpu.AX))
	}
	wantHost = []string{"Привет_äöü.txt", b}
	sort.Strings(wantHost)
	if got := hostNames(t, h.dir); !sameNames(got, wantHost) {
		t.Fatalf("host names: %q, want %q", got, wantHost)
	}
	// Create the long form of the other name in a new directory (a copy).
	h.put(nameOff, `C:\DEST`)
	h.call(0x7139, 0, 0, nameOff)
	h.put(nameOff, `C:\DEST\`+longB)
	if h.call6(0x716C, 2, 0, 0x12, nameOff, 0) {
		t.Fatalf("716Ch create: AX=%04X", h.reg(cpu.AX))
	}
	h.call(0x3E00, h.reg(cpu.AX), 0, 0)
	if got := hostNames(t, filepath.Join(h.dir, "DEST")); !sameNames(got, []string{b}) {
		t.Errorf("host names in DEST: %q, want %q", got, []string{b})
	}
}

// Nothing changes for names the page can hold: plain 8.3, umlauts in the page,
// a long name with the numeric-tail alias.
func TestRepresentableNamesUnchanged(t *testing.T) {
	h := newHarness(t, 437, Config{}, "plain.txt", "äöü.txt", "a long file name.txt")
	got := classicList(h, `C:\*.*`)
	want := []string{"PLAIN.TXT", "ÄÖÜ.TXT", "ALONGF~1.TXT"}
	for i, w := range want { // the page bytes of the umlauts
		b, _ := h.d.fs.e.CP.Encode(w)
		want[i] = string(b)
	}
	sort.Strings(want)
	if !sameNames(got, want) {
		t.Errorf("listing %q, want %q", got, want)
	}
}

// Two names whose first alias candidate is the same (same stem, same hash):
// the second one gets the next candidate, both stay distinct and lead back to
// their own host names. The pair was found by searching names "Пabc<N>.txt".
func TestLossyAliasCollision(t *testing.T) {
	const a, b = "Пabc145.txt", "Пabc362.txt"
	h := newHarness(t, 437, Config{})
	skipIfNoUnicodeNames(t, h.dir)
	os.WriteFile(filepath.Join(h.dir, a), []byte("A"), 0o644)
	os.WriteFile(filepath.Join(h.dir, b), []byte("B"), 0o644)
	page := h.d.fs.e.CP
	sa, _ := AliasFor(page, a)
	sb, _ := AliasFor(page, b)
	if sa != sb {
		t.Fatalf("the test pair no longer collides: %q and %q", sa, sb)
	}
	got := classicList(h, `C:\*.*`)
	if len(got) != 2 || got[0] == got[1] {
		t.Fatalf("listing %q: the names must be distinct", got)
	}
	if got[0] != sa {
		t.Errorf("the first name has the alias %q, want %q", got[0], sa)
	}
	// The listing is stable.
	h.d.fs.invalidate()
	if again := classicList(h, `C:\*.*`); !sameNames(again, got) {
		t.Errorf("second listing %q, first %q", again, got)
	}
	// Each alias leads to its own host name.
	for i, host := range []string{a, b} {
		base := strings.TrimSuffix(got[i], ".TXT")
		h.put(nameOff, `C:\`+got[i])
		h.put(nameOf2, `C:\`+base+".DOC")
		if h.call6(0x5600, 0, 0, nameOff, 0, nameOf2) {
			t.Fatalf("rename %q: AX=%04X", got[i], h.reg(cpu.AX))
		}
		want := strings.TrimSuffix(host, ".txt") + ".doc"
		wantContent := []string{"A", "B"}[i]
		if c, err := os.ReadFile(filepath.Join(h.dir, want)); err != nil || string(c) != wantContent {
			t.Errorf("%q: %q, %v; host names %q", want, c, err, hostNames(t, h.dir))
		}
	}
}
