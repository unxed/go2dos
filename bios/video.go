package bios

import (
	"strings"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
)

// BIOS data area addresses (linear).
const (
	bdaEquipment  = 0x410
	bdaMemSize    = 0x413
	bdaKbdFlags   = 0x417
	bdaKbdFlags2  = 0x418
	bdaKbdHead    = 0x41A
	bdaKbdTail    = 0x41C
	bdaVideoMode  = 0x449
	bdaCols       = 0x44A
	bdaPageSize   = 0x44C
	bdaPageStart  = 0x44E
	bdaCursorPos  = 0x450
	bdaCursorType = 0x460
	bdaActivePage = 0x462
	bdaCRTCBase   = 0x463
	bdaModeCtl    = 0x465
	bdaPalette    = 0x466
	bdaTicks      = 0x46C
	bdaMidnight   = 0x470
	bdaKbdStart   = 0x480
	bdaKbdEnd     = 0x482
	bdaRows       = 0x484
	bdaCharHeight = 0x485
	bdaEGAMisc    = 0x487
	bdaVGAFlags   = 0x489
	bdaKbdFlags3  = 0x496
)

// Video implements INT 10h and the CRTC/status ports of a VGA in text mode.
type Video struct {
	e       *hle.Env
	crtc    [32]byte
	crtcIdx byte
	crtcGen uint32
	reads   uint32 // input status reads, the clock of the retrace model
}

func (v *Video) b8(a uint32) byte       { return v.e.Mem.R8(a) }
func (v *Video) b16(a uint32) uint16    { return v.e.Mem.R16(a) }
func (v *Video) w8(a uint32, x byte)    { v.e.Mem.W8(a, x) }
func (v *Video) w16(a uint32, x uint16) { v.e.Mem.W16(a, x) }

func (v *Video) mode() byte { return v.b8(bdaVideoMode) }
func (v *Video) cols() int  { return int(v.b16(bdaCols)) }
func (v *Video) rows() int  { return int(v.b8(bdaRows)) + 1 }
func (v *Video) base() uint32 {
	if v.mode() == 7 {
		return 0xB0000
	}
	return 0xB8000
}

func (v *Video) pageAddr(page byte) uint32 {
	return v.base() + uint32(page)*uint32(v.b16(bdaPageSize))
}

func (v *Video) cursor(page byte) (row, col int) {
	p := v.b16(bdaCursorPos + uint32(page&7)*2)
	return int(p >> 8), int(p & 0xFF)
}

func (v *Video) setCursor(page byte, row, col int) {
	v.w16(bdaCursorPos+uint32(page&7)*2, uint16(row)<<8|uint16(col))
	if page == v.b8(bdaActivePage) {
		pos := uint16(uint32(page)*uint32(v.b16(bdaPageSize))/2) + uint16(row*v.cols()+col)
		v.crtc[0x0E], v.crtc[0x0F] = byte(pos>>8), byte(pos)
		v.crtcGen++
	}
}

func (v *Video) cellAddr(page byte, row, col int) uint32 {
	return v.pageAddr(page) + uint32((row*v.cols()+col)*2)
}

func (v *Video) setMode(m byte, clear bool) error {
	cols := 80
	switch m {
	case 0, 1:
		cols = 40
	case 2, 3, 7:
	default:
		return hle.Unsupported("video mode %02Xh (only text modes 0-3 and 7)", m)
	}
	v.w8(bdaVideoMode, m)
	v.w16(bdaCols, uint16(cols))
	if cols == 40 {
		v.w16(bdaPageSize, 0x0800)
	} else {
		v.w16(bdaPageSize, 0x1000)
	}
	v.w16(bdaPageStart, 0)
	v.w8(bdaActivePage, 0)
	v.w8(bdaRows, 24)
	v.w8(bdaCharHeight, 16)
	v.w16(bdaCRTCBase, 0x3D4)
	v.w8(bdaModeCtl, 0x29)
	v.w8(bdaPalette, 0x30)
	v.w8(bdaEGAMisc, 0x60)
	v.w8(bdaVGAFlags, 0x11)
	v.crtc[0x0C], v.crtc[0x0D] = 0, 0
	shape := uint16(0x0607)
	if m == 7 {
		shape = 0x0B0C
	}
	v.w16(bdaCursorType, shape)
	v.crtc[0x0A], v.crtc[0x0B] = byte(shape>>8), byte(shape)
	for p := byte(0); p < 8; p++ {
		v.setCursor(p, 0, 0)
	}
	if clear {
		for a := v.base(); a < v.base()+0x8000; a += 2 {
			v.e.Mem.W16(a, 0x0720)
		}
	}
	v.crtcGen++
	return nil
}

