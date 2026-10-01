package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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
