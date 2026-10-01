package machine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/dos"
	"github.com/unxed/go2dos/hle"
)

// A provider is found by scanning the multiplex numbers and comparing the
// signature; the standard functions answer as AMIS 3.6 says and the private
// function reaches the provider.
func TestAMISScan(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "amis.com"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AMIS.COM"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	mux, err := m.DOS.RegisterAMIS(dos.AMISProvider{
		Manufacturer: "go2dos", Product: "TESTPROV", Description: "test provider", Version: 0x0102,
		Call: func(e *hle.Env, fn byte) (bool, error) {
			if fn != 0x10 {
				return false, nil
			}
			e.CPU.R[cpu.AX] = 0xBEEF
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mux != 0xC0 {
		t.Errorf("first multiplex number %02X, want C0", mux)
	}
	if err := m.Load(`C:\AMIS.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d; screen:\n%s", c, m.Screen().Text())
	}
	want := []string{"MUX=C0 VER=0102", "PRIV=BEEF", "HOOKS=04 2D", "UNINST=01"}
	for i, w := range want {
		if got := m.Screen().Line(i); got != w {
			t.Fatalf("line %d = %q, want %q\nscreen:\n%s", i, got, w, m.Screen().Text())
		}
	}
	// The INT 2Dh vector points at an interrupt sharing protocol header
	// (RBIL table 02568): short jump EB 10, signature 424Bh at offset 6.
	seg, off := m.Env.Vector(0x2D)
	a := uint32(seg)<<4 + uint32(off)
	if m.Mem.R8(a) != 0xEB || m.Mem.R8(a+1) != 0x10 || m.Mem.R16(a+6) != 0x424B {
		t.Errorf("no ISP header at %04X:%04X: % X", seg, off, m.Mem.Bytes(a, 18))
	}
}

// Two providers get different numbers; a free number answers AL=00h.
func TestAMISNumbers(t *testing.T) {
	m, err := New(Config{Drives: map[byte]string{'C': t.TempDir()}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.DOS.RegisterAMIS(dos.AMISProvider{Manufacturer: "go2dos", Product: "ONE"})
	b, _ := m.DOS.RegisterAMIS(dos.AMISProvider{Manufacturer: "go2dos", Product: "TWO"})
	if a == b {
		t.Errorf("both providers on %02X", a)
	}
}
