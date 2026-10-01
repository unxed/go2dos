package frontend

import "sync"

// Clipboard is the host clipboard as the front ends and the DOS clipboard
// server (WinOldAp, INT 2Fh/17xx) see it. Implementations: MemoryClipboard
// (tests, no host clipboard), a vtui wrapper in cmd/go2dos-vtui.
type Clipboard interface {
	GetText() (string, error)
	SetText(string) error
}

// MemoryClipboard keeps the text in memory only.
type MemoryClipboard struct {
	mu   sync.Mutex
	text string
}

func (c *MemoryClipboard) GetText() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, nil
}

func (c *MemoryClipboard) SetText(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text = s
	return nil
}
