// Command go2dos runs a DOS program in a terminal, or headless for automation.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/unxed/go2dos/frontend"
)

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
	o := frontend.RegisterFlags(flag.CommandLine, true)
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage); flag.PrintDefaults() }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		return 2
	}
	var host frontend.Host
	if !o.Headless {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return frontend.Fail(errors.New("stdin is not a terminal; use -headless"))
		}
		host = newTermHost(os.Stdout)
	}
	return frontend.Run(o, flag.Args(), host)
}
