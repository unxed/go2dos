package main

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
	"github.com/unxed/vtinput"
	"github.com/unxed/vtui"
)

// sigGrace — сколько после конца команды хоста SIGINT ещё считается её
// сигналом (Go доставляет сигнал обработчику асинхронно).
const sigGrace = 200 * time.Millisecond

// host is the vtui front end (frontend.Host and frontend.Attacher): the screen
// is a vtui.ScreenBuf, keys are vtinput events.
type host struct {
	mu  sync.Mutex // Draw и RunAttached не пересекаются
	scr *vtui.ScreenBuf

	// протоколы терминала, которые включает vtinput (в тесте — ни одного)
	protocols vtinput.Protocol

	push func(bios.KeyEvent)
	page *cp.Codepage
	stop func(dump bool)

	undo     func() // вернуть терминалу обычный режим
	reader   *vtinput.Reader
	pumpDone chan struct{}

	attached atomic.Bool
	until    atomic.Int64 // UnixNano, см. sigGrace
}

func newHost(scr *vtui.ScreenBuf) *host {
	return &host{scr: scr, protocols: vtinput.DefaultProtocols}
}

// Draw gets the screen snapshot from the machine goroutine.
func (h *host) Draw(s *bios.Screen) {
	h.mu.Lock()
	defer h.mu.Unlock()
	renderScreen(h.scr, s)
}

func (h *host) Start(m *machine.Machine, display string, stop func(dump bool)) (func(), error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("stdin is not a terminal")
	}
	h.push, h.page, h.stop = m.PushKey, m.CP, stop
	if err := h.startInput(); err != nil {
		return nil, err
	}
	os.Stdout.WriteString("\x1b[?1049h\x1b[2J")
	return func() {
		os.Stdout.WriteString("\x1b[0m\x1b[?25h\x1b[?1049l")
		h.stopInput()
	}, nil
}

// startInput puts the terminal in raw mode with the vtinput protocols and
// starts turning events into keystrokes.
func (h *host) startInput() error {
	undo, err := vtinput.EnableProtocols(h.protocols)
	if err != nil {
		return err
	}
	h.undo = undo
	h.reader = vtinput.NewReader(os.Stdin, false)
	events := h.reader.GetEventChan()
	done := make(chan struct{})
	h.pumpDone = done
	go func() {
		defer close(done)
		pumpEvents(events, h.push, h.page, h.stop)
	}()
	return nil
}

// stopInput stops reading (Reader.Close wakes the blocked read through its
// stop pipe, nothing is taken from stdin) and restores the terminal.
func (h *host) stopInput() {
	h.reader.Close()
	select {
	case <-h.pumpDone:
	case <-time.After(time.Second):
	}
	h.undo()
}

// Attached reports whether a host command owns the terminal (or ended a moment
// ago, see sigGrace).
func (h *host) Attached() bool {
	return h.attached.Load() || time.Now().UnixNano() < h.until.Load()
}

// RunAttached gives the terminal to cmd (docs/HOSTEXEC.md): key reading is
// stopped, the alternate screen is left, cmd runs on the real terminal, then
// the protocols, the alternate screen and the whole frame come back. The
// thin host does not use vtui.Suspend/Resume: they work on the state of
// vtui.FrameManager, which this front end does not start.
func (h *host) RunAttached(cmd *exec.Cmd) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.attached.Store(true)
	defer func() {
		h.until.Store(time.Now().Add(sigGrace).UnixNano())
		h.attached.Store(false)
	}()
	h.stopInput()
	os.Stdout.WriteString("\x1b[0m\x1b[?25h\x1b[?1049l")
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
	if err := h.startInput(); err != nil {
		return errors.Join(runErr, err)
	}
	os.Stdout.WriteString("\x1b[?1049h\x1b[2J")
	h.scr.HardReset()
	h.scr.Flush()
	return runErr
}

// pumpEvents turns vtinput events into keystrokes until the channel closes.
// push may block while the machine's key queue is full.
func pumpEvents(events <-chan *vtinput.InputEvent, push func(bios.KeyEvent), page *cp.Codepage, stop func(dump bool)) {
	var hot hotkey
	for ev := range events {
		switch hot.feed(ev) {
		case hotNone:
			for _, k := range toKeys(ev, page) {
				push(k)
			}
		case hotPass:
			push(keys.Ctrl(']'))
		case hotQuit:
			stop(false)
		case hotDump:
			stop(true)
		}
	}
}
