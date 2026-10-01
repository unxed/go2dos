package machine

import (
	"errors"
	"testing"
	"time"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/keys"
)

func kbdExitCode(t *testing.T, err error) int {
	t.Helper()
	var ex *ExitError
	if !errors.As(err, &ex) {
		t.Fatalf("want an exit, got %v", err)
	}
	return ex.Code
}

func namedKey(t *testing.T, name string, mods byte) bios.KeyEvent {
	t.Helper()
	k, ok := keys.Named(name, mods)
	if !ok {
		t.Fatalf("no key %q", name)
	}
	return k
}

// INT 16h AH=00h, then AH=02h: the flags show the modifiers the key was pressed
// with, although the key's break code has long been handled.
//
//	mov ah,0 / int 16h / mov ah,2 / int 16h / mov ah,4Ch / int 21h   (exit code = flags)
func TestKeyReadKeepsModifiers(t *testing.T) {
	prog := []byte{0xB4, 0x00, 0xCD, 0x16, 0xB4, 0x02, 0xCD, 0x16, 0xB4, 0x4C, 0xCD, 0x21}
	for _, c := range []struct {
		name string
		mods byte
		want int
	}{{"Ins", bios.ModLShift, 2}, {"Ins", bios.ModCtrl, 4}, {"Ins", 0, 0}} {
		_, err := runBytes(t, prog, func(m *Machine) { m.PushKey(namedKey(t, c.name, c.mods)) }, 2*time.Second)
		if got := kbdExitCode(t, err); got != c.want {
			t.Errorf("%s with mods %d: flags %02X, want %02X", c.name, c.mods, got, c.want)
		}
	}
}

// The flags last until the next INT 16h read or poll (AH=01h): a program that
// shows hints for a held Shift must not see it stuck after the key was read.
//
//	read; poll (AH=01h); get flags
func TestKeyModifiersEndAtNextPoll(t *testing.T) {
	prog := []byte{0xB4, 0x00, 0xCD, 0x16, 0xB4, 0x01, 0xCD, 0x16, 0xB4, 0x02, 0xCD, 0x16, 0xB4, 0x4C, 0xCD, 0x21}
	_, err := runBytes(t, prog, func(m *Machine) { m.PushKey(namedKey(t, "Ins", bios.ModLShift)) }, 2*time.Second)
	if got := kbdExitCode(t, err); got != 0 {
		t.Errorf("flags %02X after the next poll, want 0", got)
	}
}

// Two keys queued before the program reads: each read shows the flags of its
// own key, not of the last one pressed.
//
//	read; flags -> bl; read; flags; exit code = bl<<4 | flags
func TestKeyModifiersPerKeyInBurst(t *testing.T) {
	prog := []byte{
		0xB4, 0x00, 0xCD, 0x16, 0xB4, 0x02, 0xCD, 0x16, 0x88, 0xC3,
		0xB4, 0x00, 0xCD, 0x16, 0xB4, 0x02, 0xCD, 0x16,
		0xB1, 0x04, 0xD2, 0xE3, 0x08, 0xD8, 0xB4, 0x4C, 0xCD, 0x21,
	}
	for _, c := range []struct {
		first, second byte
		want          int
	}{{bios.ModLShift, 0, 0x20}, {0, bios.ModCtrl, 0x04}, {bios.ModCtrl, bios.ModLShift, 0x42}} {
		_, err := runBytes(t, prog, func(m *Machine) {
			m.PushKey(namedKey(t, "Ins", c.first))
			m.PushKey(namedKey(t, "Ins", c.second))
		}, 2*time.Second)
		if got := kbdExitCode(t, err); got != c.want {
			t.Errorf("mods %d then %d: %02X, want %02X", c.first, c.second, got, c.want)
		}
	}
}
