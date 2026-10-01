package dos

import (
	"encoding/binary"
	"fmt"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// parentFrame is what EXEC saves to resume the parent when the child ends.
type parentFrame struct {
	psp            uint16
	ss, sp         uint16 // stack with the parent's INT 21h frame on top
	dtaSeg, dtaOff uint16
}

// image is a program file ready to be placed in memory.
type image struct {
	data  []byte
	isEXE bool
	hdr   exeHeader
	path  string // canonical DOS path with drive
}

func (d *DOS) readImage(name []byte) (*image, uint16) {
	drive, dp, errc := d.fs.canon(name, false)
	if errc != 0 {
		return nil, errc
	}
	host, _, errc := d.fs.resolve(drive, dp, false)
	if errc != 0 {
		return nil, errc
	}
	data, err := d.fs.readFile(host)
	if err != nil {
		return nil, osErr(err)
	}
	im := &image{data: data, path: string(rune('A'+drive)) + ":" + dp}
	im.isEXE = len(data) >= 0x1C && (string(data[:2]) == "MZ" || string(data[:2]) == "ZM")
	if im.isEXE {
		im.hdr = parseEXE(data)
	}
	return im, 0
}

// place copies the load module to segment start and applies relocations
// with the given segment fixup.
func (d *DOS) place(im *image, start, fixup uint16) uint16 {
	if !im.isEXE {
		d.e.Mem.SetBytes(mem.Lin(start, 0), im.data)
		return 0
	}
	h := im.hdr
	if int(h.HeaderParas)*16 > len(im.data) {
		return 0x0B // invalid format
	}
	body := im.data[int(h.HeaderParas)*16:]
	if n := int(h.loadParas()) * 16; n < len(body) {
		body = body[:n]
	}
	d.e.Mem.SetBytes(mem.Lin(start, 0), body)
	for i := 0; i < int(h.NReloc); i++ {
		o := int(h.RelocOff) + i*4
		if o+4 > len(im.data) {
			return 0x0B
		}
		off := binary.LittleEndian.Uint16(im.data[o:])
		seg := binary.LittleEndian.Uint16(im.data[o+2:])
		a := mem.Lin(start+seg, off)
		d.e.Mem.W16(a, d.e.Mem.R16(a)+fixup)
	}
	return 0
}

// envBlock returns the strings part (up to and including the double NUL)
// of an environment segment.
func (d *DOS) envBlock(seg uint16) []byte {
	m := d.e.Mem
	a := mem.Lin(seg, 0)
	var out []byte
	for i := uint32(0); i < 0x8000; i++ {
		b := m.R8(a + i)
		out = append(out, b)
		if b == 0 && (i == 0 || m.R8(a+i-1) == 0) {
			return out
		}
	}
	return append(out, 0)
}

func (d *DOS) exec(e *hle.Env) error {
	c := e.CPU
	switch c.AL() {
	case 0x00:
	case 0x03:
		return d.loadOverlay(e)
	default:
		return hle.Unsupported("INT 21h AX=%04Xh (EXEC subfunction)", c.R[cpu.AX])
	}
	name := e.Mem.ASCIIZ(e.DSDX(), 128)
	pb := mem.Lin(e.Seg(cpu.ES), c.R[cpu.BX])
	m := e.Mem
	envSeg := m.R16(pb)
	tailPtr := m.R32(pb + 2)
	fcb1, fcb2 := m.R32(pb+6), m.R32(pb+10)
	im, errc := d.readImage(name)
	if errc != 0 {
		d.fail(e, errc)
		return nil
	}
	e.Note("%s", im.path)

	// Environment: a copy of the given or the parent's block plus the program path.
	if envSeg == 0 {
		envSeg = m.R16(mem.Lin(d.psp, 0x2C))
	}
	env := d.envBlock(envSeg)
	env = append(env, 1, 0)
	env = append(env, []byte(im.path)...)
	env = append(env, 0)
	newEnv, _, errc := d.alloc(uint16((len(env)+15)/16), 0xFFFF)
	if errc != 0 {
		d.fail(e, errNoMem)
		return nil
	}
	m.SetBytes(mem.Lin(newEnv, 0), env)

	need := uint16(0x1000)
	if im.isEXE {
		need = im.hdr.loadParas() + 0x10 + im.hdr.MinAlloc
	}
	_, largest, _ := d.alloc(0xFFFF, 0)
	if largest < need {
		d.free(newEnv)
		d.fail(e, errNoMem)
		return nil
	}
	size := largest
	if im.isEXE && im.hdr.MaxAlloc != 0xFFFF {
		if want := im.hdr.loadParas() + 0x10 + im.hdr.MaxAlloc; want < size && want >= need {
			size = want
		}
	}
	psp, _, _ := d.alloc(size, 0xFFFF)
	d.writeMCB(psp-1, d.readMCB(psp-1).sig, psp, size, baseName(im.path))
	d.writeMCB(newEnv-1, d.readMCB(newEnv-1).sig, psp, d.readMCB(newEnv-1).size, "")

	// Parent state, resumed on termination.
	parent := d.psp
	d.frames = append(d.frames, parentFrame{psp: parent, ss: e.Seg(cpu.SS), sp: c.R[cpu.SP], dtaSeg: d.dtaSeg, dtaOff: d.dtaOff})
	m.W16(mem.Lin(parent, 0x2E), c.R[cpu.SP])
	m.W16(mem.Lin(parent, 0x30), e.Seg(cpu.SS))

	d.buildPSP(psp, psp+size, newEnv, parent, "")
	a := mem.Lin(psp, 0)
	retCS, retIP := e.Caller()
	m.W16(a+0x0A, retIP)
	m.W16(a+0x0C, retCS)
	// Command tail and FCBs come from the parameter block.
	tail := mem.Lin(uint16(tailPtr>>16), uint16(tailPtr))
	n := m.R8(tail)
	if n > 126 {
		n = 126
	}
	m.SetBytes(a+0x80, m.Bytes(tail, int(n)+2))
	m.W8(a+0x81+uint32(n), 0x0D)
	m.SetBytes(a+0x5C, m.Bytes(mem.Lin(uint16(fcb1>>16), uint16(fcb1)), 16))
	m.SetBytes(a+0x6C, m.Bytes(mem.Lin(uint16(fcb2>>16), uint16(fcb2)), 20))
	// Inherit handles (except those opened with the no-inherit bit).
	pj, psize := d.jft()
	for h := 0; h < 20; h++ {
		idx := byte(0xFF)
		if h < psize {
			idx = m.R8(pj + uint32(h))
		}
		if idx != 0xFF && int(idx) < len(d.sft) && d.sft[idx] != nil && d.sft[idx].mode&0x80 == 0 {
			d.sft[idx].refs++
		} else {
			idx = 0xFF
		}
		m.W8(a+0x18+uint32(h), idx)
	}

	d.psp = psp
	d.setDTA(psp, 0x80)
	if im.isEXE {
		start := psp + 0x10
		if errc := d.place(im, start, start); errc != 0 {
			return hle.Unsupported("EXEC: bad EXE relocation table in %s", im.path)
		}
		c.SetSeg(cpu.CS, start+im.hdr.CS)
		c.IP = im.hdr.IP
		c.SetSeg(cpu.SS, start+im.hdr.SS)
		c.R[cpu.SP] = im.hdr.SP
	} else {
		if len(im.data) > 0xFF00 {
			d.fail(e, 0x0B)
			return nil
		}
		d.place(im, psp+0x10, 0)
		sp := uint16(0xFFFE)
		if uint32(size)*16 < 0x10000 {
			sp = uint16(uint32(size)*16 - 2)
		}
		m.W16(mem.Lin(psp, sp), 0)
		c.SetSeg(cpu.CS, psp)
		c.IP = 0x100
		c.SetSeg(cpu.SS, psp)
		c.R[cpu.SP] = sp
	}
	c.SetSeg(cpu.DS, psp)
	c.SetSeg(cpu.ES, psp)
	c.R[cpu.AX] = d.fcbDriveStatus(psp)
	c.SetFlags(c.Flags | cpu.FlagIF)
	// Diagnostics: everything the child is given, and where it starts.
	e.Note("%s env=%04X tail=%04X:%04X [% X] fcb1=%04X:%04X fcb2=%04X:%04X psp=%04X size=%04X parent=%04X ret=%04X:%04X start=%04X:%04X ss:sp=%04X:%04X",
		im.path, envSeg, tailPtr>>16, tailPtr&0xFFFF, m.Bytes(tail, int(n)+2),
		fcb1>>16, fcb1&0xFFFF, fcb2>>16, fcb2&0xFFFF, psp, size, parent, retCS, retIP,
		c.S[cpu.CS].Sel, c.IP, c.S[cpu.SS].Sel, c.R[cpu.SP])
	if e.Event != nil {
		e.Event("exec-start")
	}
	return nil
}

func (d *DOS) loadOverlay(e *hle.Env) error {
	c := e.CPU
	im, errc := d.readImage(e.Mem.ASCIIZ(e.DSDX(), 128))
	if errc != 0 {
		d.fail(e, errc)
		return nil
	}
	e.Note("overlay %s", im.path)
	pb := mem.Lin(e.Seg(cpu.ES), c.R[cpu.BX])
	seg, fix := e.Mem.R16(pb), e.Mem.R16(pb+2)
	if errc := d.place(im, seg, fix); errc != 0 {
		d.fail(e, errc)
		return nil
	}
	d.ok(e)
	return nil
}

// endChild resumes the parent after a child process terminated. It returns
// false if the terminating process is the top-level program.
func (d *DOS) endChild(code byte, keep bool) bool {
	if len(d.frames) == 0 {
		return false
	}
	child := d.psp
	m := d.e.Mem
	retIP, retCS := m.R16(mem.Lin(child, 0x0A)), m.R16(mem.Lin(child, 0x0C))
	// Restore the INT 22h/23h/24h vectors saved in the child's PSP.
	for i, v := range []byte{0x22, 0x23, 0x24} {
		off := m.R16(mem.Lin(child, 0x0A+uint16(i*4)))
		seg := m.R16(mem.Lin(child, 0x0C+uint16(i*4)))
		d.e.SetVector(v, seg, off)
	}
	if !keep {
		d.closeAllHandles(child)
		d.freeOwnedBy(child)
	}
	f := d.frames[len(d.frames)-1]
	d.frames = d.frames[:len(d.frames)-1]
	d.psp = f.psp
	d.setDTA(f.dtaSeg, f.dtaOff)
	c := d.e.CPU
	// Return to the parent as the IRET of its INT 21h would, but to the
	// terminate address and with CF clear.
	c.SetSeg(cpu.SS, f.ss)
	flags := c.Read16(cpu.SS, f.sp+4)
	c.R[cpu.SP] = f.sp + 6
	c.SetSeg(cpu.CS, retCS)
	c.IP = retIP
	c.SetFlags(flags &^ cpu.FlagCF)
	d.e.Trace.Stream("exec", fmt.Sprintf("return: child psp=%04X ended (keep=%v), parent psp=%04X resumes at %04X:%04X ss:sp=%04X:%04X",
		child, keep, f.psp, retCS, retIP, f.ss, c.R[cpu.SP]))
	if d.e.Event != nil {
		d.e.Event("exec-return")
	}
	return true
}

// closeAllHandles closes every handle of a process (used when it ends).
func (d *DOS) closeAllHandles(psp uint16) {
	saved := d.psp
	d.psp = psp
	_, size := d.jft()
	for h := 0; h < size; h++ {
		if _, errc := d.handle(uint16(h)); errc == 0 {
			d.closeHandle(uint16(h))
		}
	}
	d.psp = saved
}
