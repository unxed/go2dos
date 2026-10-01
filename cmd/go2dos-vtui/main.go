// Command go2dos-vtui runs a DOS program in a terminal through the vtui
// library: the screen goes out through vtui.ScreenBuf, keys come from
// vtinput (kitty and win32 input modes, Windows console events). It is a
// nested module so that the main module does not depend on vtui.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/unxed/go2dos/frontend"
	"github.com/unxed/vtui"
)

const usage = `go2dos-vtui - run a DOS program (vtui front end)

usage: go2dos-vtui [flags] PROGRAM [ARGS...]

PROGRAM is a host path (its directory becomes C:) or, with -drive, a DOS
path such as C:\VC.COM.

Press Ctrl-] then:
  q  quit
  d  quit and write a diagnostic dump
  ]  send Ctrl-] to the program

flags:
`

func main() {
	os.Exit(run())
}

func run() int {
	o := frontend.RegisterFlags(flag.CommandLine, false)
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage); flag.PrintDefaults() }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		return 2
	}
	return frontend.Run(o, flag.Args(), newHost(vtui.NewScreenBuf()))
}
