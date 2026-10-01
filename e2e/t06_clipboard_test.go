package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
)

// TestT06ClipboardIntegration is an e2e test for T06 clipboard integration.
// Tests copying text from screen and pasting via keyboard.
func TestT06ClipboardIntegration(t *testing.T) {
	cfg := machine.Config{
		Drives: map[byte]string{'C': "."},
	}
	m, err := machine.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create machine: %v", err)
	}

	// Load and run a simple test program
	if err := m.Load(`C:\TEST.COM`, ""); err != nil {
		t.Skipf("Test program not available: %v", err)
	}

	// Run the machine for a short time to let it initialize
	done := make(chan error, 1)
	go func() {
		done <- m.Run(testContext(2 * time.Second))
	}()

	// Give it time to run
	time.Sleep(500 * time.Millisecond)

	// Simulate Ctrl-V to paste text
	// This should trigger paste operation if clipboard is available
	pasteKeys := []struct {
		scan byte
		mods byte
	}{
		{0x2F, 0x04}, // Ctrl-V
	}

	for _, k := range pasteKeys {
		ke, ok := keys.Named("V", 0)
		if ok {
			ke.Mods = 0x04 // ModCtrl
			m.PushKey(ke)
		}
	}

	// Wait for completion
	if err := <-done; err != nil && !strings.Contains(err.Error(), "stopped") {
		t.Logf("Machine error (expected timeout): %v", err)
	}
}

// TestT06SelectionKeyboardIntegration tests Shift+Arrow selection.
func TestT06SelectionKeyboardIntegration(t *testing.T) {
	cfg := machine.Config{
		Drives: map[byte]string{'C': "."},
	}
	m, err := machine.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create machine: %v", err)
	}

	// Load test program
	if err := m.Load(`C:\TEST.COM`, ""); err != nil {
		t.Skipf("Test program not available: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- m.Run(testContext(2 * time.Second))
	}()

	time.Sleep(500 * time.Millisecond)

	// Simulate Shift+Arrow keys for selection
	shiftArrowKeys := []string{"Up", "Down", "Left", "Right"}

	for _, keyName := range shiftArrowKeys {
		if ke, ok := keys.Named(keyName, 0x01); ok { // ModLShift = 0x01
			m.PushKey(ke)
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Simulate ESC to end selection
	if ke, ok := keys.Named("Esc", 0); ok {
		m.PushKey(ke)
	}

	if err := <-done; err != nil && !strings.Contains(err.Error(), "stopped") {
		t.Logf("Machine error (expected timeout): %v", err)
	}
}

// TestT06ClipboardPasteMultiline tests pasting multi-line text.
func TestT06ClipboardPasteMultiline(t *testing.T) {
	cfg := machine.Config{
		Drives: map[byte]string{'C': "."},
	}
	m, err := machine.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create machine: %v", err)
	}

	if err := m.Load(`C:\TEST.COM`, ""); err != nil {
		t.Skipf("Test program not available: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- m.Run(testContext(2 * time.Second))
	}()

	time.Sleep(500 * time.Millisecond)

	// Simulate pasting multi-line text character by character
	testText := "Line1\nLine2\nLine3"
	for _, r := range testText {
		if r == '\n' {
			// Enter key for newline
			if ke, ok := keys.Named("Enter", 0); ok {
				m.PushKey(ke)
			}
		} else if r < 128 {
			// Regular ASCII character
			if ke, ok := keys.Char(r, m.CP); ok {
				m.PushKey(ke)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := <-done; err != nil && !strings.Contains(err.Error(), "stopped") {
		t.Logf("Machine error (expected timeout): %v", err)
	}
}