func (v *Video) scroll(up bool, lines, attr byte, top, left, bottom, right int) {
	cols, rows := v.cols(), v.rows()
	if right >= cols {
		right = cols - 1
	}
	if bottom >= rows {
		bottom = rows - 1
	}
	if top > bottom || left > right {
		return
	}
	h := bottom - top + 1
	n := int(lines)
	if n == 0 || n > h {
		n = h
	}
	page := v.b8(bdaActivePage)
	blank := uint16(attr)<<8 | 0x20
	for i := 0; i < h; i++ {
		row := top + i
		src := row + n
		if !up {
			row = bottom - i
			src = row - n
		}
		for col := left; col <= right; col++ {
			val := blank
			if (up && src <= bottom) || (!up && src >= top) {
				val = v.e.Mem.R16(v.cellAddr(page, src, col))
			}
			v.e.Mem.W16(v.cellAddr(page, row, col), val)
		}
	}
}

// Teletype writes one character with BIOS TTY semantics (INT 10h/0Eh).
func (v *Video) Teletype(ch byte, page byte) {
	row, col := v.cursor(page)
	cols, rows := v.cols(), v.rows()
	switch ch {
	case 7:
		return
	case 8:
		if col > 0 {
			col--
		}
	case 10:
		row++
	case 13:
		col = 0
	default:
		a := v.cellAddr(page, row, col)
		v.e.Mem.W8(a, ch)
		col++
		if col >= cols {
			col = 0
			row++
		}
	}
	if row >= rows {
		attr := v.e.Mem.R8(v.cellAddr(page, rows-1, 0) + 1)
		if attr == 0 {
			attr = 7
		}
		v.scroll(true, 1, attr, 0, 0, rows-1, cols-1)
		row = rows - 1
	}
	v.setCursor(page, row, col)
}

// ActivePage returns the displayed page.
func (v *Video) ActivePage() byte { return v.b8(bdaActivePage) }

