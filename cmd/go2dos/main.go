// Command go2dos runs a DOS program in a terminal, or headless for automation.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
)

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

const usage = `go2dos - run a DOS program

usage: go2dos [flags] PROGRAM [ARGS...]

PROGRAM is a host path (its directory becomes C:) or, with -drive, a DOS
path such as C:\VC.COM.

While running in a terminal, press Ctrl-] then:
  q  quit
  d  quit and write a diagnostic dump
  ]  send Ctrl-] to the program

flags:
`

func main() {
	os.Exit(run())
}

func run() int {
	drives := driveFlags{}
	flag.Var(drives, "drive", "map a drive: LETTER=DIR (repeatable)")
	cpNum := flag.Int("cp", 0, "DOS code page (0 = from the host locale)")
	traceFile := flag.String("trace", "", "write every BIOS/DOS call as JSON lines to `FILE`")
	traceFilter := flag.String("trace-filter", "", "only trace calls starting with these comma-separated names (int21,int10,...)")
	keyScript := flag.String("keys", "", "key script to type (or @FILE)")
	headless := flag.Bool("headless", false, "run without a terminal; print the final screen")
	timeout := flag.Duration("timeout", 0, "stop after this long (default 60s with -headless)")
	dumpDir := flag.String("dump-dir", ".", "where diagnostic dumps are written")
	dumpOnExit := flag.Bool("dump-on-exit", false, "write a diagnostic dump even on a normal exit")
	screenOut := flag.String("screen-out", "", "write the final screen text to `FILE`")
	record := flag.String("record", "", "write the keys typed in this session as a script to `FILE`")
	lenient := flag.Bool("lenient", false, "answer unsupported BIOS/DOS calls \"not supported\" instead of stopping; print a summary at the end")
	hostExec := flag.Bool("host-exec", false, "allow host commands in the shell (! prefix or after DOS search)")
	watch := flag.String("watch", "", "log writes to these comma-separated addresses: linear hex or SEG:OFF, optionally /N bytes (with -trace or in dumps)")
	display := flag.String("display", "console", "terminal display: console (command output in the terminal, full-screen programs on the alternate screen) or grid")
	brk := flag.String("break", "", "log registers when execution reaches these comma-separated SEG:OFF hex addresses")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage); flag.PrintDefaults() }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		return 2
	}
	prog := flag.Arg(0)
	tail := ""
	if flag.NArg() > 1 {
		tail = " " + strings.Join(flag.Args()[1:], " ")
	}
	if len(drives) == 0 {
		abs, err := filepath.Abs(prog)
		if err != nil {
			return fail(err)
		}
		drives['C'] = filepath.Dir(abs)
		prog = `C:\` + strings.ToUpper(filepath.Base(abs))
	}

	var traceW io.Writer
	if *traceFile != "" {
		f, err := os.Create(*traceFile)
		if err != nil {
			return fail(err)
		}
		defer f.Close()
		traceW = f
	}
	var filter []string
	if *traceFilter != "" {
		filter = strings.Split(*traceFilter, ",")
	}

	interactive := !*headless
	if interactive && !term.IsTerminal(int(os.Stdin.Fd())) {
		return fail(errors.New("stdin is not a terminal; use -headless"))
	}
	var rend *renderer
	if interactive {
		rend = newRenderer(os.Stdout)
	}
	cfg := machine.Config{Drives: drives, Codepage: *cpNum, Lenient: *lenient, HostExec: *hostExec, TraceLog: traceW, TraceFilter: filter}
	for _, b := range strings.Split(*brk, ",") {
		if b == "" {
			continue
		}
		seg, off, ok := strings.Cut(b, ":")
		s, err1 := strconv.ParseUint(seg, 16, 16)
		o, err2 := strconv.ParseUint(off, 16, 16)
		if !ok || err1 != nil || err2 != nil {
			return fail(fmt.Errorf("bad -break address %q (want SEG:OFF)", b))
		}
		cfg.Break = append(cfg.Break, uint32(s)<<16|uint32(o))
	}
	ws, err := machine.ParseWatch(*watch)
	if err != nil {
		return fail(err)
	}
	cfg.Watch = ws
	if rend != nil {
		cfg.OnScreen = rend.draw
		cfg.Display = *display
	}
	m, err := machine.New(cfg)
	if err != nil {
		return fail(err)
	}
	if rend != nil && *display == "console" {
		cfg := m.CP
		m.SetConsoleOutput(func(b []byte) { rend.stream(b, cfg) }, rend.display)
	}
	if m.CodepageInfo.Note != "" {
		fmt.Fprintln(os.Stderr, "go2dos:", m.CodepageInfo.Note)
	}
	if err := m.Load(prog, tail); err != nil {
		return fail(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if *headless && *timeout == 0 {
		*timeout = 60 * time.Second
	}
	if *timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() { <-sig; cancel() }()

	wantDump := *dumpOnExit
	if interactive {
		restore, err := setupTerminal(*display != "console")
		if err != nil {
			return fail(err)
		}
		defer restore()
		p := &inputParser{page: m.CP, push: m.PushKey, cmd: func(c termCmd) {
			if c == cmdDump {
				wantDump = true
			}
			cancel()
		}}
		go p.run(os.Stdin)
	}

	scriptErr := make(chan error, 1)
	if *keyScript != "" {
		text := *keyScript
		if strings.HasPrefix(text, "@") {
			b, err := os.ReadFile(text[1:])
			if err != nil {
				return fail(err)
			}
			text = strings.TrimRight(string(b), "\r\n")
		}
		steps, err := keys.Parse(text, m.CP)
		if err != nil {
			return fail(err)
		}
		go func() {
			err := m.RunScript(ctx, steps, machine.ScriptOptions{Log: os.Stderr})
			scriptErr <- err
			if err != nil {
				cancel()
			} else if *headless && *timeout == 0 {
				cancel()
			}
		}()
	}

	runErr := m.Run(ctx)
	if interactive && *display != "console" {
		fmt.Print("\x1b[0m\x1b[?25h\x1b[?1049l")
	} else if interactive && m.GridShown() {
		rend.display(false)
	}
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
	if *headless {
		fmt.Println(m.Screen().Text())
	}
	if *screenOut != "" {
		os.WriteFile(*screenOut, []byte(m.Screen().Text()+"\n"), 0o644)
	}
	if *record != "" {
		os.WriteFile(*record, []byte(m.RecordedKeys()+"\n"), 0o644)
	}
	fmt.Fprintln(os.Stderr, "go2dos:", reason)
	if *lenient {
		fmt.Fprint(os.Stderr, "go2dos: unsupported calls answered in lenient mode:\n",
			hle.FormatUnsupported(m.Unsupported()))
	}
	if wantDump {
		dir, err := m.Dump(*dumpDir, reason)
		if err != nil {
			fmt.Fprintln(os.Stderr, "go2dos: dump failed:", err)
		} else {
			fmt.Fprintln(os.Stderr, "go2dos: diagnostic dump written to", dir)
		}
	}
	return code
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

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "go2dos:", err)
	return 1
}
