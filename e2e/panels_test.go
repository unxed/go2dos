package e2e

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
)

// Why only one panel is drawn (docs/NC-TESTING.md has the facts): VC.ASM,
// Init03 ("Default init", used when VC.INI cannot be read) copies the default
// window state to both panels and then stores CL, which is 0 after REP MOVSB,
// into WCB1.Visible: the left panel starts switched off. VC 4.99.09 says so on
// screen ("Can not read the setup file: C:\VC.INI. Use Shift-F9 to create a new
// setup file."). With a VC.INI saved while both panels were on, both are drawn
// at once. It is the program's own default, not an emulator defect.

// copyDir copies the plain files of src into a fresh drive directory.
func copyDir(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "README.TXT"), []byte("hello\r\n"), 0o644)
	os.Mkdir(filepath.Join(dir, "SUBDIR"), 0o755)
	return dir
}

// frameCorners counts the upper left panel corners in the top row that has
// any: one per panel.
func frameCorners(scr string) int {
	for _, l := range strings.Split(scr, "\n") {
		if strings.ContainsRune(l, '╔') {
			return strings.Count(l, "╔")
		}
	}
	return 0
}

// phase is a part of a key script; after it ends the screen must show
// panels panels (-1: do not check).
type phase struct {
	script string
	panels int
	what   string
}

// runPhases runs prog from the host directory dir (drive C:) through the
// phases and checks the panel count after each. If the program has not exited
// when the last phase is done, it is stopped. It returns the machine's result.
func runPhases(t *testing.T, dir, prog string, lenient bool, phases []phase) error {
	t.Helper()
	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: 437, Lenient: lenient})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\`+prog, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	scriptErr := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		// After the last phase give the program a moment to exit by itself.
		defer func() {
			select {
			case <-exited:
			case <-time.After(3 * time.Second):
			}
			cancel()
		}()
		for _, ph := range phases {
			st, err := keys.Parse(ph.script, m.CP)
			if err != nil {
				scriptErr <- err
				return
			}
			if err := m.RunScript(ctx, st, machine.ScriptOptions{}); err != nil {
				scriptErr <- err
				return
			}
			if ph.panels >= 0 {
				if got := frameCorners(m.Screen().Text()); got != ph.panels {
					t.Errorf("%s: %d panels, want %d; screen:\n%s", ph.what, got, ph.panels, m.Screen().Text())
				}
			}
		}
		scriptErr <- nil
	}()
	runErr := m.Run(ctx)
	close(exited)
	if err := <-scriptErr; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("script: %v\nmachine: %v\nscreen:\n%s", err, runErr, m.Screen().Text())
	}
	return runErr
}

func vcTwoPanels(t *testing.T, version string) {
	dir := copyDir(t, vcDir(t, version))

	// Session 1, no VC.INI: the program's own default is one panel, the right
	// one. Ctrl-F1 ("on/off left panel") switches the left one on, Shift-F9
	// saves VC.INI, F10 quits.
	err := runPhases(t, dir, "VC.COM", false, []phase{
		{`<waitfor:10Quit><wait:300ms>`, 1, "no VC.INI"},
		{`<Ctrl-F1><waitfor:║║><wait:300ms>`, 2, "after Ctrl-F1"},
		{`<Shift-F9><waitfor:current setup><Enter><wait:500ms><F10><waitfor:Do you want to quit><Enter>`, -1, ""},
	})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("session 1: want exit 0, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "VC.INI")); err != nil {
		t.Fatalf("VC.INI was not saved: %v", err)
	}

	// Session 2 with that VC.INI: both panels at once, no key pressed.
	runPhases(t, dir, "VC.COM", false, []phase{
		{`<waitfor:10Quit><wait:300ms>`, 2, "with the saved VC.INI"},
	})
}

func TestVC405TwoPanels(t *testing.T)   { vcTwoPanels(t, "4.05") }
func TestVC49909TwoPanels(t *testing.T) { vcTwoPanels(t, "4.99.09") }

// NC 5.51 (tools/fetch-nc.sh): the shipped directory has no NC.INI, and NC
// starts with the right panel only, like VC without VC.INI. Ctrl-F1 brings the
// left panel back. (Why NC starts that way is not established.) Needs no
// -lenient: INT 10h AH=FFh is answered.
func TestNC551TwoPanels(t *testing.T) {
	d := os.Getenv("GO2DOS_NC_DIR")
	if d == "" {
		t.Skip("GO2DOS_NC_DIR not set (run tools/fetch-nc.sh)")
	}
	dir := copyDir(t, d)
	runPhases(t, dir, "NC.EXE", false, []phase{
		{`<waitfor:10Quit><wait:2s>`, 1, "NC without NC.INI"},
		{`<Ctrl-F1><waitfor:║║><wait:1s>`, 2, "NC after Ctrl-F1"},
	})
}