func (v *Video) int10(e *hle.Env) error {
	c := e.CPU
	switch ah := c.AH(); ah {
	case 0x00:
		return v.setMode(c.AL()&0x7F, c.AL()&0x80 == 0)
	case 0x01:
		v.w16(bdaCursorType, c.R[cpu.CX])
		v.crtc[0x0A], v.crtc[0x0B] = c.CH(), c.CL()
		v.crtcGen++
	case 0x02:
		v.setCursor(c.BH(), int(c.DH()), int(c.DL()))
	case 0x03:
		row, col := v.cursor(c.BH())
		c.R[cpu.DX] = uint16(row)<<8 | uint16(col)
		c.R[cpu.CX] = v.b16(bdaCursorType)
	case 0x05:
		p := c.AL() & 7
		v.w8(bdaActivePage, p)
		start := uint16(uint32(p) * uint32(v.b16(bdaPageSize)))
		v.w16(bdaPageStart, start)
		v.crtc[0x0C], v.crtc[0x0D] = byte(start/2>>8), byte(start/2)
		row, col := v.cursor(p)
		v.setCursor(p, row, col)
	case 0x06, 0x07:
		v.scroll(ah == 6, c.AL(), c.BH(), int(c.CH()), int(c.CL()), int(c.DH()), int(c.DL()))
	case 0x08:
		row, col := v.cursor(c.BH())
		c.R[cpu.AX] = e.Mem.R16(v.cellAddr(c.BH(), row, col))
	case 0x09, 0x0A:
		page := c.BH()
		row, col := v.cursor(page)
		a := v.cellAddr(page, row, col)
		end := v.pageAddr(page) + uint32(v.cols()*v.rows()*2)
		for i := 0; i < int(c.R[cpu.CX]) && a < end; i++ {
			e.Mem.W8(a, c.AL())
			if ah == 0x09 {
				e.Mem.W8(a+1, c.BL())
			}
			a += 2
		}
	case 0x0E:
		v.Teletype(c.AL(), v.b8(bdaActivePage))
	case 0x0F:
		c.SetAL(v.mode())
		c.SetAH(byte(v.cols()))
		c.SetBH(v.b8(bdaActivePage))
	case 0x10:
		// Palette and blink control do not affect the text; accepted and ignored.
	case 0x11:
		if c.AL() != 0x30 {
			return hle.Unsupported("INT 10h AX=%04Xh (font functions)", c.R[cpu.AX])
		}
		c.R[cpu.CX] = uint16(v.b8(bdaCharHeight))
		c.SetDL(byte(v.rows() - 1))
		c.R[cpu.BP] = 0 // no font tables are emulated
		c.SetSeg(cpu.ES, 0xC000)
	case 0x12:
		switch c.BL() {
		case 0x10: // EGA information: color, 256K
			c.SetBH(0)
			c.SetBL(3)
			c.R[cpu.CX] = 0x0009
		case 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36:
			c.SetAL(0x12) // accepted, no effect on the emulated text screen
		default:
			return hle.Unsupported("INT 10h AH=12h BL=%02Xh", c.BL())
		}
	case 0x13:
		v.writeString(e)
	case 0x1A:
		if c.AL() == 0 {
			c.SetAL(0x1A)
			c.R[cpu.BX] = 0x0008 // VGA color
		} else {
			return hle.Unsupported("INT 10h AX=%04Xh", c.R[cpu.AX])
		}
	case 0x1B, 0x4F, 0xFE, 0xEF, 0xFA, 0xCC:
		// Probes for VGA state, VESA, multitasker shadow buffers, Hercules and
		// TSRs: leaving the registers unchanged is the "not present" answer.
		e.Note("probe: not present")
	default:
		return hle.Unsupported("INT 10h AH=%02Xh", ah)
	}
	return nil
}

func (v *Video) writeString(e *hle.Env) {
	c := e.CPU
	page, mode := c.BH(), c.AL()
	oldRow, oldCol := v.cursor(page)
	v.setCursor(page, int(c.DH()), int(c.DL()))
	src := hle.Ptr(c.S[cpu.ES].Sel, c.R[cpu.BP])
	attr := c.BL()
	for i := 0; i < int(c.R[cpu.CX]); i++ {
		ch := e.Mem.R8(src)
		src++
		if mode&2 != 0 {
			attr = e.Mem.R8(src)
			src++
		}
		row, col := v.cursor(page)
		if ch == 7 || ch == 8 || ch == 10 || ch == 13 {
			v.Teletype(ch, page)
			continue
		}
		e.Mem.W16(v.cellAddr(page, row, col), uint16(attr)<<8|uint16(ch))
		v.Teletype(0xFF, page) // advance the cursor
		e.Mem.W16(v.cellAddr(page, row, col), uint16(attr)<<8|uint16(ch))
	}
	if mode&1 == 0 {
		v.setCursor(page, oldRow, oldCol)
	}
}

// CRTC and status port access, called by the machine's port dispatcher.

