package main

import (
	"os"
	"testing"

	"golang.org/x/term"

	"github.com/unxed/vtui"
)

// Без терминала на stdin фронтенд отказывается стартовать, не трогая консоль.
func TestStartNeedsTerminal(t *testing.T) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		t.Skip("stdin is a terminal")
	}
	h := newHost(vtui.NewSilentScreenBuf())
	if _, err := h.Start(nil, "grid", func(bool) {}); err == nil {
		t.Fatal("Start succeeded without a terminal")
	}
}
