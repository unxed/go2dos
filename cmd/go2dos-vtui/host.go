package main

import (
	"errors"
	"os"
	"sync"

	"golang.org/x/term"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/machine"
	"github.com/unxed/vtinput"
	"github.com/unxed/vtui"
)

// host is the vtui front end (frontend.Host): the screen is a vtui.ScreenBuf,
// keys are vtinput events.
type host struct {
	mu  sync.Mutex
	scr *vtui.ScreenBuf
}

func newHost(scr *vtui.ScreenBuf) *host { return &host{scr: scr} }

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
	undo, err := vtinput.Enable()
	if err != nil {
		return nil, err
	}
	os.Stdout.WriteString("\x1b[?1049h\x1b[2J")
	go h.readKeys(vtinput.NewReader(os.Stdin, false), m, stop)
	return func() {
		os.Stdout.WriteString("\x1b[0m\x1b[?25h\x1b[?1049l")
		undo()
	}, nil
}

// readKeys reads vtinput events until the input ends.
func (h *host) readKeys(r *vtinput.Reader, m *machine.Machine, stop func(dump bool)) {
	var hot hotkey
	for ev := range r.GetEventChan() {
		switch hot.feed(ev) {
		case hotQuit:
			stop(false)
		case hotDump:
			stop(true)
		}
	}
}
