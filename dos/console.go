package dos

import (
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
)

// conWrite writes to the console with CON cooked-mode semantics (TAB expansion).
func (d *DOS) conWrite(b []byte) {
	v := d.b.Video
	page := v.ActivePage()
	for _, ch := range b {
		if ch == '\t' {
			for {
				v.Teletype(' ', page)
				if _, col := d.cursorCol(); col%8 == 0 {
					break
				}
			}
			continue
		}
		v.Teletype(ch, page)
	}
}

func (d *DOS) cursorCol() (int, int) {
	p := d.e.Mem.R16(0x450 + uint32(d.b.Video.ActivePage())*2)
	return int(p >> 8), int(p & 0xFF)
}

// readChar returns the next character for DOS character input; extended keys
// return 0 followed by the scan code. ok is false if no key is available.
func (d *DOS) readChar() (byte, bool) {
	if d.pendScan != 0 {
		c := d.pendScan
		d.pendScan = 0
		return c, true
	}
	w, ok := d.b.ReadKey()
	if !ok {
		return 0, false
	}
	if byte(w) == 0 {
		d.pendScan = byte(w >> 8)
	}
	return byte(w), true
}

func (d *DOS) charAvailable() bool { return d.pendScan != 0 || d.b.KeyAvailable() }

// lineInput implements cooked line editing shared by INT 21h/0Ah and reads
// from CON. It returns false (and must be retried) until Enter is pressed.
func (d *DOS) lineInput(max int) bool {
	for {
		c, ok := d.readChar()
		if !ok {
			return false
		}
		switch c {
		case 0:
			d.readChar() // drop the scan code of extended keys
		case 0x0D:
			d.conWrite([]byte{0x0D})
			d.lineDone = true
			return true
		case 0x08:
			if len(d.line) > 0 {
				d.line = d.line[:len(d.line)-1]
				d.conWrite([]byte{8, ' ', 8})
			}
		case 0x1B:
			for range d.line {
				d.conWrite([]byte{8, ' ', 8})
			}
			d.line = d.line[:0]
		default:
			if len(d.line) < max {
				d.line = append(d.line, c)
				d.conWrite([]byte{c})
			}
		}
	}
}

// conRead implements reading from CON: a cooked line ending in CR LF.
func (d *DOS) conRead(buf []byte) (int, uint16) {
	if len(d.conIn) == 0 {
		if !d.lineInput(126) {
			return 0, 0xFFFF // marker: retry
		}
		d.conIn = append(append([]byte{}, d.line...), 0x0D, 0x0A)
		d.line = d.line[:0]
		d.conWrite([]byte{0x0A})
	}
	n := copy(buf, d.conIn)
	d.conIn = d.conIn[n:]
	return n, 0
}

// charFunc handles INT 21h functions 01h-0Ch.
func (d *DOS) charFunc(e *hle.Env, ah byte) error {
	c := e.CPU
	switch ah {
	case 0x01, 0x07, 0x08:
		ch, ok := d.readChar()
		if !ok {
			e.Idle()
			return cpu.ErrRetry
		}
		if ah == 0x01 {
			d.conWrite([]byte{ch})
		}
		c.SetAL(ch)
	case 0x02:
		d.conWrite([]byte{c.DL()})
		c.SetAL(c.DL())
	case 0x05:
		e.Note("printer output discarded")
	case 0x06:
		if c.DL() != 0xFF {
			d.conWrite([]byte{c.DL()})
			c.SetAL(c.DL())
			return nil
		}
		ch, ok := d.readChar()
		e.SetZF(!ok)
		if !ok {
			e.Idle()
			c.SetAL(0)
			return nil
		}
		c.SetAL(ch)
	case 0x09:
		a := e.DSDX()
		var out []byte
		for i := 0; i < 0x10000; i++ {
			ch := e.Mem.R8(a + uint32(i))
			if ch == '$' {
				break
			}
			out = append(out, ch)
		}
		d.conWrite(out)
		c.SetAL('$')
	case 0x0A:
		buf := e.DSDX()
		max := int(e.Mem.R8(buf))
		if max == 0 {
			return nil
		}
		if !d.lineInput(max - 1) {
			e.Idle()
			return cpu.ErrRetry
		}
		e.Mem.W8(buf+1, byte(len(d.line)))
		e.Mem.SetBytes(buf+2, d.line)
		e.Mem.W8(buf+2+uint32(len(d.line)), 0x0D)
		d.line = d.line[:0]
		d.lineDone = false
	case 0x0B:
		if d.charAvailable() {
			c.SetAL(0xFF)
		} else {
			c.SetAL(0)
			e.Idle()
		}
	case 0x0C:
		for d.b.KeyAvailable() {
			d.b.ReadKey()
		}
		d.pendScan = 0
		fn := c.AL()
		switch fn {
		case 0x01, 0x06, 0x07, 0x08, 0x0A:
			c.SetAH(fn)
			return d.charFunc(e, fn)
		}
	}
	return nil
}
