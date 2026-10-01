package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"

	"golang.org/x/term"
)

// termHost is the plain ANSI terminal front end (frontend.Host and
// frontend.Console).
type termHost struct {
	rend      *renderer
	sel       *selectionHandler
	m         *machine.Machine
	clipboard frontend.Clipboard
}

func newTermHost(w io.Writer) *termHost {
	return &termHost{
		rend:      newRenderer(w),
		sel:       newSelectionHandler(),
		clipboard: &frontend.HostClipboard{},
	}
}

func (h *termHost) Draw(s *bios.Screen) {
	h.rend.draw(s)
	h.sel.updateScreen(s)
}
func (h *termHost) Stream(b []byte, page *cp.Codepage) { h.rend.stream(b, page) }
func (h *termHost) Display(grid bool)                  { h.rend.display(grid) }

func (h *termHost) Start(m *machine.Machine, display string, stop func(dump bool)) (func(), error) {
	h.m = m
	h.sel.updateScreen(m.Screen())
	restore, err := setupTerminal(display != "console")
	if err != nil {
		return nil, err
	}
	p := &inputParser{
		page: m.CP,
		push: func(ke bios.KeyEvent) {
			// Handle terminal integration (Shift+Arrow, Ctrl-V)
			if !h.handleTerminalIntegration(ke) {
				m.PushKey(ke)
			}
		},
		cmd: func(c termCmd) { stop(c == cmdDump) },
	}
	go p.run(os.Stdin)
	return restore, nil
}

// handleTerminalIntegration processes terminal integration keys (Shift+Arrow for selection, Ctrl-V for paste).
// Returns true if the key was handled, false if it should be passed to the machine.
func (h *termHost) handleTerminalIntegration(ke bios.KeyEvent) bool {
	scr := h.m.Screen()

	// Ctrl-V for pasting from clipboard
	if ke.Scan == 0x2F && ke.Mods&bios.ModCtrl != 0 { // V with Ctrl
		h.pasteClipboard()
		return true
	}

	// Shift+Arrow keys for text selection
	switch ke.Scan {
	case 0x48: // Up arrow
		if ke.Mods&bios.ModLShift != 0 {
			if !h.sel.isActive() {
				h.sel.toggleSelectionMode()
				h.sel.startSelection(scr.CursorX, scr.CursorY)
			}
			h.sel.extendSelection(scr.CursorX, scr.CursorY-1)
			return true
		}
	case 0x50: // Down arrow
		if ke.Mods&bios.ModLShift != 0 {
			if !h.sel.isActive() {
				h.sel.toggleSelectionMode()
				h.sel.startSelection(scr.CursorX, scr.CursorY)
			}
			h.sel.extendSelection(scr.CursorX, scr.CursorY+1)
			return true
		}
	case 0x4B: // Left arrow
		if ke.Mods&bios.ModLShift != 0 {
			if !h.sel.isActive() {
				h.sel.toggleSelectionMode()
				h.sel.startSelection(scr.CursorX, scr.CursorY)
			}
			h.sel.extendSelection(scr.CursorX-1, scr.CursorY)
			return true
		}
	case 0x4D: // Right arrow
		if ke.Mods&bios.ModLShift != 0 {
			if !h.sel.isActive() {
				h.sel.toggleSelectionMode()
				h.sel.startSelection(scr.CursorX, scr.CursorY)
			}
			h.sel.extendSelection(scr.CursorX+1, scr.CursorY)
			return true
		}
	}

	// ESC to end selection
	if ke.Scan == 0x01 && ke.Mods == 0 { // ESC
		if h.sel.isActive() {
			h.sel.endSelection()
			return true
		}
	}

	return false
}

// pasteClipboard reads text from the host clipboard and converts it to keystrokes,
// sending them to the machine in portions (max 15 keystrokes at a time to fit in the BIOS buffer).
func (h *termHost) pasteClipboard() {
	text, err := h.clipboard.GetText()
	if err != nil {
		// Silently fail if clipboard is not available
		return
	}

	// Convert text to keystrokes
	const maxPortion = 15 // BIOS keyboard buffer holds 15 keystrokes
	var pending []bios.KeyEvent

	for _, r := range text {
		if r == '\n' {
			// Convert newline to Enter key
			if k, ok := keys.Named("Enter", 0); ok {
				pending = append(pending, k)
			}
		} else if r < 0x80 {
			// ASCII character
			if k, ok := keys.Char(r, h.m.CP); ok {
				pending = append(pending, k)
			}
		}

		// Send in portions to avoid filling the BIOS buffer
		if len(pending) >= maxPortion {
			for _, ke := range pending {
				h.m.PushKey(ke)
			}
			pending = pending[:0]
			// Small delay to allow buffer processing
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Send remaining keystrokes
	for _, ke := range pending {
		h.m.PushKey(ke)
	}
}

func setupTerminal(alt bool) (func(), error) {
	fd := int(os.Stdin.Fd())
	st, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	undoVT := enableVT()
	if alt {
		fmt.Print("\x1b[?1049h\x1b[2J")
	}
	return func() {
		if alt {
			fmt.Print("\x1b[0m\x1b[?25h\x1b[?1049l")
		}
		undoVT()
		term.Restore(fd, st)
	}, nil
}

// renderer draws screen snapshots on an ANSI terminal, sending only the
// cells that changed since the previous frame.
type renderer struct {
	mu   sync.Mutex
	w    *bufio.Writer
	prev *bios.Screen
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
	}
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
			if !full && r.prev.Cells[i] == c {
				continue
			}
			if cx != x || cy != y {
				r.w.WriteString("\x1b[" + strconv.Itoa(y+1) + ";" + strconv.Itoa(x+1) + "H")
			}
			if int(c.Attr) != lastAttr {
				r.w.WriteString(machine.SGR(c.Attr))
				lastAttr = int(c.Attr)
			}
			r.w.WriteRune(c.Rune)
			cx, cy = x+1, y
		}
	}
	if s.CursorVisible {
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
)

// inputParser turns terminal input bytes into keystrokes.
type inputParser struct {
	page *cp.Codepage
	push func(bios.KeyEvent)
	cmd  func(termCmd)
}

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
				case hotkey:
					p.push(keys.Ctrl(']'))
				}
				pending = pending[1:]
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
