package dos

import (
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// WinOldAp constants
const (
	cfText    = 1 // plain text format
	cfOEMText = 7 // OEM code page text
)

// winoldap handles INT 2Fh AX=17xxh (WinOldAp clipboard server).
// Implements basic clipboard functions per Ralf Brown's Interrupt List.
func (d *DOS) winoldap(e *hle.Env) error {
	c := e.CPU
	ax := c.R[cpu.AX]

	// If clipboard is not available, fail all requests.
	if d.clipboard == nil {
		c.R[cpu.AX] = 0 // failure
		return nil
	}

	switch ax {
	case 0x1700: // Check installed version
		// Return 1703h to indicate WinOldAp 3.0 is installed.
		c.R[cpu.AX] = 0x1703
		return nil

	case 0x1701: // Open clipboard
		// No state needed for simple implementation.
		c.R[cpu.AX] = 1 // success
		return nil

	case 0x1702: // Empty clipboard
		// Clear the clipboard.
		err := d.clipboard.SetText("")
		if err != nil {
			c.R[cpu.AX] = 0 // failure
			return nil
		}
		c.R[cpu.AX] = 1 // success
		return nil

	case 0x1703: // Set clipboard data
		// ES:BX = data, CX = format, SI = size
		seg := c.Seg(cpu.ES)
		off := c.R[cpu.BX]
		size := uint32(c.R[cpu.SI])
		format := c.R[cpu.CX]

		if format != cfText && format != cfOEMText {
			c.R[cpu.AX] = 0 // unsupported format
			return nil
		}

		// Read data from memory.
		addr := mem.Lin(seg, off)
		data := e.Mem.Bytes(addr, int(size))

		// Convert OEM bytes to text.
		text := oemToText(e, data, format)

		// Set clipboard.
		err := d.clipboard.SetText(text)
		if err != nil {
			c.R[cpu.AX] = 0 // failure
			return nil
		}
		c.R[cpu.AX] = 1 // success
		return nil

	case 0x1704: // Query clipboard size
		// CX = format, returns DX:AX = size
		format := c.R[cpu.CX]
		if format != cfText && format != cfOEMText {
			c.R[cpu.DX] = 0
			c.R[cpu.AX] = 0 // unsupported format
			return nil
		}

		text, err := d.clipboard.GetText()
		if err != nil {
			c.R[cpu.DX] = 0
			c.R[cpu.AX] = 0
			return nil
		}

		// Size is the OEM-encoded bytes + null terminator.
		oemBytes := textToOEM(e, text, format)
		size := uint32(len(oemBytes)) + 1 // +1 for null terminator

		c.R[cpu.DX] = uint16(size >> 16)
		c.R[cpu.AX] = uint16(size & 0xFFFF)
		return nil

	case 0x1705: // Get clipboard data
		// ES:BX = buffer, CX = format, SI = buffer size
		// Returns: AX = size actually returned
		seg := c.Seg(cpu.ES)
		off := c.R[cpu.BX]
		bufSize := int(c.R[cpu.SI])
		format := c.R[cpu.CX]

		if format != cfText && format != cfOEMText {
			c.R[cpu.AX] = 0 // unsupported format
			return nil
		}

		text, err := d.clipboard.GetText()
		if err != nil {
			c.R[cpu.AX] = 0
			return nil
		}

		// Convert text to OEM bytes.
		oemBytes := textToOEM(e, text, format)

		// Limit to buffer size - 1 (for null terminator).
		copyLen := len(oemBytes)
		if copyLen > bufSize-1 {
			copyLen = bufSize - 1
		}

		// Write to memory.
		addr := mem.Lin(seg, off)
		for i := 0; i < copyLen; i++ {
			e.Mem.W8(addr+uint32(i), oemBytes[i])
		}
		// Add null terminator.
		e.Mem.W8(addr+uint32(copyLen), 0)

		c.R[cpu.AX] = uint16(copyLen) & 0xFFFF
		return nil

	case 0x1708: // Close clipboard
		// No state cleanup needed for simple implementation.
		return nil

	case 0x1709: // Compact memory
		// No-op for emulator.
		return nil

	default:
		// Unsupported WinOldAp function.
		c.R[cpu.AX] = 0 // failure
		return nil
	}
}

// oemToText converts OEM-encoded bytes to a Unicode string.
// For CF_OEMTEXT, bytes are interpreted as OEM code page.
// For CF_TEXT, bytes are interpreted as Windows ANSI (which we approximate as OEM for now).
func oemToText(e *hle.Env, data []byte, format uint16) string {
	// Convert OEM bytes to UTF-8 using the code page.
	var result []rune
	for _, b := range data {
		if b == 0 { // null terminator
			break
		}
		if b == '\r' {
			// Keep carriage returns as-is for now; they will be converted to newlines later.
			result = append(result, rune(b))
		} else {
			// Convert OEM byte to rune using the code page.
			r := e.CP.Rune(b)
			result = append(result, r)
		}
	}
	return string(result)
}

// textToOEM converts a Unicode string to OEM-encoded bytes.
// For CF_OEMTEXT, the result is OEM code page.
// For CF_TEXT, the result is Windows ANSI (which we approximate as OEM for now).
func textToOEM(e *hle.Env, text string, format uint16) []byte {
	var result []byte
	for _, r := range text {
		if r == '\n' {
			// Convert newline to CRLF for clipboard compatibility.
			result = append(result, '\r', '\n')
		} else {
			// Convert rune to OEM byte using the code page.
			if b, ok := e.CP.Byte(r); ok {
				result = append(result, b)
			} else {
				// Unmappable character: use '?' as fallback.
				result = append(result, '?')
			}
		}
	}
	return result
}
