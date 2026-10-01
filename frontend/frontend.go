// Package frontend is the code shared by the go2dos front ends (the plain
// terminal one in cmd/go2dos and the vtui one in cmd/go2dos-vtui): command
// line options, machine setup, the run loop, exit codes, dumps. A front end
// supplies only a Host: how the screen is drawn and where keys come from.
package frontend

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/dos"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
)

// Host is what a front end provides to Run.
type Host interface {
	// Draw is called from the machine goroutine whenever the screen changes
	// (Config.OnScreen); the snapshot must not be modified.
	Draw(*bios.Screen)
	// Start is called once the program is loaded, before it runs. display is
	// "console" or "grid" (see machine.Config.Display). The host takes over
	// the terminal/window and starts delivering keys with m.PushKey. stop
	// ends the session; with dump set, a diagnostic dump is written. The
	// returned function restores the terminal; Run calls it after the
	// machine stops.
	Start(m *machine.Machine, display string, stop func(dump bool)) (restore func(), err error)
}

// Attacher is implemented by hosts that can hand the terminal to a host
// command (docs/HOSTEXEC.md, the overlay mode): RunAttached pauses key input,
// restores the terminal's normal state, runs cmd on it and takes the terminal
// back; Attached reports whether cmd is running now (Run's SIGINT handler then
// leaves Ctrl-C to the command). Call RunAttached from the machine goroutine.
type Attacher interface {
	RunAttached(cmd *exec.Cmd) error
	Attached() bool
}

// Console is implemented by hosts that support the "console" display mode:
// teletype output goes to the host's normal buffer, the grid is shown only
// while a program draws on the screen directly (docs/SCREEN.md).
type Console interface {
	// Stream writes teletype output (bytes of the DOS code page).
	Stream(b []byte, page *cp.Codepage)
	// Display switches between the normal buffer (false) and the grid (true).
	Display(grid bool)
}

// ClipboardProvider is implemented by hosts that have a clipboard: Run gives
// it to the machine, so that DOS programs reach it through WinOldAp
// (INT 2Fh/17xx).
type ClipboardProvider interface {
	Clipboard() Clipboard
}

// Options are the flags common to all front ends.
type Options struct {
	Drives      map[byte]string
	Confine     bool
	ReadOnly    string // -ro: drives DOS may not change ("C", "CD", "all")
	Codepage    int
	Trace       string
	TraceFilter string
	Keys        string
	Headless    bool
	Timeout     time.Duration
	DumpDir     string
	DumpOnExit  bool
	ScreenOut   string
	Record      string
	Lenient     bool
	NoLFN       bool
	Watch       string
	Break       string
	Display     string
	Size        string
	HostExec    bool
	// Pipe is pipe mode (docs/SCREEN.md, S1): the program's standard streams
	// are those of go2dos (UTF-8 on the host, the OEM code page for DOS), no
	// screen is drawn and no host front end is needed. Ignored with Headless.
	Pipe bool
}

type driveFlags map[byte]string

func (d driveFlags) String() string { return fmt.Sprint(map[byte]string(d)) }
func (d driveFlags) Set(s string) error {
	l, dir, ok := strings.Cut(s, "=")
	if !ok || len(l) != 1 {
		return fmt.Errorf("want LETTER=DIR, got %q", s)
	}
	d[strings.ToUpper(l)[0]] = dir
	return nil
}

