// Package bios implements the high-level-emulated PC BIOS: the data area,
// INT 10h video, INT 16h/09h keyboard, INT 08h/1Ah time and small services.
package bios

import (
	"time"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
)

// Modifier bits of KeyEvent.Mods (same layout as BDA 0040:0017 bits 0-3).
const (
	ModRShift = 1 << 0
	ModLShift = 1 << 1
	ModCtrl   = 1 << 2
	ModAlt    = 1 << 3
)

// scanBreak is the scan code of the Break key (Ctrl-Pause).
const scanBreak = 0x46

// KeyEvent is one keystroke as the BIOS sees it.
type KeyEvent struct {
	Scan  byte // set-1 make code
	ASCII byte // character in the DOS code page, 0 for extended keys, E0h for gray keys
	Mods  byte
	Gray  bool // key from the separate cursor block (E0 prefix)
}

// Word returns the INT 16h buffer word.
func (k KeyEvent) Word() uint16 { return uint16(k.Scan)<<8 | uint16(k.ASCII) }

// BIOS holds the firmware state.
type BIOS struct {
	e     *hle.Env
	Video *Video

	// Keyboard controller state.
	kbdQueue []KeyEvent
	inFlight *KeyEvent
	release  bool // the in-flight event is the break code
	port60   byte
	// IRQ1 is called when a scan code is ready in port 60h.
	IRQ1 func()

	emptyPolls int
	// IdlePolls is the number of consecutive empty INT 16h polls after which
	// the guest is considered idle.
	IdlePolls int
}

// New installs the BIOS into memory and the IVT.
func New(e *hle.Env) *BIOS {
	b := &BIOS{e: e, Video: &Video{e: e}, IdlePolls: 50}
	m := e.Mem

	// Default vectors: an unexpected interrupt stops the machine (fail fast).
	for v := 0; v < 256; v++ {
		n := v
		off := e.IRETStub(e.Register("unhandled", func(e *hle.Env) error {
			return hle.Unsupported("interrupt %02Xh (no handler installed)", n)
		}))
		e.SetVector(byte(v), hle.ROMSeg, off)
	}
	iret := e.Emit([]byte{0xCF})
	for _, v := range []byte{0x01, 0x03, 0x04, 0x1B, 0x1C, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77} {
		e.SetVector(v, hle.ROMSeg, iret)
	}
	e.SetVector(0x1D, 0, 0) // video parameter table: none
	e.SetVector(0x1E, 0, 0) // diskette parameter table: none
	e.SetVector(0x1F, 0, 0) // graphics font: none
	e.SetVector(0x43, 0, 0)

	// INT 08h: tick, chain to INT 1Ch, EOI, IRET.
	tick := e.Register("int08", b.int08)
	off := e.Emit(append(hle.Trap(tick), 0xCD, 0x1C, 0x50, 0xB0, 0x20, 0xE6, 0x20, 0x58, 0xCF))
	e.SetVector(0x08, hle.ROMSeg, off)
	// INT 09h: read port 60h, update the buffer, EOI, IRET.
	kbd := e.Register("int09", b.int09)
	off = e.Emit(append(hle.Trap(kbd), 0x50, 0xB0, 0x20, 0xE6, 0x20, 0x58, 0xCF))
	e.SetVector(0x09, hle.ROMSeg, off)

	e.HookInt(0x05, "int05", func(*hle.Env) error { return nil })
	e.HookInt(0x10, "int10", b.Video.int10)
	e.HookInt(0x11, "int11", func(e *hle.Env) error { e.CPU.R[cpu.AX] = e.Mem.R16(bdaEquipment); return nil })
	e.HookInt(0x12, "int12", func(e *hle.Env) error { e.CPU.R[cpu.AX] = e.Mem.R16(bdaMemSize); return nil })
	e.HookInt(0x15, "int15", b.int15)
	e.HookInt(0x16, "int16", b.int16)
	// Lenient mode: INT 15h answers like its documented "function not
	// supported" (AH=86h, CF set); the other calls leave the registers alone.
	e.Fallback("int15", func(e *hle.Env) error { e.CPU.SetAH(0x86); e.SetCF(true); return nil })
	e.HookInt(0x1A, "int1A", b.int1A)
	e.HookInt(0x33, "int33", b.int33)

	// BIOS data area.
	m.W16(bdaEquipment, 0x0020) // 80x25 color, no diskette drives
	m.W16(bdaMemSize, 640)
	m.W16(bdaKbdHead, 0x1E)
	m.W16(bdaKbdTail, 0x1E)
	m.W16(bdaKbdStart, 0x1E)
	m.W16(bdaKbdEnd, 0x3E)
	m.W8(bdaKbdFlags3, 0x10) // enhanced (101-key) keyboard
	b.Video.setMode(3, true)
	b.setTicksFromClock()

	// ROM identification: date at F000:FFF5, model byte at F000:FFFE (AT).
	m.Poke(0xFFFF5, []byte("10/01/26")...)
	m.Poke(0xFFFFE, 0xFC)
	// Reset vector: HLT loop, never used by the HLE profile.
	m.Poke(0xFFFF0, 0xF4, 0xEB, 0xFD)
	return b
}

