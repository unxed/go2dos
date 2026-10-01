package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/unxed/go2dos/m5"
)

const usage = `m5 - Terminal-based file browser (proof of concept)

Integrates T-04 (keyboard event decoder), T-05 (UI framework),
and T-06 (filesystem abstraction) into a unified system.

usage: m5 [flags]
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	app, err := m5.NewApp(os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