func (v *Video) In(port uint16) (byte, bool) {
	switch port {
	case 0x3D4, 0x3B4:
		return v.crtcIdx, true
	case 0x3D5, 0x3B5:
		return v.crtc[v.crtcIdx&31], true
	case 0x3DA, 0x3BA:
		// Retrace is modelled per status read, not wall time, so waiting
		// loops end quickly and deterministically: each "frame" is 256 reads,
		// the first 24 in vertical retrace (bits 0 and 3), the rest with a
		// horizontal retrace (bit 0) on every other pair of reads.
		n := v.reads % 256
		v.reads++
		if n < 24 {
			return 0x09, true
		}
		return byte(n>>1) & 1, true
	case 0x3CC:
		return 0x67, true
	}
	return 0, false
}

func (v *Video) Out(port uint16, b byte) bool {
	switch port {
	case 0x3D4, 0x3B4:
		v.crtcIdx = b
	case 0x3D5, 0x3B5:
		v.crtc[v.crtcIdx&31] = b
		v.crtcGen++
	case 0x3C0, 0x3C2, 0x3C4, 0x3C5, 0x3C6, 0x3C7, 0x3C8, 0x3C9, 0x3CE, 0x3CF, 0x3D8, 0x3D9:
		// Attribute/sequencer/DAC/graphics/CGA mode registers: no effect on text.
	default:
		return false
	}
	return true
}

// Cell is one character cell of the text screen.
type Cell struct {
	Ch   byte // byte in video memory
	Attr byte // attribute byte
	Rune rune // Unicode for Ch in the current code page
}

// Screen is a snapshot of the text screen.
type Screen struct {
	Mode          byte
	Cols, Rows    int
	Cells         []Cell
	CursorX       int
	CursorY       int
	CursorVisible bool
	Version       uint32 // changes whenever video memory or CRTC state changes
}

// TextMode reports whether the snapshot holds text.
func (s *Screen) TextMode() bool { return s.Cols > 0 }

// Line returns row y as a string with trailing blanks removed.
func (s *Screen) Line(y int) string {
	var b strings.Builder
	for x := 0; x < s.Cols; x++ {
		b.WriteRune(s.Cells[y*s.Cols+x].Rune)
	}
	return strings.TrimRight(b.String(), " ")
}

// Text returns the whole screen, one line per row.
func (s *Screen) Text() string {
	lines := make([]string, s.Rows)
	for y := range lines {
		lines[y] = s.Line(y)
	}
	return strings.Join(lines, "\n")
}

// Region returns the rectangle (x, y, w, h) as text.
func (s *Screen) Region(x, y, w, h int) string {
	var lines []string
	for r := y; r < y+h && r < s.Rows; r++ {
		var b strings.Builder
		for c := x; c < x+w && c < s.Cols; c++ {
			b.WriteRune(s.Cells[r*s.Cols+c].Rune)
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	return strings.Join(lines, "\n")
}

// Snapshot captures the visible text screen from video memory, the BIOS
// data area and the CRTC registers.
func (v *Video) Snapshot() *Screen {
	s := &Screen{Mode: v.mode()}
	s.Version = v.e.Mem.RangeWrites(0xB0000, 0xC0000) + v.crtcGen
	switch s.Mode {
	case 0, 1, 2, 3, 7:
	default:
		return s
	}
	s.Cols, s.Rows = v.cols(), v.rows()
	start := uint32(v.crtc[0x0C])<<8 | uint32(v.crtc[0x0D])
	base := v.base() + start*2
	s.Cells = make([]Cell, s.Cols*s.Rows)
	for i := range s.Cells {
		w := v.e.Mem.R16(base + uint32(i*2))
		ch := byte(w)
		s.Cells[i] = Cell{Ch: ch, Attr: byte(w >> 8), Rune: v.e.CP.ScreenRune(ch)}
	}
	pos := int(uint32(v.crtc[0x0E])<<8|uint32(v.crtc[0x0F])) - int(start)
	if pos >= 0 && pos < s.Cols*s.Rows {
		s.CursorX, s.CursorY = pos%s.Cols, pos/s.Cols
		s.CursorVisible = v.crtc[0x0A]&0x20 == 0 && v.crtc[0x0A]&0x1F <= v.crtc[0x0B]&0x1F
	}
	return s
}