func (b *BIOS) setTicksFromClock() {
	t := b.e.Now()
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	ticks := uint32(t.Sub(midnight).Seconds() * 1193180.0 / 65536.0)
	b.e.Mem.W32(bdaTicks, ticks)
}

func (b *BIOS) int08(e *hle.Env) error {
	t := e.Mem.R32(bdaTicks) + 1
	if t >= 0x1800B0 {
		t = 0
		e.Mem.W8(bdaMidnight, 1)
	}
	e.Mem.W32(bdaTicks, t)
	return nil
}

// --- keyboard ---------------------------------------------------------------

// PushKey queues a keystroke from the host.
func (b *BIOS) PushKey(k KeyEvent) { b.kbdQueue = append(b.kbdQueue, k) }

// PumpKeyboard moves the next queued event into port 60h and raises IRQ1.
// It returns true if an interrupt was raised.
//
// A key is held back while the type-ahead buffer (15 keys) is full, so that a
// burst from the host (a paste) is delivered in portions as the program reads,
// not dropped; Ctrl-Break always passes (it clears the buffer).
func (b *BIOS) PumpKeyboard() bool {
	if b.inFlight != nil || len(b.kbdQueue) == 0 {
		return false
	}
	k := b.kbdQueue[0]
	if b.bufFull() && !(k.Scan == scanBreak && k.Mods&ModCtrl != 0) {
		return false
	}
	b.inFlight = &k
	b.release = false
	b.port60 = k.Scan
	b.IRQ1()
	return true
}

// KeysPending reports whether host keystrokes are still queued.
func (b *BIOS) KeysPending() bool { return len(b.kbdQueue) > 0 || b.inFlight != nil }

func (b *BIOS) int09(e *hle.Env) error {
	if b.inFlight == nil {
		return nil
	}
	k := *b.inFlight
	if !b.release {
		e.Mem.W8(bdaKbdFlags, e.Mem.R8(bdaKbdFlags)&0xF0|k.Mods&0x0F)
		if k.Scan == scanBreak && k.Mods&ModCtrl != 0 {
			// Ctrl-Break (RBIL Int 09): clear the buffer, put the word 0000h
			// in it, call INT 1Bh, set bit 7 of 0040:0071. The INT 1Bh call
			// returns to the rest of the INT 09h stub.
			m := e.Mem
			m.W16(bdaKbdHead, m.R16(bdaKbdStart))
			m.W16(bdaKbdTail, m.R16(bdaKbdStart))
			b.bufPut(0)
			m.W8(bdaBreak, m.R8(bdaBreak)|0x80)
			e.CPU.Interrupt(0x1B)
		} else if k.Scan != 0 || k.ASCII != 0 {
			if !b.bufPut(k.Word()) {
				e.Note("keyboard buffer full")
			}
		}
		// Next: the break code.
		b.release = true
		b.port60 = k.Scan | 0x80
		b.IRQ1()
		return nil
	}
	e.Mem.W8(bdaKbdFlags, e.Mem.R8(bdaKbdFlags)&0xF0)
	b.inFlight = nil
	b.kbdQueue = b.kbdQueue[1:]
	return nil
}