// RegisterFlags declares the common flags on fs and returns the Options they
// fill in. With terminal set it also declares -headless and -display (only
// the plain terminal front end has them).
func RegisterFlags(fs *flag.FlagSet, terminal bool) *Options {
	o := &Options{Drives: map[byte]string{}}
	fs.Var(driveFlags(o.Drives), "drive", "map a drive: LETTER=DIR (repeatable)")
	fs.StringVar(&o.ReadOnly, "ro", "", "make drives read-only for DOS: letters (C or CD or C,D) or all; deleting, writing, renaming are refused")
	fs.BoolVar(&o.Confine, "confine", false, "hide symbolic links that lead out of a drive directory (nothing outside the mapped directories can be reached through links)")
	fs.IntVar(&o.Codepage, "cp", 0, "DOS code page (0 = from the host locale)")
	fs.StringVar(&o.Trace, "trace", "", "write every BIOS/DOS call as JSON lines to `FILE`")
	fs.StringVar(&o.TraceFilter, "trace-filter", "", "only trace calls starting with these comma-separated names (int21,int10,...)")
	fs.StringVar(&o.Keys, "keys", "", "key script to type (or @FILE)")
	if terminal {
		fs.BoolVar(&o.Headless, "headless", false, "run without a terminal; print the final screen")
		fs.BoolVar(&o.Pipe, "pipe", false, "pipe mode: the program's stdin/stdout/stderr are go2dos's own (UTF-8 <-> OEM code page); on by itself when stdin or stdout is not a terminal")
		fs.StringVar(&o.Display, "display", "console", "terminal display: console (command output in the terminal, full-screen programs on the alternate screen) or grid")
	}
	fs.DurationVar(&o.Timeout, "timeout", 0, "stop after this long (default 60s with -headless)")
	fs.StringVar(&o.DumpDir, "dump-dir", ".", "where diagnostic dumps are written")
	fs.BoolVar(&o.DumpOnExit, "dump-on-exit", false, "write a diagnostic dump even on a normal exit")
	fs.StringVar(&o.ScreenOut, "screen-out", "", "write the final screen text to `FILE`")
	fs.StringVar(&o.Record, "record", "", "write the keys typed in this session as a script to `FILE`")
	fs.BoolVar(&o.Lenient, "lenient", false, "answer unsupported BIOS/DOS calls \"not supported\" instead of stopping; print a summary at the end")
	fs.BoolVar(&o.NoLFN, "nolfn", false, "switch the long file name API (INT 21h AH=71h) off: every 71xx call answers \"not supported\"")
	fs.StringVar(&o.Watch, "watch", "", "log writes to these comma-separated addresses: linear hex or SEG:OFF, optionally /N bytes (with -trace or in dumps)")
	fs.BoolVar(&o.HostExec, "host-exec", false, "let the built-in COMMAND.COM run host commands (a line starting with \"!\", or a command in the host PATH); leaves the sandbox")
	fs.StringVar(&o.Size, "size", "", "text screen size `WxH` (columns 80-255, rows 25-255, at most 32768 cells; default 80x25)")
	fs.StringVar(&o.Break, "break", "", "log registers when execution reaches these comma-separated SEG:OFF hex addresses")
	return o
}

// Fail prints err and returns the exit code for a startup failure.
func Fail(err error) int {
	fmt.Fprintln(os.Stderr, "go2dos:", err)
	return 1
}

