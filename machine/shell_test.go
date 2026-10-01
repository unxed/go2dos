package machine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/dos"
)

// runShell runs runcmd.com, which executes C:\COMMAND.COM (not a file: the
// built-in interpreter) with the given command tail. files are extra files
// of the drive (name -> content).
func runShell(t *testing.T, tail string, files map[string]string) (*Machine, string, string) {
	t.Helper()
	return runShellCfg(t, Config{}, tail, files)
}

// runShellCfg is runShell with extra machine settings (drives and code page are set here).
func runShellCfg(t *testing.T, cfg Config, tail string, files map[string]string) (*Machine, string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"runcmd.com", "hello.com", "mark.com"} {
		src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", n))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, n), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "SUB"), 0o755); err != nil {
		t.Fatal(err)
	}
	for n, c := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Drives, cfg.Codepage = map[byte]string{'C': dir}, 437
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\RUNCMD.COM`, tail); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = m.Run(ctx)
	if c := exitCode(t, err); c != 0 {
		t.Fatalf("runcmd exit code %d", c)
	}
	return m, m.Screen().Text(), dir
}

func lineList(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		out = append(out, strings.TrimRight(l, " "))
	}
	return out
}

// A program from the command line: its output, and its exit code comes back.
func TestShellRunsProgram(t *testing.T) {
	_, text, _ := runShell(t, " /C HELLO.COM", nil)
	l := lineList(text)
	if l[0] != "Hello from go2dos" || l[1] != "EXIT AX=0007" {
		t.Fatalf("screen:\n%s", text)
	}
}

// The extension may be left out (COM, EXE, BAT in that order); the tail is
// passed as typed.
func TestShellTail(t *testing.T) {
	_, text, dir := runShell(t, " /C mark -a  b/c", nil)
	if l := lineList(text); l[0] != "EXIT AX=0003" {
		t.Fatalf("screen:\n%s", text)
	}
	b, err := os.ReadFile(filepath.Join(dir, "RAN.TXT"))
	if err != nil || string(b) != " -a  b/c" {
		t.Fatalf("RAN.TXT = %q, %v", b, err)
	}
}

func TestShellEcho(t *testing.T) {
	_, text, _ := runShell(t, " /C ECHO hi there", nil)
	l := lineList(text)
	if l[0] != "hi there" || l[1] != "EXIT AX=0000" {
		t.Fatalf("screen:\n%s", text)
	}
}

func TestShellUnknown(t *testing.T) {
	_, text, _ := runShell(t, " /C nosuchprog", nil)
	l := lineList(text)
	if l[0] != "Bad command or file name" || l[1] != "EXIT AX=00FF" {
		t.Fatalf("screen:\n%s", text)
	}
}

// A batch file: parameters, SET and %VAR%, redirection, TYPE, an external
// program with redirected output.
func TestShellBatch(t *testing.T) {
	bat := "@echo off\r\nset FOO=bar\r\necho %1 done\r\necho %FOO%\r\n" +
		"echo data> OUT.TXT\r\ntype OUT.TXT\r\nhello.com > H.TXT\r\ntype H.TXT\r\nset\r\n"
	_, text, dir := runShell(t, " /C T1.BAT x", map[string]string{"T1.BAT": bat})
	l := lineList(text)
	want := []string{"x done", "bar", "data", "Hello from go2dos"}
	for i, w := range want {
		if l[i] != w {
			t.Fatalf("line %d = %q, want %q\nscreen:\n%s", i, l[i], w, text)
		}
	}
	if !strings.Contains(text, "FOO=bar") || !strings.Contains(text, "COMSPEC=C:\\COMMAND.COM") {
		t.Errorf("SET lacks the variables:\n%s", text)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "H.TXT")); err != nil || string(b) != "Hello from go2dos\r\n" {
		t.Errorf("H.TXT = %q, %v", b, err)
	}
}

func TestShellDirAndCD(t *testing.T) {
	bat := "@echo off\r\ncd SUB\r\ncd\r\ncd ..\r\ndir\r\n"
	_, text, _ := runShell(t, " /C T2.BAT", map[string]string{"T2.BAT": bat})
	if l := lineList(text); l[0] != `C:\SUB` {
		t.Fatalf("screen:\n%s", text)
	}
	for _, w := range []string{`Directory of C:\`, "HELLO    COM", "SUB", "<DIR>", "file(s)", "dir(s)"} {
		if !strings.Contains(text, w) {
			t.Errorf("DIR output lacks %q:\n%s", w, text)
		}
	}
}

// With a real COMMAND.COM file the built-in interpreter is not used.
func TestShellFileWins(t *testing.T) {
	hello, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "hello.com"))
	if err != nil {
		t.Fatal(err)
	}
	_, text, _ := runShell(t, " /C ignored", map[string]string{"COMMAND.COM": string(hello)})
	if l := lineList(text); l[0] != "Hello from go2dos" || l[1] != "EXIT AX=0007" {
		t.Fatalf("screen:\n%s", text)
	}
}

// Quotes and long names in the file commands, and the long name in DIR.
func TestShellLongNames(t *testing.T) {
	bat := "@echo off\r\n" +
		"copy \"A long file name.txt\" \"Copy of it.txt\"\r\n" +
		"ren \"Copy of it.txt\" \"Second name.txt\"\r\n" +
		"md \"New dir with spaces\"\r\n" +
		"cd \"New dir with spaces\"\r\n" +
		"copy \"..\\A long file name.txt\" inside.txt\r\n" +
		"cd ..\r\n" +
		"type \"Second name.txt\"\r\n" +
		"dir\r\n" +
		"del \"A long file name.txt\"\r\n"
	_, text, dir := runShell(t, " /C T3.BAT", map[string]string{"T3.BAT": bat, "A long file name.txt": "long-content\r\n"})
	for _, w := range []string{"long-content", "A long file name.txt", "Second name.txt", "New dir with spaces"} {
		if !strings.Contains(text, w) {
			t.Errorf("screen lacks %q:\n%s", w, text)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "A long file name.txt")); err == nil {
		t.Errorf("DEL left the file")
	}
	for _, p := range []string{"Second name.txt", filepath.Join("New dir with spaces", "inside.txt")} {
		if b, err := os.ReadFile(filepath.Join(dir, p)); err != nil || string(b) != "long-content\r\n" {
			t.Errorf("%s = %q, %v", p, b, err)
		}
	}
}

// A name that the code page cannot show is listed by its alias, and the alias works.
func TestShellAliasName(t *testing.T) {
	host := "Файл.txt" // cp437 has no Cyrillic
	page, err := cp.Get(437)
	if err != nil {
		t.Fatal(err)
	}
	short, _ := dos.AliasFor(page, host)
	bat := "@echo off\r\ndir\r\ntype " + short + "\r\n"
	_, text, _ := runShell(t, " /C T4.BAT", map[string]string{"T4.BAT": bat, host: "alias-content\r\n"})
	if !strings.Contains(text, "alias-content") {
		t.Errorf("TYPE by alias failed:\n%s", text)
	}
}