func (b *BIOS) bufPut(w uint16) bool {
	m := b.e.Mem
	head, tail := m.R16(bdaKbdHead), m.R16(bdaKbdTail)
	start, end := m.R16(bdaKbdStart), m.R16(bdaKbdEnd)
	next := tail + 2
	if next >= end {
		next = start
	}
	if next == head {
		return false
	}
	m.W16(0x400+uint32(tail), w)
	m.W16(bdaKbdTail, next)
	return true
}

// bufFull reports whether the type-ahead buffer has no free slot.
func (b *BIOS) bufFull() bool {
	m := b.e.Mem
	next := m.R16(bdaKbdTail) + 2
	if next >= m.R16(bdaKbdEnd) {
		next = m.R16(bdaKbdStart)
	}
	return next == m.R16(bdaKbdHead)
}

// BufferedKeys returns how many keys wait in the type-ahead buffer (at most 15).
func (b *BIOS) BufferedKeys() int {
	m := b.e.Mem
	head, tail := int(m.R16(bdaKbdHead)), int(m.R16(bdaKbdTail))
	n := (tail - head) / 2
	if n < 0 {
		n += (int(m.R16(bdaKbdEnd)) - int(m.R16(bdaKbdStart))) / 2
	}
	return n
}

func (b *BIOS) bufPeek() (uint16, bool) {
	m := b.e.Mem
	head, tail := m.R16(bdaKbdHead), m.R16(bdaKbdTail)
	if head == tail {
		return 0, false
	}
	return m.R16(0x400 + uint32(head)), true
}

func (b *BIOS) bufGet() uint16 {
	m := b.e.Mem
	head := m.R16(bdaKbdHead)
	w := m.R16(0x400 + uint32(head))
	head += 2
	if head >= m.R16(bdaKbdEnd) {
		head = m.R16(bdaKbdStart)
	}
	m.W16(bdaKbdHead, head)
	return w
}

// PeekKey returns the first key of the BIOS buffer without removing it.
func (b *BIOS) PeekKey() (uint16, bool) { return b.bufPeek() }

// KeyAvailable reports whether the BIOS buffer holds a key.
func (b *BIOS) KeyAvailable() bool { _, ok := b.bufPeek(); return ok }

// ReadKey removes a key from the buffer (for DOS console input).
func (b *BIOS) ReadKey() (uint16, bool) {
	if _, ok := b.bufPeek(); !ok {
		return 0, false
	}
	return compat(b.bufGet()), true
}

// compat converts an enhanced-keyboard word for the original functions.
func compat(w uint16) uint16 {
	if byte(w) == 0xE0 && w>>8 != 0 {
		return w & 0xFF00
	}
	return w
}

func (b *BIOS) poll(e *hle.Env) {
	b.emptyPolls++
	if b.emptyPolls >= b.IdlePolls {
		b.emptyPolls = 0
		e.Idle()
	}
}

func (b *BIOS) int16(e *hle.Env) error {
	c := e.CPU
	switch ah := c.AH(); ah {
	case 0x00, 0x10:
		if _, ok := b.bufPeek(); !ok {
			e.Idle()
			return cpu.ErrRetry
		}
		b.emptyPolls = 0
		w := b.bufGet()
		if ah == 0x00 {
			w = compat(w)
		}
		c.R[cpu.AX] = w
	case 0x01, 0x11:
		w, ok := b.bufPeek()
		e.SetZF(!ok)
		if !ok {
			b.poll(e)
			return nil
		}
		b.emptyPolls = 0
		if ah == 0x01 {
			w = compat(w)
		}
		c.R[cpu.AX] = w
	case 0x02:
		c.SetAL(e.Mem.R8(bdaKbdFlags))
	case 0x12:
		c.SetAL(e.Mem.R8(bdaKbdFlags))
		c.SetAH(e.Mem.R8(bdaKbdFlags2))
	case 0x03:
		// Typematic rate: accepted, no effect.
	case 0x05:
		if b.bufPut(c.R[cpu.CX]) {
			c.SetAL(0)
		} else {
			c.SetAL(1)
		}
	default:
		return hle.Unsupported("INT 16h AH=%02Xh", ah)
	}
	return nil
}

