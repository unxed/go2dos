package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/frontend"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"

	"golang.org/x/term"
)

// termHost is the plain ANSI terminal front end (frontend.Host,
// frontend.Console and frontend.Attacher).
type termHost struct {
	rend *renderer
	in   *stdinReader
	clip frontend.MemoryClipboard // what the DOS clipboard server (INT 2Fh/17xx) sees
	sel  *selection               // selection mode (input goroutine only), nil: off

	fd       int
	st       *term.State
	undoVT   func()
	altOn    bool // the alternate screen is shown
	attached atomic.Bool
	until    atomic.Int64 // UnixNano: SIGINT sent during the command may still be delivered shortly after it ends
}

// sigGrace — сколько после конца команды SIGINT ещё считается её сигналом.
const sigGrace = 200 * time.Millisecond

func newTermHost(w io.Writer) *termHost {
	return &termHost{rend: newRenderer(w), in: newStdinReader(os.Stdin)}
}

func (h *termHost) Draw(s *bios.Screen)                { h.rend.draw(s) }
func (h *termHost) Stream(b []byte, page *cp.Codepage) { h.rend.stream(b, page) }

func (h *termHost) Display(grid bool) {
	h.rend.display(grid)
	h.altOn = grid
}

func (h *termHost) Start(m *machine.Machine, display string, stop func(dump bool)) (func(), error) {
	restore, err := h.setup(display != "console")
	if err != nil {
		return nil, err
	}
	push := func(k bios.KeyEvent) {
		if h.sel != nil {
			if h.rend.overlayOn() {
				h.selectKey(m, k)
				return
			}
			h.sel = nil // the grid went away (console mode)
		}
		m.PushKey(k)
	}
	p := &inputParser{page: m.CP, push: push, paste: m.PasteText, cmd: func(c termCmd) {
		switch c {
		case cmdSelect:
			h.startSelect(m)
		case cmdCopy:
			h.copyScreen(m)
		case cmdPaste:
			go h.pasteClipboard(m) // may block while the key queue is full
		default:
			stop(c == cmdDump)
		}
	}}
	go p.run(h.in)
	return restore, nil
}

// Clipboard is the clipboard that the DOS programs see (frontend.ClipboardProvider).
func (h *termHost) Clipboard() frontend.Clipboard { return &h.clip }

// copyScreen puts the text of the screen on the clipboards: ours and the terminal's.
func (h *termHost) copyScreen(m *machine.Machine) {
	text := m.Screen().Text()
	h.clip.SetText(text)
	h.rend.copyToTerminal(text)
}

// startSelect turns the selection mode on (the grid display only).
func (h *termHost) startSelect(m *machine.Machine) {
	s := m.Screen()
	if !s.TextMode() {
		return
	}
	h.sel = newSelection(s)
	if !h.rend.setSelection(h.sel) {
		h.sel = nil
	}
}

// selectKey gives a key press to the selection mode; a finished selection
// goes to the clipboards, like copyScreen's text.
func (h *termHost) selectKey(m *machine.Machine, k bios.KeyEvent) {
	switch h.sel.key(k) {
	case selCopy:
		x0, y0, x1, y1 := h.sel.rect()
		h.sel = nil
		h.rend.setSelection(nil)
		text := m.Screen().Selection(x0, y0, x1, y1)
		h.clip.SetText(text)
		h.rend.copyToTerminal(text)
	case selCancel:
		h.sel = nil
		h.rend.setSelection(nil)
	default:
		h.rend.setSelection(h.sel)
	}
}

// pasteClipboard types our clipboard into the program.
func (h *termHost) pasteClipboard(m *machine.Machine) {
	if text, err := h.clip.GetText(); err == nil {
		m.PasteText(text)
	}
}

// setup puts the terminal in raw mode (and on the alternate screen with alt).
func (h *termHost) setup(alt bool) (func(), error) {
	h.fd = int(os.Stdin.Fd())
	st, err := term.MakeRaw(h.fd)
	if err != nil {
		return nil, err
	}
	h.st = st
	h.undoVT = enableVT()
	os.Stdout.WriteString("\x1b[?2004h") // bracketed paste
	if alt {
		h.Display(true)
	}
	return func() {
		if h.altOn {
			h.Display(false)
		}
		os.Stdout.WriteString("\x1b[?2004l")
		h.undoVT()
		term.Restore(h.fd, h.st)
	}, nil
}

// Attached reports whether a host command owns the terminal (or ended a
// moment ago: Go delivers a signal to its handler asynchronously, and a
// Ctrl-C meant for a short command must not stop the emulator).
func (h *termHost) Attached() bool {
	return h.attached.Load() || time.Now().UnixNano() < h.until.Load()
}

