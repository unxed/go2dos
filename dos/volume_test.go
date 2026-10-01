package dos

import (
	"testing"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/mem"
)

// INT 21h AX=440Dh CX=0866h (RBIL, MS-DOS 4.0+): get media ID.
func TestGetMediaID(t *testing.T) {
	h := newHarness(t, 437, Config{})
	if cf := h.call(0x440D, 3, 0x0866, 0x0100); cf {
		t.Fatalf("CF set, AX=%04X", h.e.CPU.R[cpu.AX])
	}
	buf := h.bytes(0x0100, 0x19)
	if buf[0] != 0 || buf[1] != 0 {
		t.Errorf("info level %v, want 0", buf[:2])
	}
	want := h.d.fs.serial(2)
	got := uint32(buf[2]) | uint32(buf[3])<<8 | uint32(buf[4])<<16 | uint32(buf[5])<<24
	if got != want || got == 0 {
		t.Errorf("serial %08X, want %08X (non-zero)", got, want)
	}
	if s := string(buf[6:17]); s != "NO NAME    " {
		t.Errorf("label %q, want NO NAME", s)
	}
	if s := string(buf[17:25]); s != "FAT16   " {
		t.Errorf("file system %q", s)
	}
	// The default drive (BL=0) is C: here.
	h.call(0x440D, 0, 0x0866, 0x0200)
	if string(h.bytes(0x0200, 0x19)) != string(buf) {
		t.Error("BL=0 differs from BL=3")
	}
	// AH=69h AL=0 returns the same structure.
	if cf := h.call(0x6900, 3, 0, 0x0300); cf {
		t.Fatal("69h failed")
	}
	if string(h.bytes(0x0300, 0x19)) != string(buf) {
		t.Error("69h differs from 440Dh/66h")
	}
}

func TestGetMediaIDLabel(t *testing.T) {
	h := newHarness(t, 437, Config{Labels: map[byte]string{'c': "mydisk"}})
	h.call(0x440D, 3, 0x0866, 0x0100)
	if s := string(h.bytes(0x0100+6, 11)); s != "MYDISK     " {
		t.Errorf("label %q", s)
	}
}

func TestGenericIOCTLErrors(t *testing.T) {
	h := newHarness(t, 437, Config{})
	// D: is not mapped: invalid drive.
	if cf := h.call(0x440D, 4, 0x0866, 0x0100); !cf || h.e.CPU.R[cpu.AX] != 0x0F {
		t.Errorf("unmapped drive: CF=%v AX=%04X, want CF and 000F", cf, h.e.CPU.R[cpu.AX])
	}
	// Not the disk category: invalid function.
	if cf := h.call(0x440D, 3, 0x0166, 0x0100); !cf || h.e.CPU.R[cpu.AX] != 1 {
		t.Errorf("category 01h: CF=%v AX=%04X, want CF and 0001", cf, h.e.CPU.R[cpu.AX])
	}
}

func TestDriveIOCTL(t *testing.T) {
	h := newHarness(t, 437, Config{})
	if h.call(0x4408, 3, 0, 0) || h.e.CPU.R[cpu.AX] != 1 {
		t.Errorf("4408h: AX=%04X, want 1 (fixed)", h.e.CPU.R[cpu.AX])
	}
	if h.call(0x4409, 3, 0, 0xFFFF) || h.e.CPU.R[cpu.DX] != 0 {
		t.Errorf("4409h: DX=%04X, want 0 (local)", h.e.CPU.R[cpu.DX])
	}
	if h.call(0x440E, 3, 0, 0) || h.e.CPU.AL() != 0 {
		t.Errorf("440Eh: AL=%02X, want 0", h.e.CPU.AL())
	}
	if h.call(0x440F, 3, 0, 0) {
		t.Error("440Fh failed")
	}
	for _, ax := range []uint16{0x4408, 0x4409, 0x440E, 0x440F} {
		if cf := h.call(ax, 9, 0, 0); !cf || h.e.CPU.R[cpu.AX] != 0x0F {
			t.Errorf("%04Xh on an unmapped drive: CF=%v AX=%04X, want CF and 000F", ax, cf, h.e.CPU.R[cpu.AX])
		}
	}
	// 4401h: closed handle, DH != 0 (invalid data), then success.
	if cf := h.call(0x4401, 7, 0, 0); !cf || h.e.CPU.R[cpu.AX] != 6 {
		t.Errorf("4401h bad handle: CF=%v AX=%04X, want CF and 0006", cf, h.e.CPU.R[cpu.AX])
	}
	if cf := h.call(0x4401, 0, 0, 0x0200); !cf || h.e.CPU.R[cpu.AX] != 0x0D {
		t.Errorf("4401h DH=2: CF=%v AX=%04X, want CF and 000D", cf, h.e.CPU.R[cpu.AX])
	}
	if cf := h.call(0x4401, 0, 0, 0x00A0); cf {
		t.Errorf("4401h on CON failed: AX=%04X", h.e.CPU.R[cpu.AX])
	}
}