// In and Out handle the keyboard controller ports.
func (b *BIOS) In(port uint16) (byte, bool) {
	switch port {
	case 0x60:
		return b.port60, true
	case 0x64:
		if b.inFlight != nil {
			return 0x1D, true
		}
		return 0x1C, true
	}
	return 0, false
}

// --- time ---------------------------------------------------------------------

func bcd(n int) byte { return byte(n/10<<4 | n%10) }

func (b *BIOS) int1A(e *hle.Env) error {
	c := e.CPU
	switch ah := c.AH(); ah {
	case 0x00:
		t := e.Mem.R32(bdaTicks)
		c.R[cpu.CX], c.R[cpu.DX] = uint16(t>>16), uint16(t)
		c.SetAL(e.Mem.R8(bdaMidnight))
		e.Mem.W8(bdaMidnight, 0)
	case 0x01:
		e.Mem.W32(bdaTicks, uint32(c.R[cpu.CX])<<16|uint32(c.R[cpu.DX]))
	case 0x02:
		t := e.Now()
		c.SetCH(bcd(t.Hour()))
		c.SetCL(bcd(t.Minute()))
		c.SetDH(bcd(t.Second()))
		c.SetDL(0)
		e.SetCF(false)
	case 0x04:
		t := e.Now()
		c.SetCH(bcd(t.Year() / 100))
		c.SetCL(bcd(t.Year() % 100))
		c.SetDH(bcd(int(t.Month())))
		c.SetDL(bcd(t.Day()))
		e.SetCF(false)
	case 0x03, 0x05:
		e.Note("setting the RTC is ignored")
		e.SetCF(false)
	default:
		return hle.Unsupported("INT 1Ah AH=%02Xh", ah)
	}
	return nil
}

// --- misc -----------------------------------------------------------------------

func (b *BIOS) int15(e *hle.Env) error {
	c := e.CPU
	switch ah := c.AH(); ah {
	case 0x88: // extended memory size
		c.R[cpu.AX] = 0
		e.SetCF(false)
	case 0x4F: // keyboard intercept: keep the scan code
		e.SetCF(true)
	case 0x90, 0x91: // OS hooks: device busy / interrupt complete
		c.SetAH(0)
		e.SetCF(false)
	case 0x24: // A20 gate
		switch c.AL() {
		case 0:
			e.Mem.SetA20(false)
		case 1:
			e.Mem.SetA20(true)
		case 2:
			if e.Mem.A20() {
				c.SetAL(1)
			} else {
				c.SetAL(0)
			}
		case 3:
			c.R[cpu.BX] = 3
		}
		c.SetAH(0)
		e.SetCF(false)
	case 0x86: // wait CX:DX microseconds
		d := time.Duration(uint32(c.R[cpu.CX])<<16|uint32(c.R[cpu.DX])) * time.Microsecond
		if d > time.Second {
			d = time.Second
		}
		time.Sleep(d)
		e.SetCF(false)
	case 0x10:
		// TopView/DESQview API. AX=1022h BX=0000h is GETVER (RBIL): "BX
		// nonzero, TopView or compatible loaded". None is, so BX stays 0.
		// The other AH=10h functions (AL=04h-12h, 26h-2Ah) make DESQview 2.x
		// pop up "Programming error": nothing to emulate, fail fast.
		if c.R[cpu.AX] != 0x1022 {
			return hle.Unsupported("INT 15h AX=%04Xh", c.R[cpu.AX])
		}
		c.R[cpu.BX] = 0
		e.Note("TopView GETVER: not loaded (BX=0)")
	case 0xC0, 0xC1, 0xC2, 0x87, 0x89, 0xE8, 0x41, 0x64:
		// Documented "function not supported" answer.
		c.SetAH(0x86)
		e.SetCF(true)
	default:
		return hle.Unsupported("INT 15h AH=%02Xh", ah)
	}
	return nil
}

// int33 behaves as if no mouse driver were installed: reset reports AX=0,
// every other call is a no-op.
func (b *BIOS) int33(e *hle.Env) error {
	c := e.CPU
	switch c.R[cpu.AX] {
	case 0x0000, 0x0021:
		c.R[cpu.AX] = 0
		c.R[cpu.BX] = 0
	default:
		e.Note("no mouse driver: ignored")
	}
	return nil
}
