package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/unxed/go2dos/frontend"
)

// sysClip is the clipboard that DOS programs see in the terminal front end
// (WinOldAp, INT 2Fh/17xx), tied to the system clipboard:
//
//	write: a host tool (wl-copy, xclip, xsel, pbcopy) if there is one, else OSC 52;
//	read:  the same tool; without one, what we last held (our own copies and
//	       whatever the terminal pasted, see remember).
//
// Mode (-clip-sync): auto (tool, then OSC 52), tool, osc52, off (memory only).
type sysClip struct {
	mem   frontend.MemoryClipboard
	mode  string
	osc   func(text string) // OSC 52 write to the terminal; nil: none
	rd    []string          // read command (stdout = text), nil: none
	wr    []string          // write command (stdin = text), nil: none
	read  func(args []string) (string, error)
	write func(args []string, text string) error
	mu    sync.Mutex
}

const clipToolTimeout = time.Second

// clipTool is a pair of commands for one clipboard system.
type clipTool struct {
	need     string // the environment variable that says the system is reachable ("" = none)
	rd, wr   []string
	osFilter string // GOOS, "" = any
}

var clipTools = []clipTool{
	{"WAYLAND_DISPLAY", []string{"wl-paste", "-n"}, []string{"wl-copy"}, ""},
	{"DISPLAY", []string{"xclip", "-selection", "clipboard", "-o"}, []string{"xclip", "-selection", "clipboard", "-i"}, ""},
	{"DISPLAY", []string{"xsel", "-b", "-o"}, []string{"xsel", "-b", "-i"}, ""},
	{"", []string{"pbpaste"}, []string{"pbcopy"}, "darwin"},
}

// findClipTool picks the first tool whose both programs are installed.
func findClipTool(getenv func(string) string, look func(string) (string, error), goos string) (rd, wr []string) {
	for _, t := range clipTools {
		if t.osFilter != "" && t.osFilter != goos {
			continue
		}
		if t.need != "" && getenv(t.need) == "" {
			continue
		}
		if _, err := look(t.rd[0]); err != nil {
			continue
		}
		if _, err := look(t.wr[0]); err != nil {
			continue
		}
		return t.rd, t.wr
	}
	return nil, nil
}

func newSysClip(mode string, osc func(string)) *sysClip {
	c := &sysClip{mode: mode, osc: osc, read: readClipTool, write: writeClipTool}
	if mode == "auto" || mode == "tool" {
		c.rd, c.wr = findClipTool(os.Getenv, exec.LookPath, runtime.GOOS)
	}
	return c
}

// readClipTool runs a clipboard reader and returns its stdout.
func readClipTool(args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), clipToolTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	return string(out), err
}

// writeClipTool feeds text to a clipboard writer. Its stdout and stderr stay
// files (/dev/null), not pipes: xclip -i leaves a daemon that keeps them open.
func writeClipTool(args []string, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), clipToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (c *sysClip) GetText() (string, error) {
	if c.rd != nil {
		if s, err := c.read(c.rd); err == nil {
			c.mem.SetText(s)
			return s, nil
		}
	}
	return c.mem.GetText()
}

func (c *sysClip) SetText(s string) error {
	c.mem.SetText(s)
	if c.mode == "off" {
		return nil
	}
	if c.wr != nil {
		if err := c.write(c.wr, s); err == nil {
			return nil
		}
	}
	if c.mode != "tool" && c.osc != nil {
		c.osc(s)
	}
	return nil
}

// remember keeps text that the terminal pasted (the system clipboard
// content) so that Shift-Ins in the program gives the same text.
func (c *sysClip) remember(s string) {
	if c.mode != "off" {
		c.mem.SetText(s)
	}
}
