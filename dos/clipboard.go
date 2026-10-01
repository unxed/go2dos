// Package dos implements a high-level-emulated DOS kernel.
// clipboard.go implements the WinOldAp clipboard server (INT 2Fh AX=17xxh)
// and provides the Clipboard interface for frontends.
package dos

// Clipboard is the text clipboard interface for clipboard servers and frontends.
// GetText returns the current clipboard content (or empty string if unavailable).
// SetText writes text to the clipboard.
type Clipboard interface {
	GetText() (string, error)
	SetText(text string) error
}

// InMemoryClipboard is a simple in-memory Clipboard implementation for testing.
type InMemoryClipboard struct {
	text string
}

// NewInMemoryClipboard creates an in-memory clipboard.
func NewInMemoryClipboard() *InMemoryClipboard {
	return &InMemoryClipboard{}
}

// GetText returns the current clipboard content.
func (c *InMemoryClipboard) GetText() (string, error) {
	return c.text, nil
}

// SetText writes text to the clipboard.
func (c *InMemoryClipboard) SetText(text string) error {
	c.text = text
	return nil
}
