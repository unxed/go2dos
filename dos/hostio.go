package dos

import (
	"io"
	"sync"
	"unicode/utf8"

	"github.com/unxed/go2dos/cp"
)

// Pipe mode (docs/SCREEN.md, S1): standard input, output and error of the
// DOS program (handles 0, 1, 2) are the host's streams. The host side is
// UTF-8, the DOS side is bytes of the OEM code page; the conversion is done
// here, one character at a time, so a character split across two reads of
// the host stream is handled.
//
// Input: bytes that have no mapping in the code page, and invalid UTF-8,
// become '?'. A bare LF becomes CR LF (CR LF stays), because DOS text and
// the line input of INT 21h/0Ah end a line with CR. Output is converted
// byte by byte (CR LF is passed unchanged).

const hostOutFlushAt = 4096

type hostIO struct {
	page *cp.Codepage
	in   io.Reader
	out  io.Writer
	errw io.Writer

	mu      sync.Mutex
	inBuf   []byte // DOS bytes ready to be read
	inEOF   bool   // the host stream has ended
	started bool

	// Used by the reader goroutine only.
	pend   []byte // an incomplete UTF-8 sequence
	lastCR bool

	// Used by the machine goroutine only.
	outBuf []byte // UTF-8 not yet written to out
}

func newHostIO(page *cp.Codepage, in io.Reader, out, errw io.Writer) *hostIO {
	return &hostIO{page: page, in: in, out: out, errw: errw}
}

// start launches the reader on first use. Reading happens in a goroutine so
// that a program waiting for input does not stop the machine (timers, the
// timeout, Ctrl-C).
func (h *hostIO) start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return
	}
	h.started = true
	if h.in == nil {
		h.inEOF = true
		return
	}
	go h.pump()
}

func (h *hostIO) pump() {
	raw := make([]byte, 4096)
	for {
		n, err := h.in.Read(raw)
		if n > 0 {
			h.feed(raw[:n])
		}
		if err != nil {
			h.finish()
			return
		}
	}
}

// feed converts host bytes to DOS bytes and queues them.
func (h *hostIO) feed(b []byte) {
	h.pend = append(h.pend, b...)
	var out []byte
	for len(h.pend) > 0 && utf8.FullRune(h.pend) {
		r, size := utf8.DecodeRune(h.pend)
		h.pend = h.pend[size:]
		c := byte('?')
		if r != utf8.RuneError || size != 1 {
			if v, ok := h.page.Byte(r); ok {
				c = v
			}
		}
		if c == '\n' && !h.lastCR {
			out = append(out, '\r')
		}
		h.lastCR = c == '\r'
		out = append(out, c)
	}
	h.mu.Lock()
	h.inBuf = append(h.inBuf, out...)
	h.mu.Unlock()
}

// finish ends the input: a sequence cut short by the end becomes '?'.
func (h *hostIO) finish() {
	h.mu.Lock()
	for range h.pend {
		h.inBuf = append(h.inBuf, '?')
	}
	h.pend = nil
	h.inEOF = true
	h.mu.Unlock()
}

// read takes up to len(buf) DOS bytes. With nothing available it returns
// n=0 and wait=true while the host stream is still open (the caller idles and
// retries), or n=0 and wait=false at the end of the input.
func (h *hostIO) read(buf []byte) (n int, wait bool) {
	h.start()
	if len(buf) == 0 {
		return 0, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.inBuf) == 0 {
		return 0, !h.inEOF
	}
	n = copy(buf, h.inBuf)
	h.inBuf = h.inBuf[n:]
	return n, false
}

// available reports whether a byte can be read now.
func (h *hostIO) available() bool {
	h.start()
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.inBuf) > 0
}

// waiting reports that nothing is queued but the input has not ended.
func (h *hostIO) waiting() bool {
	h.start()
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.inBuf) == 0 && !h.inEOF
}

// atEOF reports that the input has ended and everything was read.
func (h *hostIO) atEOF() bool {
	h.start()
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.inBuf) == 0 && h.inEOF
}

// writeOut queues DOS bytes for the host's standard output as UTF-8.
func (h *hostIO) writeOut(b []byte) {
	for _, c := range b {
		h.outBuf = utf8.AppendRune(h.outBuf, h.page.Rune(c))
	}
	if len(h.outBuf) >= hostOutFlushAt {
		h.flush()
	}
}

// writeErr writes DOS bytes to the host's standard error at once (after the
// pending standard output, so that both keep their order on a terminal).
func (h *hostIO) writeErr(b []byte) {
	h.flush()
	if h.errw == nil {
		return
	}
	var s []byte
	for _, c := range b {
		s = utf8.AppendRune(s, h.page.Rune(c))
	}
	h.errw.Write(s)
}

func (h *hostIO) flush() {
	if len(h.outBuf) == 0 {
		return
	}
	if h.out != nil {
		h.out.Write(h.outBuf)
	}
	h.outBuf = h.outBuf[:0]
}

// PipeMode reports whether the program's standard streams are the host's.
func (d *DOS) PipeMode() bool { return d.host != nil }

// FlushHost writes the pending standard output to the host (pipe mode); the
// machine calls it after every slice of execution and when it stops.
func (d *DOS) FlushHost() {
	if d.host != nil {
		d.host.flush()
	}
}

// HostTTY receives the characters written through the BIOS/console teletype
// in pipe mode (INT 29h, INT 10h/0Eh, CON): they go to standard output.
func (d *DOS) HostTTY(ch byte) {
	if d.host != nil {
		d.host.writeOut([]byte{ch})
	}
}

// hostStdin reports that handle 0 of the current process is the host's
// standard input (not redirected to a file by the shell).
func (d *DOS) hostStdin() bool {
	if d.host == nil {
		return false
	}
	of, errc := d.handle(0)
	return errc == 0 && of.dev == devHostIn
}

// hostChar is readChar for host input. At the end of the input it returns ^Z:
// DEV.ASM (IOFUNC): "Input ... Returns ^Z on EOF".
func (d *DOS) hostChar() (byte, bool) {
	var b [1]byte
	n, wait := d.host.read(b[:])
	if n == 1 {
		return b[0], true
	}
	if wait {
		return 0, false
	}
	return 0x1A, true
}
