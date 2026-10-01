package machine

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// Pipe mode (docs/SCREEN.md, S1): DOS handles 0, 1, 2 are host streams.
// The programs are hand-assembled COM files (the bytes were checked with a
// disassembler; the listing is next to each).

// progEcho copies standard input to standard output with INT 21h AH=3Fh/40h
// until a read returns 0 bytes; exit code 0 (1 on a DOS error).
//
//	100: mov ah,3Fh / xor bx,bx / mov cx,40h / mov dx,200h / int 21h
//	10C: jc fail / or ax,ax / jz done
//	112: mov cx,ax / mov ah,40h / mov bx,1 / mov dx,200h / int 21h
//	11E: jc fail / jmp 100
//	122: done: mov ax,4C00h / int 21h
//	127: fail: mov ax,4C01h / int 21h
var progEcho = []byte{
	0xB4, 0x3F, 0x31, 0xDB, 0xB9, 0x40, 0x00, 0xBA, 0x00, 0x02, 0xCD, 0x21,
	0x72, 0x19, 0x09, 0xC0, 0x74, 0x10,
	0x89, 0xC1, 0xB4, 0x40, 0xBB, 0x01, 0x00, 0xBA, 0x00, 0x02, 0xCD, 0x21,
	0x72, 0x07, 0xEB, 0xDE,
	0xB8, 0x00, 0x4C, 0xCD, 0x21,
	0xB8, 0x01, 0x4C, 0xCD, 0x21,
}

// progOut writes 'A' with INT 29h, 'B' with AH=40h to handle 1, 0x82 with
// AH=02h, "D" with AH=09h, 'E' with AH=40h to handle 2; exit code 5.
var progOut = []byte{
	0xB0, 0x41, 0xCD, 0x29,
	0xB4, 0x40, 0xBB, 0x01, 0x00, 0xB9, 0x01, 0x00, 0xBA, 0x30, 0x01, 0xCD, 0x21,
	0xB2, 0x82, 0xB4, 0x02, 0xCD, 0x21,
	0xBA, 0x31, 0x01, 0xB4, 0x09, 0xCD, 0x21,
	0xB4, 0x40, 0xBB, 0x02, 0x00, 0xB9, 0x01, 0x00, 0xBA, 0x33, 0x01, 0xCD, 0x21,
	0xB8, 0x05, 0x4C, 0xCD, 0x21,
	'B', 'D', '$', 'E',
}

// progLine reads one line with INT 21h AH=0Ah (buffer at 200h, size 20), writes
// the text and the CR with AH=40h, then reads characters with AH=08h and writes
// each with AH=02h until ^Z (the end of the input); exit code 0.
var progLine = []byte{
	0xC6, 0x06, 0x00, 0x02, 0x14, 0xBA, 0x00, 0x02, 0xB4, 0x0A, 0xCD, 0x21,
	0x8A, 0x0E, 0x01, 0x02, 0x30, 0xED, 0x41,
	0xBA, 0x02, 0x02, 0xBB, 0x01, 0x00, 0xB4, 0x40, 0xCD, 0x21,
	0xB4, 0x08, 0xCD, 0x21, 0x3C, 0x1A, 0x74, 0x08,
	0x88, 0xC2, 0xB4, 0x02, 0xCD, 0x21, 0xEB, 0xF0,
	0xB8, 0x00, 0x4C, 0xCD, 0x21,
}

type pipeResult struct {
	stdout, stderr string
	code           int
}

func runPipe(t *testing.T, page int, prog []byte, stdin io.Reader) pipeResult {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "P.COM"), prog, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: page, Stdin: stdin, Stdout: &out, Stderr: &errb})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\P.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	code := exitCode(t, m.Run(ctx))
	return pipeResult{out.String(), errb.String(), code}
}

func TestPipeHello(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "testdata", "progs", "hello.com"))
	if err != nil {
		t.Fatal(err)
	}
	r := runPipe(t, 437, src, nil)
	if r.stdout != "Hello from go2dos\r\n" || r.stderr != "" || r.code != 7 {
		t.Errorf("stdout %q, stderr %q, exit code %d", r.stdout, r.stderr, r.code)
	}
}

// Input is UTF-8 -> OEM -> program -> OEM -> UTF-8: the program sees bytes of
// the code page and a CR LF at the end of a line; unmappable characters are '?'.
func TestPipeEchoConversion(t *testing.T) {
	cases := []struct {
		page    int
		in, out string
	}{
		{437, "h\u00e9llo \u0416!\n", "h\u00e9llo ?!\r\n"},
		{437, "a\r\nb\nc", "a\r\nb\r\nc"},
		{866, "\u0416\u0443\u043a\n", "\u0416\u0443\u043a\r\n"},
		{437, "\xffab\xe2\x82", "?ab??"}, // invalid UTF-8, then a sequence cut short by the end
	}
	for _, c := range cases {
		r := runPipe(t, c.page, progEcho, strings.NewReader(c.in))
		if r.stdout != c.out || r.code != 0 {
			t.Errorf("cp%d %q: stdout %q (exit %d), want %q", c.page, c.in, r.stdout, r.code, c.out)
		}
	}
}

// A character split over several reads of the host stream is converted whole.
func TestPipeSplitCharacter(t *testing.T) {
	r := runPipe(t, 866, progEcho, iotest.OneByteReader(strings.NewReader("\u0416\u0443\u043a\n")))
	if r.stdout != "\u0416\u0443\u043a\r\n" || r.code != 0 {
		t.Errorf("stdout %q (exit %d)", r.stdout, r.code)
	}
}

// INT 29h (the console), AH=40h, AH=02h and AH=09h all reach standard output in
// the order of the calls; handle 2 reaches standard error.
func TestPipeOutputPaths(t *testing.T) {
	r := runPipe(t, 437, progOut, nil)
	if r.stdout != "AB\u00e9D" || r.stderr != "E" || r.code != 5 {
		t.Errorf("stdout %q, stderr %q, exit code %d", r.stdout, r.stderr, r.code)
	}
}

// AH=0Ah ends the line at CR and echoes to standard output; AH=08h goes on
// with the rest (here the LF of the CR LF) and returns ^Z at the end.
func TestPipeLineInput(t *testing.T) {
	r := runPipe(t, 437, progLine, strings.NewReader("abc\nxyz"))
	if want := "abc\rabc\r\nxyz"; r.stdout != want || r.code != 0 {
		t.Errorf("stdout %q (exit %d), want %q", r.stdout, r.code, want)
	}
}

// Input that arrives late: the program waits for it (the machine is not
// blocked) and gets the data and then the end of the input.
func TestPipeLateInput(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		time.Sleep(300 * time.Millisecond)
		io.WriteString(pw, "late\n")
		time.Sleep(100 * time.Millisecond)
		io.WriteString(pw, "later")
		pw.Close()
	}()
	start := time.Now()
	r := runPipe(t, 437, progEcho, pr)
	if r.stdout != "late\r\nlater" || r.code != 0 {
		t.Errorf("stdout %q (exit %d)", r.stdout, r.code)
	}
	if d := time.Since(start); d < 300*time.Millisecond {
		t.Errorf("finished after %v, before the input was written", d)
	}
}