// RunAttached gives the terminal to cmd: the input reader is paused (so it
// cannot take the command's first keystroke), the alternate screen is left,
// cooked mode is restored, cmd runs on the real terminal; afterwards raw mode,
// the alternate screen and the reader come back. Call it from the machine
// goroutine, so that no frame is drawn meanwhile.
func (h *termHost) RunAttached(cmd *exec.Cmd) error {
	h.attached.Store(true)
	defer func() {
		h.until.Store(time.Now().Add(sigGrace).UnixNano())
		h.attached.Store(false)
	}()
	h.in.pause()
	defer h.in.resume()
	grid := h.altOn
	if grid {
		h.rend.display(false)
	}
	if err := term.Restore(h.fd, h.st); err != nil {
		return err
	}
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	runErr := cmd.Run()
	st, err := term.MakeRaw(h.fd)
	if err != nil {
		return errors.Join(runErr, err)
	}
	h.st = st
	if grid {
		h.rend.display(true)
	}
	return runErr
}

// renderer draws screen snapshots on an ANSI terminal, sending only the
// cells that changed since the previous frame.
type renderer struct {
	mu   sync.Mutex
	w    *bufio.Writer
	prev *bios.Screen
	sel  *selection // selection overlay (a copy), nil: none
}

func newRenderer(w io.Writer) *renderer { return &renderer{w: bufio.NewWriterSize(w, 64<<10)} }

// display switches between the terminal's normal buffer (console stream)
// and the alternate screen (grid).
func (r *renderer) display(grid bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if grid {
		r.w.WriteString("\x1b[?1049h\x1b[2J")
		r.prev = nil
	} else {
		r.w.WriteString("\x1b[0m\x1b[?25h\x1b[?1049l")
		r.prev, r.sel = nil, nil
	}
	r.w.Flush()
}

// copyToTerminal asks the terminal to put text on its clipboard (OSC 52).
func (r *renderer) copyToTerminal(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.w.WriteString("\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a")
	r.w.Flush()
}

// stream writes teletype output to the normal buffer: CR, LF, BS and BEL
// stay control characters, every other byte is the glyph the screen shows.
func (r *renderer) stream(b []byte, page *cp.Codepage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range b {
		switch c {
		case 7, 8, 10, 13:
			r.w.WriteByte(c)
		default:
			r.w.WriteRune(page.ScreenRune(c))
		}
	}
	r.w.Flush()
}

func (r *renderer) draw(s *bios.Screen) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drawLocked(s, false)
}

// setSelection shows the selection overlay (sl is copied), or removes it with
// nil, and repaints the last frame. It reports false, doing nothing, if no
// grid frame is on the terminal (the console mode).
func (r *renderer) setSelection(sl *selection) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.prev == nil {
		return false
	}
	r.sel = nil
	if sl != nil {
		c := *sl
		r.sel = &c
	}
	r.drawLocked(r.prev, true)
	return true
}

// overlayOn reports whether the selection overlay is shown.
func (r *renderer) overlayOn() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sel != nil
}

// drawLocked paints s; with force every cell, not only the changed ones.
func (r *renderer) drawLocked(s *bios.Screen, force bool) {
	if !s.TextMode() {
		return
	}
	full := r.prev == nil || r.prev.Cols != s.Cols || r.prev.Rows != s.Rows
	if full {
		r.w.WriteString("\x1b[0m\x1b[2J")
	}
	lastAttr := -1
	cx, cy := -1, -1
	for y := 0; y < s.Rows; y++ {
		for x := 0; x < s.Cols; x++ {
			i := y*s.Cols + x
			c := s.Cells[i]
			if !full && !force && r.prev.Cells[i] == c {
				continue
			}
			if cx != x || cy != y {
				r.w.WriteString("\x1b[" + strconv.Itoa(y+1) + ";" + strconv.Itoa(x+1) + "H")
			}
			attr := r.sel.attrAt(x, y, c.Attr)
			if int(attr) != lastAttr {
				r.w.WriteString(machine.SGR(attr))
				lastAttr = int(attr)
			}
			r.w.WriteRune(c.Rune)
			cx, cy = x+1, y
		}
	}
	if s.CursorVisible && r.sel == nil {
		r.w.WriteString("\x1b[" + strconv.Itoa(s.CursorY+1) + ";" + strconv.Itoa(s.CursorX+1) + "H\x1b[?25h")
	} else {
		r.w.WriteString("\x1b[?25l")
	}
	r.w.Flush()
	r.prev = s
}

// escape-key commands of the terminal frontend
const hotkey = 0x1D // Ctrl-]

type termCmd int

const (
	cmdNone termCmd = iota
	cmdQuit
	cmdDump
	cmdCopy   // Ctrl-] c: the screen text to the clipboard
	cmdPaste  // Ctrl-] v: the clipboard as keystrokes
	cmdSelect // Ctrl-] s: choose a screen area with the cursor keys and copy it
)

// inputParser turns terminal input bytes into keystrokes.
type inputParser struct {
	page *cp.Codepage
	push func(bios.KeyEvent)
	cmd  func(termCmd)
	// paste receives text that the terminal pasted in bracketed-paste mode
	// (ESC [ 200 ~ ... ESC [ 201 ~); nil: the markers are ignored.
	paste func(string)
}

const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

var csiFinal = map[byte]string{'A': "Up", 'B': "Down", 'C': "Right", 'D': "Left", 'H': "Home", 'F': "End",
	'P': "F1", 'Q': "F2", 'R': "F3", 'S': "F4"}
