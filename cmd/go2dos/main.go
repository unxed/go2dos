// Command go2dos runs a DOS program in a terminal, or headless for automation.
package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/unxed/go2dos/frontend"
)

const usage = `go2dos - run a DOS program

usage: go2dos [flags] PROGRAM [ARGS...]

If stdin or stdout is not a terminal (or with -pipe) go2dos runs in pipe
mode: the program's stdin/stdout/stderr are go2dos's own, with text converted
between UTF-8 and the OEM code page, and nothing is drawn.

PROGRAM is a host path (its directory becomes C:) or, with -drive, a DOS
path such as C:\VC.COM.

While running in a terminal, press Ctrl-] then:
  q  quit
  d  quit and write a diagnostic dump
  c  copy the screen text to the clipboard
  v  paste the clipboard as keystrokes
  ]  send Ctrl-] to the program

flags:
`

func main() {
	os.Exit(run())
}

func run() int {
	o := frontend.RegisterFlags(flag.CommandLine, true)
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage); flag.PrintDefaults() }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		return 2
	}
	var host frontend.Host
	if !o.Headless {
		// Without a terminal on either side the program works as a filter
		// (pipe mode): its standard streams are ours.
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			o.Pipe = true
		}
		if !o.Pipe {
			host = newTermHost(os.Stdout)
		}
	}
	return frontend.Run(o, flag.Args(), host)
}
