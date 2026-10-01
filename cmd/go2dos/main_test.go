package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Тест запускает собственный тестовый бинарник как go2dos: при
// GO2DOS_TEST_RUN=1 он вместо тестов исполняет run() с флагами из командной
// строки (так проверяется разбор флагов и коды выхода без go build).
func TestMain(m *testing.M) {
	if os.Getenv("GO2DOS_TEST_PTY") == "1" && ptyHelperMain != nil {
		os.Exit(ptyHelperMain())
	}
	if os.Getenv("GO2DOS_TEST_RUN") == "1" {
		os.Exit(run())
	}
	os.Exit(m.Run())
}

// ptyHelperMain задаётся в attach_linux_test.go: хост-процесс для теста в pty.
var ptyHelperMain func() int

// COM-программа: INT 21h AH=5Ah (не поддерживается), затем выход с кодом 0
// только если ответ «неверная функция» (CF=1, AX=1), иначе с кодом 1.
var unsupportedProg = []byte{
	0xB4, 0x5A, 0xCD, 0x21, // mov ah,5Ah; int 21h
	0x73, 0x09, // jnc bad
	0x3D, 0x01, 0x00, // cmp ax,1
	0x75, 0x04, // jne bad
	0xB8, 0x00, 0x4C, 0xCD, 0x21, // mov ax,4C00h; int 21h
	0xB8, 0x01, 0x4C, 0xCD, 0x21, // bad: mov ax,4C01h; int 21h
}

func runGo2dos(t *testing.T, args ...string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "UNSUP.COM"), unsupportedProg, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], append(args, filepath.Join(dir, "UNSUP.COM"))...)
	cmd.Env = append(os.Environ(), "GO2DOS_TEST_RUN=1")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, errb.String()
}

func TestFlagLenient(t *testing.T) {
	code, out := runGo2dos(t, "-headless", "-timeout", "10s", "-cp", "437", "-lenient", "-dump-dir", t.TempDir())
	if code != 0 {
		t.Fatalf("exit code %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "unsupported calls answered in lenient mode") || !strings.Contains(out, "int21") || !strings.Contains(out, "AH=5Ah") {
		t.Errorf("no summary in stderr:\n%s", out)
	}
}

func TestNoLenientFailsFast(t *testing.T) {
	code, out := runGo2dos(t, "-headless", "-timeout", "10s", "-cp", "437", "-dump-dir", t.TempDir())
	if code != 3 {
		t.Fatalf("exit code %d, want 3 (fail fast)\n%s", code, out)
	}
	if !strings.Contains(out, "unsupported: INT 21h AH=5Ah") || strings.Contains(out, "lenient mode") {
		t.Errorf("unexpected stderr:\n%s", out)
	}
}

// Pipe mode (docs/SCREEN.md, S1): with stdin and stdout that are not
// terminals go2dos is a filter. The child process has pipes for all three
// streams, so it needs no -pipe flag.
func runPipeChild(t *testing.T, prog []byte, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "PIPE.COM")
	if err := os.WriteFile(path, prog, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], append(args, path)...)
	cmd.Env = append(os.Environ(), "GO2DOS_TEST_RUN=1")
	cmd.Stdin = strings.NewReader(stdin)
	var outb, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outb, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, outb.String(), errb.String()
}

// hello.com prints a line with INT 21h/09h and exits with code 7.
func TestPipeModeHello(t *testing.T) {
	hello, err := os.ReadFile(filepath.Join("..", "..", "testdata", "progs", "hello.com"))
	if err != nil {
		t.Fatal(err)
	}
	code, out, errs := runPipeChild(t, hello, "", "-cp", "437")
	if code != 7 || out != "Hello from go2dos\r\n" {
		t.Errorf("exit code %d, stdout %q (stderr %q)", code, out, errs)
	}
	if strings.Contains(errs, "exit code") {
		t.Errorf("a normal exit is not reported in pipe mode: stderr %q", errs)
	}
}

// A filter: reads standard input with INT 21h/3Fh until the end and writes it
// to standard output with 40h (the program is hand-assembled, see
// machine/pipe_test.go). Text is converted UTF-8 <-> the code page both ways.
var echoProg = []byte{
	0xB4, 0x3F, 0x31, 0xDB, 0xB9, 0x40, 0x00, 0xBA, 0x00, 0x02, 0xCD, 0x21,
	0x72, 0x19, 0x09, 0xC0, 0x74, 0x10,
	0x89, 0xC1, 0xB4, 0x40, 0xBB, 0x01, 0x00, 0xBA, 0x00, 0x02, 0xCD, 0x21,
	0x72, 0x07, 0xEB, 0xDE,
	0xB8, 0x00, 0x4C, 0xCD, 0x21,
	0xB8, 0x01, 0x4C, 0xCD, 0x21,
}

func TestPipeModeFilter(t *testing.T) {
	cases := []struct {
		page, in, out string
	}{
		{"437", "café Ж\n", "café ?\r\n"},
		{"866", "Привет, ä!\nsecond", "Привет, ?!\r\nsecond"},
	}
	for _, c := range cases {
		code, out, errs := runPipeChild(t, echoProg, c.in, "-cp", c.page)
		if code != 0 || out != c.out {
			t.Errorf("cp%s %q: exit code %d, stdout %q, want %q (stderr %q)", c.page, c.in, code, out, c.out, errs)
		}
	}
}
