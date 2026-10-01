// Package mem implements the address space: conventional memory, the HMA,
// the video window and the BIOS ROM, with A20 control and per-page write
// counters (the screen uses them to detect changes; a JIT will use them to
// invalidate translated code).
package mem

const (
	// Size covers 1 MiB plus the HMA.
	Size     = 0x110000
	PageBits = 12
	pages    = Size >> PageBits

	romStart  = 0xF0000
	romEnd    = 0x100000
	holeStart = 0xC8000 // option ROM / UMB window: unmapped in the PoC; below it the 64 KiB video window B8000-C7FFF
)

// Memory is the emulated physical memory.
type Memory struct {
	RAM    []byte
	a20    bool
	writes [pages]uint32

	// Watch, if set, is called for writes to addresses in Watched
	// (a diagnostic aid; it costs one map lookup per write when enabled).
	Watched map[uint32]bool
	Watch   func(addr uint32, old, v byte)
}

// New returns zeroed memory with A20 disabled.
func New() *Memory {
	return &Memory{RAM: make([]byte, Size)}
}

// SetA20 enables or disables the A20 line.
func (m *Memory) SetA20(on bool) { m.a20 = on }

// A20 reports the state of the A20 line.
func (m *Memory) A20() bool { return m.a20 }

func (m *Memory) addr(a uint32) uint32 {
	if !m.a20 {
		return a & 0xFFFFF
	}
	return a
}

// Read8 implements cpu.Bus.
func (m *Memory) Read8(a uint32) byte {
	a = m.addr(a)
	if a >= Size {
		return 0xFF
	}
	if a >= holeStart && a < romStart {
		return 0xFF
	}
	return m.RAM[a]
}

// Write8 implements cpu.Bus. Writes to ROM and unmapped areas are ignored.
func (m *Memory) Write8(a uint32, v byte) {
	a = m.addr(a)
	if a >= Size || (a >= holeStart && a < romEnd) {
		return
	}
	if m.Watched != nil && m.Watched[a] {
		m.Watch(a, m.RAM[a], v)
	}
	m.RAM[a] = v
	m.writes[a>>PageBits]++
}

// Poke writes without the ROM check; used to set up the firmware.
func (m *Memory) Poke(a uint32, b ...byte) {
	copy(m.RAM[a:], b)
	for p := a >> PageBits; p <= (a+uint32(len(b)))>>PageBits && p < pages; p++ {
		m.writes[p]++
	}
}

// PageWrites returns the write counter of the page containing address a.
func (m *Memory) PageWrites(a uint32) uint32 { return m.writes[a>>PageBits] }

// RangeWrites sums the write counters over [start, end).
func (m *Memory) RangeWrites(start, end uint32) uint32 {
	var s uint32
	for p := start >> PageBits; p < (end+(1<<PageBits)-1)>>PageBits; p++ {
		s += m.writes[p]
	}
	return s
}

// Helpers for HLE code (linear addresses, no A20 masking needed below 1 MiB).

func (m *Memory) R8(a uint32) byte { return m.Read8(a) }
func (m *Memory) R16(a uint32) uint16 {
	return uint16(m.Read8(a)) | uint16(m.Read8(a+1))<<8
}
func (m *Memory) R32(a uint32) uint32 { return uint32(m.R16(a)) | uint32(m.R16(a+2))<<16 }
func (m *Memory) W8(a uint32, v byte) { m.Write8(a, v) }
func (m *Memory) W16(a uint32, v uint16) {
	m.Write8(a, byte(v))
	m.Write8(a+1, byte(v>>8))
}
func (m *Memory) W32(a uint32, v uint32) {
	m.W16(a, uint16(v))
	m.W16(a+2, uint16(v>>16))
}

// Bytes copies n bytes starting at a.
func (m *Memory) Bytes(a uint32, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = m.Read8(a + uint32(i))
	}
	return b
}

// SetBytes copies b to address a.
func (m *Memory) SetBytes(a uint32, b []byte) {
	for i, v := range b {
		m.Write8(a+uint32(i), v)
	}
}

// ASCIIZ reads a zero-terminated string of at most max bytes.
func (m *Memory) ASCIIZ(a uint32, max int) []byte {
	var b []byte
	for i := 0; i < max; i++ {
		v := m.Read8(a + uint32(i))
		if v == 0 {
			break
		}
		b = append(b, v)
	}
	return b
}

// Lin converts a real-mode segment:offset pair to a linear address.
func Lin(seg, off uint16) uint32 { return uint32(seg)<<4 + uint32(off) }
