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
func runShellCfg(t *testing.T, cfg Config, tail string, files map[string]string, pre ...func(*Machine)) (*Machine, string, string) {
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
	for _, f := range pre {
		f(m)
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

// CLIP: show, set from the line, from a file, clear; redirection with a long name, and
// >> keeps what the file had.
func TestShellClip(t *testing.T) {
	clip := &dos.MemClipboard{Text: "first\nsecond"}
	bat := "@echo off\r\n" +
		"clip\r\n" +
		"clip > \"clip out.txt\"\r\n" +
		"echo more >> \"clip out.txt\"\r\n" +
		"clip this text\r\n" +
		"clip < in.txt\r\n"
	_, text, dir := runShellCfg(t, Config{Clipboard: clip}, " /C T5.BAT", map[string]string{"T5.BAT": bat, "in.txt": "line1\r\nline2\r\n"})
	if !strings.Contains(text, "first") || !strings.Contains(text, "second") {
		t.Errorf("CLIP did not show the text:\n%s", text)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "clip out.txt")); string(b) != "first\r\nsecond\r\nmore\r\n" {
		t.Errorf("clip out.txt = %q", b)
	}
	if clip.Text != "line1\nline2\n" {
		t.Errorf("clipboard %q, want the file text", clip.Text)
	}
}

func TestShellClipSetAndClear(t *testing.T) {
	clip := &dos.MemClipboard{Text: "old"}
	runShellCfg(t, Config{Clipboard: clip}, " /C CLIP this text", nil)
	if clip.Text != "this text" {
		t.Errorf("clipboard %q", clip.Text)
	}
	runShellCfg(t, Config{Clipboard: clip}, " /C CLIP /C", nil)
	if clip.Text != "" {
		t.Errorf("clipboard %q after CLIP /C", clip.Text)
	}
}

func TestShellClipNone(t *testing.T) {
	_, text, _ := runShell(t, " /C CLIP", nil)
	if !strings.Contains(text, "No clipboard") {
		t.Errorf("screen:\n%s", text)
	}
}

// CLS clears the emulated screen; MODE CON shows and sets the window size.
func TestShellClsAndMode(t *testing.T) {
	bat := "@echo off\r\necho before\r\ncls\r\necho after\r\nmode con\r\n"
	_, text, _ := runShell(t, " /C T6.BAT", map[string]string{"T6.BAT": bat})
	if strings.Contains(text, "before") {
		t.Errorf("CLS left the text:\n%s", text)
	}
	for _, w := range []string{"after", "Lines:        25", "Columns:      80"} {
		if !strings.Contains(text, w) {
			t.Errorf("screen lacks %q:\n%s", w, text)
		}
	}
}

func TestShellModeSet(t *testing.T) {
	m, _, _ := runShell(t, " /C MODE CON COLS=100 LINES=40", nil)
	if cols, rows := m.BIOS.Video.Size(); cols != 100 || rows != 40 {
		t.Errorf("size %dx%d, want 100x40", cols, rows)
	}
}

func TestShellModeBad(t *testing.T) {
	_, text, _ := runShell(t, " /C MODE CON COLS=10", nil)
	if !strings.Contains(text, "columns 80-255") {
		t.Errorf("screen:\n%s", text)
	}
}

// In console mode CLS also clears the terminal (the host's Clear), while the stream is shown.
func TestShellClsClearsConsole(t *testing.T) {
	n := 0
	runShellCfg(t, Config{Display: "console"}, " /C CLS", nil, func(m *Machine) {
		m.SetConsoleOutput(func([]byte) {}, func(bool) {})
		m.SetConsoleClear(func() { n++ })
	})
	if n != 1 {
		t.Errorf("terminal cleared %d times, want 1", n)
	}
}

// -ro: nothing that the shell does changes the drive (the backstop of dos/fsro.go).
func TestShellReadOnlyDrive(t *testing.T) {
	bat := "@echo off\r\n" +
		"copy hello.com copy.com\r\n" +
		"del hello.com\r\n" +
		"ren mark.com other.com\r\n" +
		"md newdir\r\n" +
		"rd sub\r\n" +
		"echo x > out.txt\r\n" +
		"echo y >> keep.txt\r\n" +
		"type keep.txt\r\n"
	_, text, dir := runShellCfg(t, Config{ReadOnly: map[byte]bool{'C': true}}, " /C T7.BAT", map[string]string{"T7.BAT": bat, "keep.txt": "kept\r\n"})
	for _, gone := range []string{"copy.com", "other.com", "newdir", "out.txt"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s was created", gone)
		}
	}
	for _, kept := range []string{"hello.com", "mark.com", "SUB"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s is gone: %v", kept, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "keep.txt")); string(b) != "kept\r\n" {
		t.Errorf("keep.txt = %q", b)
	}
	if !strings.Contains(text, "kept") {
		t.Errorf("reading must still work:\n%s", text)
	}
}