// Run runs args[0] (with the rest as its command tail) and returns the
// process exit code. host may be nil only with Options.Headless.
func Run(o *Options, args []string, host Host) int {
	pipe := o.Pipe && !o.Headless
	if host == nil && !o.Headless && !pipe {
		return Fail(errors.New("no front end; use -headless"))
	}
	prog := args[0]
	tail := ""
	if len(args) > 1 {
		tail = " " + strings.Join(args[1:], " ")
	}
	drives := o.Drives
	if len(drives) == 0 {
		abs, err := filepath.Abs(prog)
		if err != nil {
			return Fail(err)
		}
		drives = map[byte]string{'C': filepath.Dir(abs)}
		prog = `C:\` + strings.ToUpper(filepath.Base(abs))
	}

	var traceW io.Writer
	if o.Trace != "" {
		f, err := os.Create(o.Trace)
		if err != nil {
			return Fail(err)
		}
		defer f.Close()
		traceW = f
	}
	var filter []string
	if o.TraceFilter != "" {
		filter = strings.Split(o.TraceFilter, ",")
	}

	cfg := machine.Config{Drives: drives, Codepage: o.Codepage, Lenient: o.Lenient, NoLFN: o.NoLFN, HostExec: o.HostExec, TraceLog: traceW, TraceFilter: filter}
	cfg.Confine = o.Confine
	if o.ReadOnly != "" {
		ro, err := dos.ParseReadOnly(o.ReadOnly)
		if err != nil {
			return Fail(err)
		}
		cfg.ReadOnly = ro
	}
	if o.Size != "" {
		w, h, ok := strings.Cut(o.Size, "x")
		cols, err1 := strconv.Atoi(w)
		rows, err2 := strconv.Atoi(h)
		if !ok || err1 != nil || err2 != nil {
			return Fail(fmt.Errorf("bad -size %q (want WxH, for example 132x43)", o.Size))
		}
		cfg.Cols, cfg.Rows = cols, rows
	}
	for _, b := range strings.Split(o.Break, ",") {
		if b == "" {
			continue
		}
		seg, off, ok := strings.Cut(b, ":")
		sg, err1 := strconv.ParseUint(seg, 16, 16)
		of, err2 := strconv.ParseUint(off, 16, 16)
		if !ok || err1 != nil || err2 != nil {
			return Fail(fmt.Errorf("bad -break address %q (want SEG:OFF)", b))
		}
		cfg.Break = append(cfg.Break, uint32(sg)<<16|uint32(of))
	}
	ws, err := machine.ParseWatch(o.Watch)
	if err != nil {
		return Fail(err)
	}
	cfg.Watch = ws
	if pipe {
		cfg.Stdin, cfg.Stdout, cfg.Stderr = os.Stdin, os.Stdout, os.Stderr
	}
	if prov, ok := host.(ClipboardProvider); ok && !o.Headless {
		cfg.Clipboard = prov.Clipboard()
	}
	console, _ := host.(Console)
	display := o.Display
	if display == "" || console == nil {
		display = "grid" // "console" needs a host that can stream
	}
	if !o.Headless && !pipe {
		cfg.OnScreen = host.Draw
		cfg.Display = display
	}
	m, err := machine.New(cfg)
	if err != nil {
		return Fail(err)
	}
	if !o.Headless && !pipe && display == "console" {
		page := m.CP
		m.SetConsoleOutput(func(b []byte) { console.Stream(b, page) }, console.Display)
		if cl, ok := console.(interface{ Clear() }); ok {
			m.SetConsoleClear(cl.Clear)
		}
	}
	if m.CodepageInfo.Note != "" {
		fmt.Fprintln(os.Stderr, "go2dos:", m.CodepageInfo.Note)
	}
	if err := m.Load(prog, tail); err != nil {
		return Fail(err)
	}

	timeout := o.Timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if o.Headless && timeout == 0 {
		timeout = 60 * time.Second
	}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	att, _ := host.(Attacher)
	go func() {
		for range sig {
			if att != nil && att.Attached() {
				continue // Ctrl-C belongs to the host command
			}
			cancel()
			return
		}
	}()

	wantDump := o.DumpOnExit
	restore := func() {}
	if !o.Headless && !pipe {
		restore, err = host.Start(m, display, func(dump bool) {
			if dump {
				wantDump = true
			}
			cancel()
		})
		if err != nil {
			return Fail(err)
		}
	}

	scriptErr := make(chan error, 1)
	if o.Keys != "" {
		text := o.Keys
		if strings.HasPrefix(text, "@") {
			b, err := os.ReadFile(text[1:])
			if err != nil {
				restore()
				return Fail(err)
			}
			text = strings.TrimRight(string(b), "\r\n")
		}
		steps, err := keys.Parse(text, m.CP)
		if err != nil {
			restore()
			return Fail(err)
		}
		go func() {
			err := m.RunScript(ctx, steps, machine.ScriptOptions{Log: os.Stderr})
			scriptErr <- err
			if err != nil {
				cancel()
			} else if o.Headless && o.Timeout == 0 {
				cancel()
			}
		}()
	}

	runErr := m.Run(ctx)
	if !o.Headless && !pipe && display == "console" && m.GridShown() {
		console.Display(false)
	}
	restore()
	code := 0
	reason := ""
	var exit *machine.ExitError
	var fault *machine.FaultError
	switch {
	case errors.As(runErr, &exit):
		code = exit.Code
		reason = runErr.Error()
	case errors.As(runErr, &fault):
		code, wantDump, reason = 3, true, runErr.Error()
	case errors.Is(runErr, context.DeadlineExceeded):
		reason = "timeout"
	case errors.Is(runErr, context.Canceled):
		reason = "stopped by the user"
	case runErr != nil:
		code, wantDump, reason = 3, true, runErr.Error()
	}
	select {
	case err := <-scriptErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "go2dos: key script:", err)
			code, wantDump, reason = 4, true, "key script: "+err.Error()
		}
	default:
	}
	if o.Headless {
		fmt.Println(m.Screen().Text())
	}
	if o.ScreenOut != "" {
		os.WriteFile(o.ScreenOut, []byte(m.Screen().Text()+"\n"), 0o644)
	}
	if o.Record != "" {
		os.WriteFile(o.Record, []byte(m.RecordedKeys()+"\n"), 0o644)
	}
	if !pipe || exit == nil {
		// In pipe mode stderr belongs to the program: only abnormal ends are reported
		fmt.Fprintln(os.Stderr, "go2dos:", reason)
	}
	if o.Lenient {
		fmt.Fprint(os.Stderr, "go2dos: unsupported calls answered in lenient mode:\n",
			hle.FormatUnsupported(m.Unsupported()))
	}
	if wantDump {
		dir, err := m.Dump(o.DumpDir, reason)
		if err != nil {
			fmt.Fprintln(os.Stderr, "go2dos: dump failed:", err)
		} else {
			fmt.Fprintln(os.Stderr, "go2dos: diagnostic dump written to", dir)
		}
	}
	return code
}
