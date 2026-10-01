package keys

// T-04: Win32 Keyboard Event Decoder
// Decodes ANSI escape sequences and raw input into keyboard events

import (
	"bytes"
)

// KeyType represents the type of key event.
type KeyType int

const (
	KeyRune KeyType = iota
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyDelete
	KeyBackspace
	KeyEnter
	KeyTab
	KeyEscape
	KeyCtrlC
	KeyCtrlV
	KeyCtrlX
)

// KeyEvent represents a keyboard event.
type KeyEvent struct {
	Type  KeyType
	Rune  rune
	Ctrl  bool
	Alt   bool
	Shift bool
}

// Decoder decodes ANSI escape sequences and raw input into key events.
type Decoder struct {
	buf bytes.Buffer
}

// NewDecoder creates a new key decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Push adds bytes to the decoder's buffer.
func (d *Decoder) Push(b []byte) {
	d.buf.Write(b)
}

// Next returns the next key event, or nil if no complete event is available.
func (d *Decoder) Next() *KeyEvent {
	if d.buf.Len() == 0 {
		return nil
	}

	b := d.buf.Bytes()

	// Check for escape sequences
	if len(b) > 0 && b[0] == '\x1b' {
		return d.parseEscape()
	}

	// Check for control characters
	if len(b) > 0 {
		return d.parseControl(b[0])
	}

	return nil
}

// parseEscape parses ANSI escape sequences
func (d *Decoder) parseEscape() *KeyEvent {
	b := d.buf.Bytes()

	if len(b) < 2 {
		return nil
	}

	// ESC [ sequences (CSI)
	if b[1] == '[' {
		return d.parseCsi()
	}

	// Other escape sequences
	if len(b) >= 2 {
		d.buf.Next(2)
		return &KeyEvent{Type: KeyEscape}
	}

	return nil
}

// parseCsi parses Control Sequence Introducer sequences
func (d *Decoder) parseCsi() *KeyEvent {
	b := d.buf.Bytes()

	if len(b) < 3 {
		return nil
	}

	// Find the end of the sequence
	endIdx := -1
	for i := 2; i < len(b); i++ {
		if (b[i] >= 'A' && b[i] <= 'Z') || (b[i] >= 'a' && b[i] <= 'z') {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil
	}

	final := b[endIdx]

	// Consume the sequence
	d.buf.Next(endIdx + 1)

	// Parse the final byte
	switch final {
	case 'A':
		return &KeyEvent{Type: KeyArrowUp}
	case 'B':
		return &KeyEvent{Type: KeyArrowDown}
	case 'C':
		return &KeyEvent{Type: KeyArrowRight}
	case 'D':
		return &KeyEvent{Type: KeyArrowLeft}
	case 'H':
		return &KeyEvent{Type: KeyHome}
	case 'F':
		return &KeyEvent{Type: KeyEnd}
	}

	return nil
}

// parseControl parses control characters
func (d *Decoder) parseControl(ch byte) *KeyEvent {
	// Consume one byte
	d.buf.Next(1)

	switch ch {
	case '\r', '\n':
		return &KeyEvent{Type: KeyEnter}
	case '\t':
		return &KeyEvent{Type: KeyTab}
	case '\x08', '\x7f': // Backspace or DEL
		return &KeyEvent{Type: KeyBackspace}
	case '\x03': // Ctrl-C
		return &KeyEvent{Type: KeyCtrlC, Ctrl: true}
	case '\x16': // Ctrl-V
		return &KeyEvent{Type: KeyCtrlV, Ctrl: true}
	case '\x18': // Ctrl-X
		return &KeyEvent{Type: KeyCtrlX, Ctrl: true}
	case '\x1b': // Escape
		d.buf.UnreadByte()
		return nil
	default:
		if ch < 32 {
			return &KeyEvent{Type: KeyRune, Rune: rune(ch), Ctrl: true}
		}
		return &KeyEvent{Type: KeyRune, Rune: rune(ch)}
	}
}
