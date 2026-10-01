// Package e2e runs real DOS programs end to end. The programs are not part
// of the repository: tools/fetch-vc.sh downloads Volkov Commander (BSD-2)
// and the test is skipped unless GO2DOS_VC_DIR points at the result.
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

func vcDir(t *testing.T, version string) string {
	root := os.Getenv("GO2DOS_VC_DIR")
	if root == "" {
		t.Skip("GO2DOS_VC_DIR not set (run tools/fetch-vc.sh)")
	}
	return filepath.Join(root, version)
}

// session runs VC in a fresh drive with a few files and returns the screens
// captured by the script.
func session(t *testing.T, version, script string) (*machine.Machine, error) {
	t.Helper()
	return sessionFiles(t, version, script, nil)
}

// sessionFiles is session with extra files (name -> content) in the drive.
func sessionFiles(t *testing.T, version, script string, extra map[string]string) (*machine.Machine, error) {
	t.Helper()
	m, err, _ := sessionDir(t, version, script, extra)
	return m, err
}

// sessionDir is sessionFiles that also returns the host directory of drive C:.
func sessionDir(t *testing.T, version, script string, extra map[string]string) (*machine.Machine, error, string) {
	t.Helper()
	return sessionOpts(t, version, script, extra, machine.Config{})
}

// sessionOpts is sessionDir with machine settings (drives and code page are set here).
func sessionOpts(t *testing.T, version, script string, extra map[string]string, cfg machine.Config) (*machine.Machine, error, string) {
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
	os.WriteFile(filepath.Join(dir, "README.TXT"), []byte("hello\r\n"), 0o644)
	os.Mkdir(filepath.Join(dir, "SUBDIR"), 0o755)
	for name, content := range extra {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}

	cfg.Drives = map[byte]string{'C': dir}
	if cfg.Codepage == 0 {
		cfg.Codepage = 437
	}
	m, err := machine.New(cfg)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	scriptErr := make(chan error, 1)
	go func() { scriptErr <- m.RunScript(ctx, steps, machine.ScriptOptions{}) }()
	runErr := m.Run(ctx)
	if err := <-scriptErr; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("script: %v\nmachine: %v", err, runErr)
	}
	return m, runErr, dir
}

func TestVC405PanelsAndQuit(t *testing.T) {
	m, err := session(t, "4.05",
		`<waitfor:10Quit><waitfor:readme   txt><Tab><waitfor:SUBDIR><Down><Enter><waitfor:C:\SUBDIR><F10><waitfor:Do you want to quit><Enter>`)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}
	if !strings.Contains(m.Screen().Text(), "The Volkov Commander, Version 4.05") {
		t.Errorf("exit screen:\n%s", m.Screen().Text())
	}
}

// VC 4.99.09 starts through VC.OVL (EXEC) and reads the volume serial number
// with INT 21h AX=440Dh CX=0866h (VCLABEL.INC, GetVol); the test checks that
// it gets as far as the panels, can change into a directory and quit.
func TestVC49909PanelsAndQuit(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><waitfor:readme   txt><Tab><waitfor:SUBDIR><Down><Enter><waitfor:C:\SUBDIR><F10><waitfor:Do you want to quit><Enter>`)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}
}

// VC 4.99.09 tries every file call as INT 21h AX=71xxh first (VCCOMMON.INC,
// Intr21). Its panels start in short-name mode, showing the aliases; Ctrl-N
// switches to long names, which the test checks on a directory with long names.
func TestVC49909LongNames(t *testing.T) {
	m, err := sessionFiles(t, "4.99.09",
		`<waitfor:10Quit><waitfor:averyl~1 txt><waitfor:second~1 doc><Ctrl-N><waitfor:A Very Long><waitfor:second long><F10><waitfor:Do you want to quit><Enter>`,
		map[string]string{"A Very Long File Name.txt": "x", "second long document name.doc": "x"})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}
}