// AH=32h: host directories have no DPB; the documented answer is AL=FFh.
func TestGetDPB(t *testing.T) {
	h := newHarness(t, 437, Config{})
	h.call(0x3200, 0, 0, 3)
	if h.e.CPU.AL() != 0xFF {
		t.Errorf("AL=%02X, want FF", h.e.CPU.AL())
	}
}

// findLabel runs FindFirst with attribute attr on pattern and returns the
// name found (DTA+1Eh) and its attribute, or the error code.
func findFirst(h *harness, pattern string, attr uint16) (string, byte, uint16) {
	h.put(0x0010, pattern)
	if cf := h.call(0x4E00, 0, attr, 0x0010); cf {
		return "", 0, h.e.CPU.R[cpu.AX]
	}
	m := h.e.Mem
	a := mem.Lin(tDTA, 0)
	return string(m.ASCIIZ(a+0x1E, 13)), m.R8(a + 0x15), 0
}

// FindFirst with attribute 08h returns the volume label (RBIL INT 21h AH=4Eh).
func TestFindVolumeLabel(t *testing.T) {
	h := newHarness(t, 437, Config{Labels: map[byte]string{'C': "Disk"}}, "readme.txt")
	if err := osMkdir(h, "sub"); err != nil {
		t.Fatal(err)
	}
	name, attr, errc := findFirst(h, `C:\*.*`, 0x08)
	if errc != 0 || name != "DISK" || attr != 0x08 {
		t.Fatalf("got %q attr %02X err %02X, want DISK 08", name, attr, errc)
	}
	// Only the label: no second match.
	if cf := h.call(0x4F00, 0, 0, 0); !cf {
		t.Error("FindNext after the label found another entry")
	}
	// A volume-label search looks in the root whatever the path says.
	if name, _, errc := findFirst(h, `C:\SUB\*.*`, 0x08); errc != 0 || name != "DISK" {
		t.Errorf(`C:\SUB\*.* attr 08: %q err %02X`, name, errc)
	}
	// Ordinary searches do not see the label.
	if name, _, _ := findFirst(h, `C:\*.*`, 0x16); name != "README.TXT" {
		t.Errorf("attr 16h found %q, want README.TXT", name)
	}
	// With 08h among other bits the label comes first, then the files.
	if name, attr, _ := findFirst(h, `C:\*.*`, 0x18); name != "DISK" || attr != 0x08 {
		t.Errorf("attr 18h first: %q %02X", name, attr)
	}
	if cf := h.call(0x4F00, 0, 0, 0); cf {
		t.Error("attr 18h: files after the label are missing")
	}
	// A label longer than 8 characters gets a dot after the eighth.
	h2 := newHarness(t, 437, Config{Labels: map[byte]string{'C': "ABCDEFGHIJK"}})
	if name, _, _ := findFirst(h2, `C:\*.*`, 0x08); name != "ABCDEFGH.IJK" {
		t.Errorf("long label %q", name)
	}
}

func TestFindNoVolumeLabel(t *testing.T) {
	h := newHarness(t, 437, Config{}, "readme.txt")
	if _, _, errc := findFirst(h, `C:\*.*`, 0x08); errc != 0x02 {
		t.Errorf("error %02X, want 02 (nothing found)", errc)
	}
}
