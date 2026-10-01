package machine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// "!" sends the line to the host shell; the output lands on the DOS screen
// and the exit code comes back.
func TestHostExecBang(t *testing.T) {
	_, text, _ := runShellCfg(t, Config{HostExec: true}, " /C !echo hostexec-ok", nil)
	l := lineList(text)
	if l[0] != "hostexec-ok" || l[1] != "EXIT AX=0000" {
		t.Fatalf("screen:\n%s", text)
	}
	_, text, _ = runShellCfg(t, Config{HostExec: true}, " /C !exit 3", nil)
	if l := lineList(text); l[0] != "EXIT AX=0003" {
		t.Fatalf("screen:\n%s", text)
	}
}

// Off by default: no way out of the sandbox.
func TestHostExecOffByDefault(t *testing.T) {
	_, text, _ := runShell(t, " /C !echo hostexec-ok", nil)
	l := lineList(text)
	if !strings.HasPrefix(l[0], "Host commands are disabled") || l[1] != "EXIT AX=00FF" {
		t.Fatalf("screen:\n%s", text)
	}
	// A command that is only on the host is not found either.
	_, text, _ = runShell(t, " /C go version", nil)
	if l := lineList(text); l[0] != "Bad command or file name" {
		t.Fatalf("screen:\n%s", text)
	}
}

// A command that is not a DOS program but is in the host PATH runs there.
func TestHostExecPath(t *testing.T) {
	_, text, _ := runShellCfg(t, Config{HostExec: true}, " /C go version", nil)
	if l := lineList(text); !strings.HasPrefix(l[0], "go version") {
		t.Fatalf("screen:\n%s", text)
	}
}

// The working directory is the host directory of the DOS current directory,
// and the host shell does the redirection itself.
func TestHostExecCwd(t *testing.T) {
	bat := "@echo off\r\ncd SUB\r\n!echo x> F.TXT\r\n"
	_, _, dir := runShellCfg(t, Config{HostExec: true}, " /C T.BAT", map[string]string{"T.BAT": bat})
	if _, err := os.Stat(filepath.Join(dir, "SUB", "F.TXT")); err != nil {
		t.Fatalf("F.TXT is not in SUB: %v", err)
	}
}

// DOS wins on a name clash: HELLO.COM is a DOS program even with host-exec.
func TestHostExecDOSFirst(t *testing.T) {
	_, text, _ := runShellCfg(t, Config{HostExec: true}, " /C HELLO.COM", nil)
	if l := lineList(text); l[0] != "Hello from go2dos" || l[1] != "EXIT AX=0007" {
		t.Fatalf("screen:\n%s", text)
	}
}
