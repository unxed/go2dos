package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/mem"
)

// Offsets in the test data segment: names and the FindData record.
const (
	nameOff = 0x0010
	nameOf2 = 0x0110
	findOff = 0x0400
	bufOff  = 0x0600
)

func (h *harness) reg(r int) uint16 { return h.e.CPU.R[r] }

// asciiz returns the string at DS:off.
func (h *harness) asciiz(off uint16) string {
	return string(h.e.Mem.ASCIIZ(mem.Lin(tDS, off), 300))
}

// findData reads the long and short names and the attributes from the FindData record.
func (h *harness) findData() (long, short string, attr uint32) {
	return h.asciiz(findOff + 0x2C), h.asciiz(findOff + 0x130), h.e.Mem.R32(mem.Lin(tDS, findOff))
}

func TestLFNVolumeInfo(t *testing.T) {
	h := newHarness(t, 437, Config{})
	h.put(nameOff, `C:\`)
	if h.call6(0x71A0, 0, 32, nameOff, 0, bufOff) {
		t.Fatalf("71A0h failed: AX=%04X", h.reg(cpu.AX))
	}
	if h.asciiz(bufOff) != "FAT" {
		t.Errorf("file system %q", h.asciiz(bufOff))
	}
	if bx := h.reg(cpu.BX); bx&0x4000 == 0 || bx&0x0002 == 0 {
		t.Errorf("flags %04X lack LFN support (4000h) or case preservation (2)", bx)
	}
	if h.reg(cpu.CX) != 255 || h.reg(cpu.DX) != 260 {
		t.Errorf("limits %d/%d, want 255/260", h.reg(cpu.CX), h.reg(cpu.DX))
	}
	h.put(nameOff, `Q:\`)
	if !h.call6(0x71A0, 0, 32, nameOff, 0, bufOff) || h.reg(cpu.AX) != 0x0F {
		t.Errorf("unmapped drive: AX=%04X, want 000F", h.reg(cpu.AX))
	}
}

// Anything else answers 7100h: "not supported".
func TestLFNUnsupported(t *testing.T) {
	h := newHarness(t, 437, Config{})
	if !h.call(0x71A7, 0, 0, 0) || h.reg(cpu.AX) != 0x7100 {
		t.Errorf("AX=%04X, want CF and 7100", h.reg(cpu.AX))
	}
}

func TestLFNFind(t *testing.T) {
	h := newHarness(t, 866, Config{}, "Длинное имя файла.txt", "readme.txt", "名前.txt")
	if err := osMkdir(h, "Sub Directory"); err != nil {
		t.Fatal(err)
	}
	h.put(nameOff, `C:\*`)
	// SI=1: MS-DOS date and time.
	if h.call6(0x714E, 0, 0x0016, nameOff, 1, findOff) {
		t.Fatalf("714Eh failed: AX=%04X", h.reg(cpu.AX))
	}
	handle := h.reg(cpu.AX)
	if handle == 0 {
		t.Fatal("zero search handle")
	}
	type found struct {
		long, short string
		attr        uint32
		flags       uint16
	}
	var all []found
	for {
		long, short, attr := h.findData()
		all = append(all, found{long, short, attr, h.reg(cpu.CX)})
		if h.call6(0x714F, handle, 0, 0, 1, findOff) {
			if h.reg(cpu.AX) != errNoMore {
				t.Fatalf("714Fh: AX=%04X, want 0012", h.reg(cpu.AX))
			}
			break
		}
		if h.e.CPU.AH() != 0x4F {
			t.Errorf("714Fh: AH=%02X, want 4F", h.e.CPU.AH())
		}
	}
	if h.call(0x71A1, handle, 0, 0) {
		t.Fatal("71A1h failed")
	}
	if !h.call(0x71A1, handle, 0, 0) {
		t.Error("71A1h on a closed handle succeeded")
	}
	// Names are in cp866; the Japanese name does not fit and gets '_'.
	enc := func(s string) string { b, _ := h.d.fs.e.CP.Encode(s); return string(b) }
	// The Japanese name is given out under a unique alias and a long form with
	// '_' for each character and ~HHHH before the extension (names.go).
	jaShort, jaLong := AliasFor(h.d.fs.e.CP, "名前.txt")
	want := map[string]found{
		enc("Длинное имя файла.txt"): {enc("Длинное имя файла.txt"), enc("ДЛИННО~1.TXT"), 0x20, 0},
		"readme.txt":    {"readme.txt", "README.TXT", 0x20, 0},
		jaLong:          {jaLong, jaShort, 0x20, 1},
		"Sub Directory": {"Sub Directory", "SUBDIR~1", 0x10, 0},
	}
	if len(all) != len(want) {
		t.Fatalf("found %d entries, want %d: %+v", len(all), len(want), all)
	}
	for _, f := range all {
		w, ok := want[f.long]
		if !ok {
			t.Errorf("unexpected entry %+v", f)
			continue
		}
		if f.short != w.short {
			t.Errorf("%q: short name %q, want %q", f.long, f.short, w.short)
		}
		if f.attr != w.attr || f.flags != w.flags {
			t.Errorf("%q: attr %02X flags %d, want %02X/%d", f.long, f.attr, f.flags, w.attr, w.flags)
		}
	}
}

func TestLFNFindMasks(t *testing.T) {
	h := newHarness(t, 437, Config{}, "Long File Name.txt", "other.dat")
	if err := osMkdir(h, "Some Directory"); err != nil {
		t.Fatal(err)
	}
	find := func(pattern string, allow, must byte) []string {
		h.put(nameOff, pattern)
		var out []string
		if h.call6(0x714E, 0, uint16(must)<<8|uint16(allow), nameOff, 1, findOff) {
			if h.reg(cpu.AX) != errFileNotFound {
				t.Fatalf("%s: AX=%04X", pattern, h.reg(cpu.AX))
			}
			return nil
		}
		hd := h.reg(cpu.AX)
		for {
			l, _, _ := h.findData()
			out = append(out, l)
			if h.call6(0x714F, hd, 0, 0, 1, findOff) {
				break
			}
		}
		h.call(0x71A1, hd, 0, 0)
		return out
	}
	if got := find(`C:\*.*`, 0x16, 0); len(got) != 3 {
		t.Errorf("*.* found %v", got)
	}
	if got := find(`C:\long*`, 0x16, 0); len(got) != 1 || got[0] != "Long File Name.txt" {
		t.Errorf("long* found %v", got)
	}
	if got := find(`C:\*.TXT`, 0x16, 0); len(got) != 1 {
		t.Errorf("*.TXT found %v", got)
	}
	// The alias matches too.
	if got := find(`C:\LONGFI~1.TXT`, 0x16, 0); len(got) != 1 {
		t.Errorf("alias search found %v", got)
	}
	if got := find(`C:\*`, 0x16, 0x10); len(got) != 1 || got[0] != "Some Directory" {
		t.Errorf("required directory attribute found %v", got)
	}
	if got := find(`C:\*`, 0x00, 0); len(got) != 2 {
		t.Errorf("no directories allowed found %v", got)
	}
	if got := find(`C:\*.nothing`, 0x16, 0); got != nil {
		t.Errorf("found %v, want nothing", got)
	}
}

func TestLFNVolumeLabelSearch(t *testing.T) {
	h := newHarness(t, 437, Config{Labels: map[byte]string{'C': "Disk"}}, "readme.txt")
	h.put(nameOff, `C:\*.*`)
	if h.call6(0x714E, 0, 0x0008, nameOff, 1, findOff) {
		t.Fatalf("714Eh label search failed: AX=%04X", h.reg(cpu.AX))
	}
	if long, _, attr := h.findData(); long != "DISK" || attr != 8 {
		t.Errorf("label %q attr %02X", long, attr)
	}
}

func TestLFNFindDateTime(t *testing.T) {
	h := newHarness(t, 437, Config{}, "Some Name.txt")
	h.put(nameOff, `C:\Some*`)
	h.call6(0x714E, 0, 0x16, nameOff, 1, findOff)
	m := h.e.Mem
	info, err := os.Stat(filepath.Join(h.dir, "Some Name.txt"))
	if err != nil {
		t.Fatal(err)
	}
	tm, dt := dosTime(info.ModTime())
	if got := m.R32(mem.Lin(tDS, findOff+0x14)); got != uint32(dt)<<16|uint32(tm) {
		t.Errorf("DOS write time %08X, want %08X", got, uint32(dt)<<16|uint32(tm))
	}
	if m.R32(mem.Lin(tDS, findOff+0x20)) != 1 { // the file holds one byte
		t.Errorf("size %d", m.R32(mem.Lin(tDS, findOff+0x20)))
	}
	// SI=0: 64-bit file time, 100 ns since 1601.
	h.call6(0x714E, 0, 0x16, nameOff, 0, findOff)
	lo, hi := m.R32(mem.Lin(tDS, findOff+0x14)), m.R32(mem.Lin(tDS, findOff+0x18))
	if ft := uint64(hi)<<32 | uint64(lo); ft != fileTime64(info.ModTime()) || ft < 116444736000000000 {
		t.Errorf("file time %d", ft)
	}
}

func TestLFNTrueName(t *testing.T) {
	h := newHarness(t, 437, Config{}, "Long File Name.txt")
	if err := osMkdir(h, "Some Directory"); err != nil {
		t.Fatal(err)
	}
	h.d.fs.invalidate()
	os.WriteFile(filepath.Join(h.dir, "Some Directory", "Inner Name.dat"), nil, 0o644)

	truename := func(cl byte, path string) (string, uint16) {
		h.put(nameOff, path)
		if h.call6(0x7160, 0, uint16(cl), 0, nameOff, bufOff) {
			return "", h.reg(cpu.AX)
		}
		return h.asciiz(bufOff), 0
	}
	cases := []struct {
		cl      byte
		in, out string
		errc    uint16
		descr   string
	}{
		{1, `C:\Long File Name.txt`, `C:\LONGFI~1.TXT`, 0, "short name"},
		{1, `c:\some directory\inner name.dat`, `C:\SOMEDI~1\INNERN~1.DAT`, 0, "short path, any case"},
		{2, `C:\LONGFI~1.TXT`, `C:\Long File Name.txt`, 0, "long name from the alias"},
		{2, `C:\somedi~1\innern~1.dat`, `C:\Some Directory\Inner Name.dat`, 0, "long path from the alias"},
		{2, `C:\SOME DIRECTORY\new file.txt`, `C:\Some Directory\new file.txt`, 0, "file that does not exist yet"},
		{1, `C:\new.txt`, `C:\NEW.TXT`, 0, "short name of a new file"},
		{0, `c:\some directory\..\x.txt`, `C:\X.TXT`, 0, "canonical path"},
		{2, `C:\missing dir\x.txt`, ``, 2, "missing directory"},
		{1, `Q:\x`, ``, 3, "invalid drive"},
	}
	for _, c := range cases {
		got, errc := truename(c.cl, c.in)
		if got != c.out || errc != c.errc {
			t.Errorf("%s: 7160h CL=%d %q -> %q err %02X, want %q err %02X", c.descr, c.cl, c.in, got, errc, c.out, c.errc)
		}
	}
	// A relative path starts at the current directory.
	h.put(nameOff, `Some Directory`)
	if h.call(0x713B, 0, 0, nameOff) {
		t.Fatalf("713Bh failed: AX=%04X", h.reg(cpu.AX))
	}
	if got, _ := truename(2, `Inner Name.dat`); got != `C:\Some Directory\Inner Name.dat` {
		t.Errorf("relative long name %q", got)
	}
	// 7147h: the current directory in long names, without drive and backslash.
	if h.call6(0x7147, 0, 0, 0, bufOff, 0) {
		t.Fatalf("7147h failed: AX=%04X", h.reg(cpu.AX))
	}
	if got := h.asciiz(bufOff); got != "Some Directory" {
		t.Errorf("7147h: %q", got)
	}
	// The classic call sees the alias.
	h.call6(0x4700, 0, 0, 0, bufOff, 0)
	if got := h.asciiz(bufOff); got != "SOMEDI~1" {
		t.Errorf("4700h: %q, want SOMEDI~1", got)
	}
}

func TestLFNDirectories(t *testing.T) {
	h := newHarness(t, 437, Config{})
	h.put(nameOff, `C:\A Long Directory`)
	if h.call(0x7139, 0, 0, nameOff) {
		t.Fatalf("7139h failed: AX=%04X", h.reg(cpu.AX))
	}
	if st, err := os.Stat(filepath.Join(h.dir, "A Long Directory")); err != nil || !st.IsDir() {
		t.Fatalf("the host directory is missing: %v", err)
	}
	if !h.call(0x7139, 0, 0, nameOff) || h.reg(cpu.AX) != errAccess {
		t.Errorf("7139h on an existing directory: AX=%04X, want 0005", h.reg(cpu.AX))
	}
	if h.call(0x713B, 0, 0, nameOff) {
		t.Fatal("713Bh failed")
	}
	if !h.call(0x713A, 0, 0, nameOff) || h.reg(cpu.AX) != errCurDir {
		t.Errorf("713Ah on the current directory: AX=%04X, want 0010", h.reg(cpu.AX))
	}
	h.put(nameOff, `\`)
	h.call(0x713B, 0, 0, nameOff)
	h.put(nameOff, `a long directory`) // case does not matter
	if h.call(0x713A, 0, 0, nameOff) {
		t.Fatalf("713Ah failed: AX=%04X", h.reg(cpu.AX))
	}
	if _, err := os.Stat(filepath.Join(h.dir, "A Long Directory")); err == nil {
		t.Error("the directory is still there")
	}
}

func TestLFNOpen(t *testing.T) {
	h := newHarness(t, 437, Config{}, "Existing Long Name.txt")
	open := func(name string, mode, action uint16) (uint16, uint16, bool) {
		h.put(nameOff, name)
		cf := h.call6(0x716C, mode, 0, action, nameOff, 0)
		return h.reg(cpu.AX), h.reg(cpu.CX), cf
	}
	// Open an existing file by long name, any case and by alias.
	for _, n := range []string{`C:\Existing Long Name.txt`, `C:\existing long NAME.TXT`, `C:\EXISTI~1.TXT`} {
		hd, act, cf := open(n, 0, 0x01)
		if cf || act != 1 {
			t.Fatalf("open %q: CF=%v AX=%04X action %d", n, cf, hd, act)
		}
		h.call(0x3E00, hd, 0, 0)
	}
	// Open of a missing file fails; create-new of an existing one fails.
	if _, _, cf := open(`C:\no such file.txt`, 0, 0x01); !cf || h.reg(cpu.AX) != errFileNotFound {
		t.Errorf("open of a missing file: CF=%v AX=%04X", cf, h.reg(cpu.AX))
	}
	if _, _, cf := open(`C:\Existing Long Name.txt`, 2, 0x10); !cf || h.reg(cpu.AX) != errExists {
		t.Errorf("create-new of an existing file: CF=%v AX=%04X", cf, h.reg(cpu.AX))
	}
	// Create a file (action 12h as VC's 3Ch conversion does) and write to it.
	hd, act, cf := open(`C:\Brand New File.txt`, 2, 0x12)
	if cf || act != 2 {
		t.Fatalf("create: CF=%v AX=%04X action %d", cf, hd, act)
	}
	h.put(bufOff, "data")
	if h.call(0x4000, hd, 4, bufOff) {
		t.Fatalf("write failed: AX=%04X", h.reg(cpu.AX))
	}
	h.call(0x3E00, hd, 0, 0)
	if b, err := os.ReadFile(filepath.Join(h.dir, "Brand New File.txt")); err != nil || string(b) != "data" {
		t.Errorf("host file: %q %v", b, err)
	}
	// Replace it (truncate).
	hd, act, cf = open(`C:\brand new file.txt`, 2, 0x12)
	if cf || act != 3 {
		t.Fatalf("replace: CF=%v AX=%04X action %d", cf, hd, act)
	}
	h.call(0x3E00, hd, 0, 0)
	if st, _ := os.Stat(filepath.Join(h.dir, "Brand New File.txt")); st == nil || st.Size() != 0 {
		t.Errorf("not truncated: %v", st)
	}
}

func TestLFNRenameDelete(t *testing.T) {
	h := newHarness(t, 437, Config{}, "First Long Name.txt", "Second.txt", "Third One.txt", "Keep.dat")
	h.put(nameOff, `C:\First Long Name.txt`)
	h.put(nameOf2, `C:\Renamed To Another Long Name.txt`)
	if h.call6(0x7156, 0, 0, nameOff, 0, nameOf2) {
		t.Fatalf("7156h failed: AX=%04X", h.reg(cpu.AX))
	}
	if _, err := os.Stat(filepath.Join(h.dir, "Renamed To Another Long Name.txt")); err != nil {
		t.Errorf("renamed file: %v", err)
	}
	// An existing target is refused.
	h.put(nameOff, `C:\Second.txt`)
	h.put(nameOf2, `C:\keep.dat`)
	if !h.call6(0x7156, 0, 0, nameOff, 0, nameOf2) || h.reg(cpu.AX) != errAccess {
		t.Errorf("rename onto an existing file: AX=%04X, want 0005", h.reg(cpu.AX))
	}
	// Delete one file without wildcards.
	h.put(nameOff, `C:\third one.TXT`)
	if h.call6(0x7141, 0, 0, nameOff, 0, 0) {
		t.Fatalf("7141h failed: AX=%04X", h.reg(cpu.AX))
	}
	if _, err := os.Stat(filepath.Join(h.dir, "Third One.txt")); err == nil {
		t.Error("Third One.txt is still there")
	}
	// Wildcards need SI=1.
	h.put(nameOff, `C:\*.txt`)
	if h.call6(0x7141, 0, 0x0027, nameOff, 1, 0) {
		t.Fatalf("7141h with wildcards failed: AX=%04X", h.reg(cpu.AX))
	}
	left, _ := os.ReadDir(h.dir)
	if len(left) != 1 || left[0].Name() != "Keep.dat" {
		t.Errorf("left after *.txt: %v", left)
	}
	h.put(nameOff, `C:\*.txt`)
	if !h.call6(0x7141, 0, 0x0027, nameOff, 1, 0) || h.reg(cpu.AX) != errFileNotFound {
		t.Errorf("deleting nothing: AX=%04X, want 0002", h.reg(cpu.AX))
	}
}

func TestLFNAttributes(t *testing.T) {
	h := newHarness(t, 437, Config{}, "Some File.txt")
	h.put(nameOff, `C:\Some File.txt`)
	if h.call(0x7143, 0, 0, nameOff) || h.reg(cpu.CX) != 0x20 {
		t.Errorf("get attributes: CX=%04X, want 0020", h.reg(cpu.CX))
	}
	if h.call(0x7143, 1, 0x21, nameOff) {
		t.Fatalf("set attributes failed: AX=%04X", h.reg(cpu.AX))
	}
	if st, _ := os.Stat(filepath.Join(h.dir, "Some File.txt")); st.Mode().Perm()&0o200 != 0 {
		t.Error("the file is still writable")
	}
	h.call(0x7143, 0, 0, nameOff)
	if h.reg(cpu.CX)&1 == 0 {
		t.Errorf("read-only bit missing: CX=%04X", h.reg(cpu.CX))
	}
	h.call(0x7143, 1, 0x20, nameOff)
	// Write time: set, then get.
	tm, dt := uint16(12<<11|30<<5|10), uint16((2020-1980)<<9|6<<5|15)
	if h.call6(0x7143, 3, tm, nameOff, 0, dt) {
		t.Fatalf("set time failed: AX=%04X", h.reg(cpu.AX))
	}
	if h.call(0x7143, 4, 0, nameOff) || h.reg(cpu.CX) != tm || h.reg(cpu.DI) != dt {
		t.Errorf("get time: CX=%04X DI=%04X, want %04X %04X", h.reg(cpu.CX), h.reg(cpu.DI), tm, dt)
	}
	if h.call(0x7143, 2, 0, nameOff) || h.reg(cpu.AX) != 1 {
		t.Errorf("physical size: AX=%04X", h.reg(cpu.AX))
	}
	if !h.call(0x7143, 9, 0, nameOff) || h.reg(cpu.AX) != errInvalidFunc {
		t.Errorf("subfunction 9: AX=%04X, want 0001", h.reg(cpu.AX))
	}
	h.put(nameOff, `C:\Missing.txt`)
	if !h.call(0x7143, 0, 0, nameOff) || h.reg(cpu.AX) != errFileNotFound {
		t.Errorf("missing file: AX=%04X", h.reg(cpu.AX))
	}
}

// Config.NoLFN: every 71xx answers 7100h, including the volume query that
// would announce LFN support, and the classic calls keep working.
func TestNoLFN(t *testing.T) {
	h := newHarness(t, 437, Config{NoLFN: true}, "Long File Name.txt")
	h.put(nameOff, `C:\`)
	for _, ax := range []uint16{0x710D, 0x7139, 0x714E, 0x7160, 0x716C, 0x71A0} {
		if !h.call6(ax, 0, 32, nameOff, 0, bufOff) || h.reg(cpu.AX) != 0x7100 {
			t.Errorf("AX=%04X: AX=%04X, want CF and 7100", ax, h.reg(cpu.AX))
		}
	}
	h.put(nameOff, `C:\*.*`)
	if h.call(0x4E00, 0, 0, nameOff) {
		t.Errorf("the classic FindFirst failed: AX=%04X", h.reg(cpu.AX))
	}
}
