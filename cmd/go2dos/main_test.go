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
	if os.Getenv("GO2DOS_TEST_RUN") == "1" {
		os.Exit(run())
	}
	os.Exit(m.Run())
}

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
