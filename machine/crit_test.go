package machine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/bios"
)

// runCrit runs a test program from testdata/progs with a command tail and
// keys queued up front (or, when waitFor is set, once line 0 of the screen
// shows it). Drive D is a second directory; cfg may add options.
func runCrit(t *testing.T, prog, tail string, cfg Config, keys ...bios.KeyEvent) (*Machine, []string, error) {
	t.Helper()
	cdir, ddir := t.TempDir(), t.TempDir()
	for _, n := range []string{"crit24.com", "ctrlc.com"} {
		src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", n))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cdir, n), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Drives = map[byte]string{'C': cdir, 'D': ddir}
	cfg.Codepage = 437
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\`+strings.ToUpper(prog), tail); err != nil {
		t.Fatal(err)
	}
	if waitFor == "" {
		for _, k := range keys {
			m.PushKey(k)
		}
	} else {
		// Keys that must not arrive before the program has hooked a vector.
		go func() {
			for strings.TrimSpace(m.Screen().Line(0)) != waitFor {
				time.Sleep(time.Millisecond)
			}
			for _, k := range keys {
				m.PushKey(k)
			}
		}()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = m.Run(ctx)
	var lines []string
	for i := 0; i < 8; i++ {
		lines = append(lines, strings.TrimRight(m.Screen().Line(i), " "))
	}
	return m, lines, err
}

func wantLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("screen line %d = %q, want %q\nscreen:\n%s", i, got[i], w, strings.Join(got, "\n"))
		}
	}
	if len(got) > len(want) && got[len(want)] != "" {
		t.Fatalf("extra output after the expected lines:\n%s", strings.Join(got, "\n"))
	}
}

// waitFor is set by a test that needs its keys after the program's first line.
var waitFor string

const (
	hFail  = "H AH=1C AL=03 DI=0002 AX=3D00 BP=0070 SI=02C0 N=01"
	hRetry = "H AH=1C AL=03 DI=0002 AX=3D00 BP=0070 SI=02C0 N=02"
)

var (
	keyCtrlC  = bios.KeyEvent{Scan: 0x2E, ASCII: 3, Mods: bios.ModCtrl}
	keyBreak  = bios.KeyEvent{Scan: 0x46, Mods: bios.ModCtrl}
	keyLetter = bios.KeyEvent{Scan: 0x2D, ASCII: 'x'}
)

// No fault configured: no INT 24h; the missing file is plain error 2.
func TestCritNone(t *testing.T) {
	_, lines, err := runCrit(t, "crit24.com", " 3O", Config{})
	exitCode(t, err)
	wantLines(t, lines, "OPEN CF=01 AX=0002")
}

// A drive that is not ready: INT 24h gets RBIL's arguments, Fail fails the call.
func TestCritNotReadyFail(t *testing.T) {
	_, lines, err := runCrit(t, "crit24.com", " 3O", Config{NotReady: map[byte]bool{'D': true}})
	if c := exitCode(t, err); c != 0 {
		t.Fatalf("exit code %d", c)
	}
	wantLines(t, lines, hFail, "OPEN CF=01 AX=0003")
}

// Retry repeats the call (and calls INT 24h again); the program's handler
// answers Fail the second time.
func TestCritRetry(t *testing.T) {
	_, lines, err := runCrit(t, "crit24.com", " 1O", Config{NotReady: map[byte]bool{'D': true}})
	exitCode(t, err)
	wantLines(t, lines, hFail, hRetry, "OPEN CF=01 AX=0003")
}

// Ignore is not allowed (AH bit 5 clear), so it becomes Fail (RBIL Int 24).
func TestCritIgnoreIsFail(t *testing.T) {
	_, lines, err := runCrit(t, "crit24.com", " 0O", Config{NotReady: map[byte]bool{'D': true}})
	exitCode(t, err)
	wantLines(t, lines, hFail, "OPEN CF=01 AX=0003")
}

// Abort ends the program with exit type 2 and code 0 (4Dh gives AX=0200).
func TestCritAbort(t *testing.T) {
	_, lines, err := runCrit(t, "crit24.com", " 2P", Config{NotReady: map[byte]bool{'D': true}})
	exitCode(t, err)
	wantLines(t, lines, hFail, "EXIT AX=0200")
}

// Without a handler of its own the kernel's one answers Fail.
func TestCritKernelDefault(t *testing.T) {
	_, lines, err := runCrit(t, "crit24.com", " 9O", Config{NotReady: map[byte]bool{'D': true}})
	exitCode(t, err)
	wantLines(t, lines, "OPEN CF=01 AX=0003")
}

// Write protection: writes raise error 00h (AH bit 0 set), reads do not.
func TestCritWriteProtect(t *testing.T) {
	cfg := Config{WriteProtect: map[byte]bool{'D': true}}
	_, lines, err := runCrit(t, "crit24.com", " 3C", cfg)
	exitCode(t, err)
	wantLines(t, lines, "H AH=1D AL=03 DI=0000 AX=3C00 BP=0070 SI=02C0 N=01", "OPEN CF=01 AX=0003")
	_, lines, err = runCrit(t, "crit24.com", " 3O", cfg)
	exitCode(t, err)
	wantLines(t, lines, "OPEN CF=01 AX=0002")
}

// Free space of a drive that is not ready: FAT area, AX=FFFFh.
func TestCritFreeSpace(t *testing.T) {
	_, lines, err := runCrit(t, "crit24.com", " 3F", Config{NotReady: map[byte]bool{'D': true}})
	exitCode(t, err)
	wantLines(t, lines, "H AH=1A AL=03 DI=0002 AX=3600 BP=0070 SI=02C0 N=01", "FREE AX=FFFF")
}

// ^C in a console function: DOS prints ^C, calls INT 23h; IRET repeats the call.
func TestCtrlCRepeat(t *testing.T) {
	_, lines, err := runCrit(t, "ctrlc.com", " 0", Config{}, keyCtrlC, keyLetter)
	exitCode(t, err)
	wantLines(t, lines, "^C", "CC", "GOT x")
}

// Ctrl-Break from the keyboard: INT 09h, INT 1Bh (DOS's one), then the same.
func TestCtrlBreakRepeat(t *testing.T) {
	_, lines, err := runCrit(t, "ctrlc.com", " 0", Config{}, keyBreak, keyLetter)
	exitCode(t, err)
	wantLines(t, lines, "^C", "CC", "GOT x")
}

// INT 23h that returns with STC, RETF ends the program: exit type 1, code 0.
func TestCtrlCAbort(t *testing.T) {
	_, lines, err := runCrit(t, "ctrlc.com", " 1P", Config{}, keyCtrlC)
	exitCode(t, err)
	wantLines(t, lines, "^C", "EXIT AX=0100")
}

// The default INT 23h also ends the program, here after Ctrl-Break.
func TestCtrlCDefault(t *testing.T) {
	_, lines, err := runCrit(t, "ctrlc.com", " 2P", Config{}, keyBreak)
	exitCode(t, err)
	wantLines(t, lines, "^C", "EXIT AX=0100")
}

// A program's own INT 1Bh replaces DOS's: it is called by INT 09h with bit 7
// of 0040:0071 set, and DOS sees no ^C.
func TestCtrlBreakOwnInt1B(t *testing.T) {
	waitFor = "R"
	defer func() { waitFor = "" }()
	_, lines, err := runCrit(t, "ctrlc.com", " 3", Config{}, keyBreak, keyLetter)
	exitCode(t, err)
	wantLines(t, lines, "R", "B71=80", "GOT x")
}
