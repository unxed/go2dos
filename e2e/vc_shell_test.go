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

// A command typed on the VC command line runs through COMSPEC (C:\COMMAND.COM,
// which is not a file: the built-in interpreter, dos/shell.go) and VC comes
// back. mark.com writes its command tail to RAN.TXT; ECHO with > is an internal
// command of the shell and shows that the shell, not VC, ran the line.
func vcCommandLine(t *testing.T, version string) {
	mark, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "mark.com"))
	if err != nil {
		t.Fatal(err)
	}
	m, runErr, dir := sessionDir(t, version,
		`<waitfor:10Quit>echo shell>OUT.TXT<Enter><wait:3s><waitfor:10Quit>mark.com one two<Enter><wait:3s><waitfor:10Quit><F10><waitfor:Do you want to quit><Enter>`,
		map[string]string{"MARK.COM": string(mark)})
	var ex *machine.ExitError
	if !errors.As(runErr, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", runErr, m.Screen().Text())
	}
	b, err := os.ReadFile(filepath.Join(dir, "RAN.TXT"))
	if err != nil {
		t.Fatalf("the program did not run: %v", err)
	}
	t.Logf("RAN.TXT = %q", b)
	// ECHO with redirection is an internal command: only the shell can do it.
	if o, err := os.ReadFile(filepath.Join(dir, "OUT.TXT")); err != nil || string(o) != "shell\r\n" {
		t.Fatalf("OUT.TXT = %q, %v", o, err)
	}
}

func TestVC405CommandLine(t *testing.T)   { vcCommandLine(t, "4.05") }
func TestVC49909CommandLine(t *testing.T) { vcCommandLine(t, "4.99.09") }

// A host command with ! prefix executes on the host and prints its output.
func vcHostExec(t *testing.T, version string) {
	t.Helper()

	// Create a temporary directory for the test
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

	// Create a machine with HostExec enabled
	m, err := machine.New(machine.Config{
		Drives:   map[byte]string{'C': dir},
		Codepage: 437,
		HostExec: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\VC.COM`, ""); err != nil {
		t.Fatal(err)
	}

	// Parse and run the script
	script := `<waitfor:10Quit>!echo hostexec-ok<Enter><wait:3s><waitfor:10Quit><F10><waitfor:Do you want to quit><Enter>`
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

	var ex *machine.ExitError
	if !errors.As(runErr, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", runErr, m.Screen().Text())
	}

	// The output "hostexec-ok" should appear on the screen.
	screen := m.Screen().Text()
	if !strings.Contains(screen, "hostexec-ok") {
		t.Fatalf("output does not contain 'hostexec-ok'; screen:\n%s", screen)
	}
}

func TestVC405HostExec(t *testing.T)   { vcHostExec(t, "4.05") }
func TestVC49909HostExec(t *testing.T) { vcHostExec(t, "4.99.09") }
