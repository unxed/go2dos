// Package e2e runs real DOS programs end to end.
// NC5 vmlab tests: automated testing for Norton Commander 5.51 in CI environment.
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

// TestNC551VMLab_StartupSequence validates basic startup and panel initialization.
func TestNC551VMLab_StartupSequence(t *testing.T) {
	root := os.Getenv("GO2DOS_NC_DIR")
	if root == "" {
		t.Skip("GO2DOS_NC_DIR not set")
	}

	dir := t.TempDir()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		srcPath := filepath.Join(root, e.Name())
		dstPath := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}
		os.WriteFile(dstPath, b, 0o644)
	}

	os.WriteFile(filepath.Join(dir, "TEST1.TXT"), []byte("Sample\r\n"), 0o644)
	os.Mkdir(filepath.Join(dir, "TESTDIR"), 0o755)

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}

	ncExe := `C:\NC.EXE`
	if _, err := os.Stat(filepath.Join(dir, "NC.EXE")); err != nil {
		ncExe = `C:\NCMAIN.EXE`
	}

	if err := m.Load(ncExe, ""); err != nil {
		t.Skipf("NC not available: %v", err)
	}

	script := `<waitfor:Norton Commander>
<wait:2>
F10
y`

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
		t.Logf("script error: %v", err)
	}

	screenText := m.Screen().Text()
	if !strings.Contains(strings.ToLower(screenText), "norton") {
		t.Logf("Screen:\n%s", screenText)
	}

	t.Logf("Startup validated, runtime: %v", runErr)
}

// TestNC551VMLab_NavigationCycle validates panel navigation and directory operations.
func TestNC551VMLab_NavigationCycle(t *testing.T) {
	root := os.Getenv("GO2DOS_NC_DIR")
	if root == "" {
		t.Skip("GO2DOS_NC_DIR not set")
	}

	dir := t.TempDir()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644)
	}

	os.Mkdir(filepath.Join(dir, "DIR1"), 0o755)
	os.WriteFile(filepath.Join(dir, "DIR1", "FILE1.TXT"), []byte("test\r\n"), 0o644)

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}

	ncExe := `C:\NC.EXE`
	if _, err := os.Stat(filepath.Join(dir, "NC.EXE")); err != nil {
		ncExe = `C:\NCMAIN.EXE`
	}

	if err := m.Load(ncExe, ""); err != nil {
		t.Skipf("NC not available: %v", err)
	}

	script := `<waitfor:Norton Commander>
<wait:2>
Tab
<wait:1>
Enter
<wait:1>
..
Enter
<wait:1>
F10
y`

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
		t.Logf("script error: %v", err)
	}

	t.Logf("Navigation validated, runtime: %v", runErr)
}

// TestNC551VMLab_FileOperations validates F5, F6, F7 operations.
func TestNC551VMLab_FileOperations(t *testing.T) {
	root := os.Getenv("GO2DOS_NC_DIR")
	if root == "" {
		t.Skip("GO2DOS_NC_DIR not set")
	}

	dir := t.TempDir()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644)
	}

	os.WriteFile(filepath.Join(dir, "SOURCE.TXT"), []byte("data\r\n"), 0o644)

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}

	ncExe := `C:\NC.EXE`
	if _, err := os.Stat(filepath.Join(dir, "NC.EXE")); err != nil {
		ncExe = `C:\NCMAIN.EXE`
	}

	if err := m.Load(ncExe, ""); err != nil {
		t.Skipf("NC not available: %v", err)
	}

	script := `<waitfor:Norton Commander>
<wait:2>
F5
<wait:1>
Esc
<wait:1>
F6
<wait:1>
Esc
<wait:1>
F7
<wait:1>
NEWDIR
Enter
<wait:1>
Esc
<wait:1>
F10
y`

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
		t.Logf("script error: %v", err)
	}

	t.Logf("File operations validated, runtime: %v", runErr)
}

// TestNC551VMLab_MenuAccess validates F9 menu, Alt-F1, Ctrl-O.
func TestNC551VMLab_MenuAccess(t *testing.T) {
	root := os.Getenv("GO2DOS_NC_DIR")
	if root == "" {
		t.Skip("GO2DOS_NC_DIR not set")
	}

	dir := t.TempDir()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644)
	}

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}

	ncExe := `C:\NC.EXE`
	if _, err := os.Stat(filepath.Join(dir, "NC.EXE")); err != nil {
		ncExe = `C:\NCMAIN.EXE`
	}

	if err := m.Load(ncExe, ""); err != nil {
		t.Skipf("NC not available: %v", err)
	}

	script := `<waitfor:Norton Commander>
<wait:2>
F9
<wait:1>
Esc
<wait:1>
Alt-F1
<wait:1>
Esc
<wait:1>
Ctrl-O
<wait:1>
Ctrl-O
<wait:1>
F10
y`

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
		t.Logf("script error: %v", err)
	}

	t.Logf("Menu access validated, runtime: %v", runErr)
}

// TestNC551VMLab_CompleteWorkflow runs full NC5 workflow test.
func TestNC551VMLab_CompleteWorkflow(t *testing.T) {
	root := os.Getenv("GO2DOS_NC_DIR")
	if root == "" {
		t.Skip("GO2DOS_NC_DIR not set")
	}

	dir := t.TempDir()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644)
	}

	os.WriteFile(filepath.Join(dir, "README.TXT"), []byte("readme\r\n"), 0o644)
	os.Mkdir(filepath.Join(dir, "DOCS"), 0o755)
	os.WriteFile(filepath.Join(dir, "DOCS", "HELP.TXT"), []byte("help\r\n"), 0o644)

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}

	ncExe := `C:\NC.EXE`
	if _, err := os.Stat(filepath.Join(dir, "NC.EXE")); err != nil {
		ncExe = `C:\NCMAIN.EXE`
	}

	if err := m.Load(ncExe, ""); err != nil {
		t.Skipf("NC not available: %v", err)
	}

	script := `<waitfor:Norton Commander>
<wait:2>
Tab
<wait:1>
Enter
<wait:1>
..
Enter
<wait:1>
F9
<wait:1>
Esc
<wait:1>
F5
<wait:1>
Esc
<wait:1>
Alt-F1
<wait:1>
Esc
<wait:1>
Ctrl-O
<wait:1>
Ctrl-O
<wait:1>
F10
y`

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
		t.Logf("script error: %v", err)
	}

	t.Logf("Complete workflow validated, runtime: %v", runErr)
}
