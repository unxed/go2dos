package machine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

// runHostAPI runs hostexec.com, a client of AMIS DOS-HOST/HOSTEXEC, with the
// tail " <m> <command>".
func runHostAPI(t *testing.T, cfg Config, tail string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "hostexec.com"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "HOSTEXEC.COM"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "SUB"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Drives, cfg.Codepage = map[byte]string{'C': dir}, 437
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\HOSTEXEC.COM`, tail); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Fatalf("exit code %d; screen:\n%s", c, m.Screen().Text())
	}
	return m.Screen().Text(), dir
}

// The API runs the line, prints its output to the caller's standard output
// and returns the exit code.
func TestHostExecAPI(t *testing.T) {
	text, _ := runHostAPI(t, Config{HostExec: true}, " C echo api-ok")
	l := lineList(text)
	if l[0] != "api-ok" || l[1] != "RC=0000 CF=00" {
		t.Fatalf("screen:\n%s", text)
	}
	text, _ = runHostAPI(t, Config{HostExec: true}, " C exit 5")
	if l := lineList(text); l[0] != "RC=0005 CF=00" {
		t.Fatalf("screen:\n%s", text)
	}
}

// A DOS directory is mapped to the host directory.
func TestHostExecAPIDir(t *testing.T) {
	_, dir := runHostAPI(t, Config{HostExec: true}, " D echo x> F.TXT")
	if _, err := os.Stat(filepath.Join(dir, "SUB", "F.TXT")); err != nil {
		t.Fatalf("F.TXT is not in SUB: %v", err)
	}
}

// Off by default: the provider is there and answers "access denied" (5).
func TestHostExecAPIDisabled(t *testing.T) {
	text, _ := runHostAPI(t, Config{}, " C echo api-ok")
	if l := lineList(text); l[0] != "RC=0005 CF=01" {
		t.Fatalf("screen:\n%s", text)
	}
}

// waitFile polls for a file written by a detached host process.
func waitFile(t *testing.T, p string) string {
	t.Helper()
	for i := 0; i < 100; i++ {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			return string(b)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no %s", p)
	return ""
}

// START host-command runs detached (no waiting, no output on the screen);
// the line goes to the host shell whole, redirects included.
func TestStartHostCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh")
	}
	_, text, dir := runShellCfg(t, Config{HostExec: true}, ` /C START echo started-ok > st.out`, nil)
	if l := lineList(text); l[0] != "EXIT AX=0000" {
		t.Fatalf("screen:\n%s", text)
	}
	if got := waitFile(t, filepath.Join(dir, "st.out")); got != "started-ok\n" {
		t.Fatalf("out %q", got)
	}
}

// START file hands a file (long name too) to the opener; -open-cmd replaces
// it: here the "opener" writes the path it was given.
func TestStartOpensFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh")
	}
	_, text, dir := runShellCfg(t, Config{HostExec: true, OpenCmd: "echo > opened.out"},
		` /C START "Long Doc Name.txt"`, map[string]string{"Long Doc Name.txt": "x"})
	if l := lineList(text); l[0] != "EXIT AX=0000" {
		t.Fatalf("screen:\n%s", text)
	}
	got := waitFile(t, filepath.Join(dir, "opened.out"))
	if !strings.HasSuffix(strings.TrimSpace(got), "Long Doc Name.txt") {
		t.Fatalf("opener got %q", got)
	}
}

// START of a DOS program is the same as typing it; START needs -host-exec.
func TestStartDOSProgramAndOff(t *testing.T) {
	_, text, _ := runShellCfg(t, Config{HostExec: true}, " /C START hello", nil)
	if l := lineList(text); l[0] == "" || strings.HasPrefix(l[0], "Bad command") || strings.HasPrefix(l[0], "Host commands") {
		t.Fatalf("screen:\n%s", text)
	}
	_, text, _ = runShell(t, " /C START hello", nil)
	if l := lineList(text); !strings.HasPrefix(l[0], "Host commands are disabled") || l[1] != "EXIT AX=00FF" {
		t.Fatalf("screen:\n%s", text)
	}
	_, text, _ = runShellCfg(t, Config{HostExec: true}, " /C START", nil)
	if l := lineList(text); !strings.HasPrefix(l[0], "Usage: START") {
		t.Fatalf("screen:\n%s", text)
	}
}