var csiTilde = map[int]string{1: "Home", 2: "Ins", 3: "Del", 4: "End", 5: "PgUp", 6: "PgDn", 7: "Home", 8: "End",
	11: "F1", 12: "F2", 13: "F3", 14: "F4", 15: "F5", 17: "F6", 18: "F7", 19: "F8", 20: "F9", 21: "F10", 23: "F11", 24: "F12"}

func xtermMods(n int) byte {
	n--
	var m byte
	if n&1 != 0 {
		m |= bios.ModLShift
	}
	if n&2 != 0 {
		m |= bios.ModAlt
	}
	if n&4 != 0 {
		m |= bios.ModCtrl
	}
	return m
}

func (p *inputParser) named(name string, mods byte) {
	if k, ok := keys.Named(name, mods); ok {
		p.push(k)
	}
}

// run reads from r until it fails. Lone ESC is recognized by a short pause.
func (p *inputParser) run(r io.Reader) {
	ch := make(chan []byte, 16)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				ch <- append([]byte(nil), buf[:n]...)
			}
			if err != nil {
				close(ch)
				return
			}
		}
	}()
	var pending []byte
	hot := false
	for {
		var data []byte
		var ok bool
		if len(pending) > 0 {
			select {
			case data, ok = <-ch:
			case <-time.After(50 * time.Millisecond):
				data, ok = nil, true
			}
		} else {
			data, ok = <-ch
		}
		if !ok {
			return
		}
		pending = append(pending, data...)
		flush := data == nil
		for len(pending) > 0 {
			if hot {
				hot = false
				switch pending[0] {
				case 'q', 'Q':
					p.cmd(cmdQuit)
				case 'd', 'D':
					p.cmd(cmdDump)
				case 'c', 'C':
					p.cmd(cmdCopy)
				case 'v', 'V':
					p.cmd(cmdPaste)
				case 's', 'S':
					p.cmd(cmdSelect)
				case hotkey:
					p.push(keys.Ctrl(']'))
				}
				pending = pending[1:]
				continue
			}
			if p.paste != nil && bytes.HasPrefix(pending, []byte(pasteStart)) {
				end := bytes.Index(pending, []byte(pasteEnd))
				if end < 0 {
					if !flush {
						break // the rest of the paste has not arrived
					}
					end = len(pending)
				}
				p.paste(string(pending[len(pasteStart):end]))
				pending = pending[min(end+len(pasteEnd), len(pending)):]
				continue
			}
			n := p.parse(pending, flush)
			if n == 0 {
				break
			}
			if pending[0] == hotkey {
				hot = true
			}
			pending = pending[n:]
		}
	}
}

// parse consumes one key from b and returns the bytes used (0 = need more).
func (p *inputParser) parse(b []byte, flush bool) int {
	c := b[0]
	switch {
	case c == 0x1B:
		if len(b) == 1 {
			if flush {
				p.named("Esc", 0)
				return 1
			}
			return 0
		}
		if b[1] == '[' || b[1] == 'O' {
			end := 2
			for end < len(b) && (b[end] < 0x40 || b[end] > 0x7E) {
				end++
			}
			if end >= len(b) {
				if flush {
					p.named("Esc", 0)
					return 1
				}
				return 0
			}
			p.csi(string(b[2:end]), b[end])
			return end + 1
		}
		// ESC + key: Alt+key
		r, size := utf8.DecodeRune(b[1:])
		if r < 0x80 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '=') {
			p.push(keys.Alt(byte(r)))
		} else {
			p.named("Esc", 0)
			return 1
		}
		return 1 + size
	case c == hotkey:
		return 1
	case c == 0x7F || c == 0x08:
		p.named("Backspace", 0)
	case c == 0x0D:
		p.named("Enter", 0)
	case c == 0x0A:
		p.named("Enter", bios.ModCtrl)
	case c == 0x09:
		p.named("Tab", 0)
	case c == 0x00:
		p.push(bios.KeyEvent{Scan: 0x03, Mods: bios.ModCtrl}) // Ctrl-2 / Ctrl-@
	case c < 0x20:
		p.push(keys.Ctrl('a' + c - 1))
	default:
		if !utf8.FullRune(b) && !flush {
			return 0
		}
		r, size := utf8.DecodeRune(b)
		if k, ok := keys.Char(r, p.page); ok {
			p.push(k)
		}
		return size
	}
	return 1
}

func (p *inputParser) csi(params string, final byte) {
	parts := strings.Split(params, ";")
	mods := byte(0)
	if len(parts) == 2 {
		if n, err := strconv.Atoi(parts[1]); err == nil {
			mods = xtermMods(n)
		}
	}
	switch {
	case final == '~':
		n, _ := strconv.Atoi(parts[0])
		if name, ok := csiTilde[n]; ok {
			p.named(name, mods)
		}
	case final == 'Z':
		p.named("Tab", bios.ModLShift)
	default:
		if name, ok := csiFinal[final]; ok {
			p.named(name, mods)
		}
	}
}
