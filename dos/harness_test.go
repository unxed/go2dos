package dos

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Segments of the test buffers, above the kernel data and below the stack.
const (
	tDS    = 0x5000
	tDTA   = 0x5800
	tStack = 0x6000
	tPSP   = 0x4000
)

// harness calls INT 21h handlers directly, without running guest code.
type harness struct {
	t   *testing.T
	d   *DOS
	e   *hle.Env
	dir string
}

func newHarness(t *testing.T, page int, cfg Config, files ...string) *harness {
	t.Helper()
	dir := t.TempDir()
	for _, n := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := mem.New()
	c, err := cp.Get(page)
	if err != nil {
		t.Fatal(err)
	}
	e := hle.New(cpu.New(m, nil), m, c, time.Now, hle.NewTracer(4, nil, nil))
	cfg.Drives = map[byte]string{'C': dir}
	d, err := New(e, bios.New(e), cfg)
	if err != nil {
		t.Fatal(err)
	}
	d.setDTA(tDTA, 0)
	// A PSP with a handle table: handle 0 is the console, the others are closed.
	d.psp = tPSP
	m.W16(mem.Lin(tPSP, 0x32), 20)
	m.W32(mem.Lin(tPSP, 0x34), uint32(tPSP)<<16|0x18)
	for i := uint32(0); i < 20; i++ {
		m.W8(mem.Lin(tPSP, 0x18)+i, 0xFF)
	}
	m.W8(mem.Lin(tPSP, 0x18), 0)
	return &harness{t: t, d: d, e: e, dir: dir}
}

// call runs INT 21h with the given registers and reports CF.
func (h *harness) call(ax, bx, cx, dx uint16) bool { return h.call6(ax, bx, cx, dx, 0, 0) }

// call6 is call with SI and DI as well.
func (h *harness) call6(ax, bx, cx, dx, si, di uint16) bool {
	h.t.Helper()
	c := h.e.CPU
	c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX] = ax, bx, cx, dx
	c.R[cpu.SI], c.R[cpu.DI] = si, di
	c.SetSeg(cpu.DS, tDS)
	c.SetSeg(cpu.ES, tDS)
	c.SetSeg(cpu.SS, tStack)
	c.R[cpu.SP] = 0x100
	c.Write16(cpu.SS, c.R[cpu.SP]+4, 0) // FLAGS in the INT frame
	if err := h.d.int21(h.e); err != nil {
		h.t.Fatalf("INT 21h AX=%04X: %v", ax, err)
	}
	return h.e.CallerFlags()&cpu.FlagCF != 0
}

// put stores an ASCIZ string at DS:off.
func (h *harness) put(off uint16, s string) {
	h.e.Mem.SetBytes(mem.Lin(tDS, off), append([]byte(s), 0))
}

// bytes returns n bytes at DS:off.
func (h *harness) bytes(off uint16, n int) []byte {
	return append([]byte(nil), h.e.Mem.Bytes(mem.Lin(tDS, off), n)...)
}

func osMkdir(h *harness, name string) error { return os.Mkdir(filepath.Join(h.dir, name), 0o755) }
