package frontend

import "sync"

// Clipboard is the host clipboard as the front ends and the DOS clipboard
// server (WinOldAp, INT 2Fh/17xx) see it. Implementations: MemoryClipboard
// (tests, no host clipboard), a vtui wrapper in cmd/go2dos-vtui.
type Clipboard interface {
	GetText() (string, error)
	SetText(string) error
}

// ClipboardExtended extends Clipboard with file transfer support.
type ClipboardExtended interface {
	Clipboard
	GetFiles() ([]string, error)
	SetFiles(paths []string) error
	IsDragDropAvailable() bool
	OnDragDrop(paths []string) bool
}

// MemoryClipboard keeps text and files in memory only.
type MemoryClipboard struct {
	mu    sync.Mutex
	text  string
	files []string
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

func (c *MemoryClipboard) GetFiles() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.files) == 0 {
		return []string{}, nil
	}
	result := make([]string, len(c.files))
	copy(result, c.files)
	return result, nil
}

func (c *MemoryClipboard) SetFiles(paths []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files = make([]string, len(paths))
	copy(c.files, paths)
	return nil
}

func (c *MemoryClipboard) IsDragDropAvailable() bool {
	return false
}

func (c *MemoryClipboard) OnDragDrop(paths []string) bool {
	return false
}
