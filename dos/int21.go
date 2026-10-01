package dos

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

func (d *DOS) ok(e *hle.Env) { e.SetCF(false) }

func (d *DOS) fail(e *hle.Env, code uint16) {
	d.lastErr = code
	e.CPU.R[cpu.AX] = code
	e.SetCF(true)
}

// result sets CF/AX from an error code (0 = success).
func (d *DOS) result(e *hle.Env, code uint16) {
	if code != 0 {
		d.fail(e, code)
		return
	}
	d.ok(e)
}

func (d *DOS) str(seg, off uint16) []byte { return d.e.Mem.ASCIIZ(mem.Lin(seg, off), 128) }

func (d *DOS) int21(e *hle.Env) error {
	c := e.CPU
	ah := c.AH()
	if d.breakFlag != 0 && ah > 0x0C && d.statChk(e) { // BREAK ON: ^C is checked on every call
		return nil
	}
	if d.guard(e) {
		return nil
	}
	switch {
	case ah >= 0x01 && ah <= 0x0C:
		return d.charFunc(e, ah)
	}
	switch ah {
	case 0x00:
		d.terminate(0, false)
	case 0x0D: // disk reset
	case 0x0E:
		if l := c.DL(); l < 26 && d.fs.drives[l] != "" {
			d.fs.cur = int(l)
		}
		c.SetAL(lastDrive)
	case 0x19:
		c.SetAL(byte(d.fs.cur))
	case 0x1A:
		d.setDTA(e.Seg(cpu.DS), c.R[cpu.DX])
	case 0x2F:
		c.SetSeg(cpu.ES, d.dtaSeg)
		c.R[cpu.BX] = d.dtaOff
	case 0x25:
		e.SetVector(c.AL(), e.Seg(cpu.DS), c.R[cpu.DX])
	case 0x35:
		seg, off := e.Vector(c.AL())
		c.SetSeg(cpu.ES, seg)
		c.R[cpu.BX] = off
	case 0x29:
		a := mem.Lin(e.Seg(cpu.DS), c.R[cpu.SI])
		src := e.Mem.Bytes(a, 128)
		n, al := d.parseFCB(src, mem.Lin(e.Seg(cpu.ES), c.R[cpu.DI]), c.AL())
		c.R[cpu.SI] += uint16(n)
		c.SetAL(al)
	case 0x2A:
		t := e.Now()
		c.R[cpu.CX] = uint16(t.Year())
		c.SetDH(byte(t.Month()))
		c.SetDL(byte(t.Day()))
		c.SetAL(byte(t.Weekday()))
	case 0x2B: // set date: validated like DOS, then ignored (the host clock rules)
		y, mo, dd := int(c.R[cpu.CX]), int(c.DH()), int(c.DL())
		valid := y >= 1980 && y <= 2099 && mo >= 1 && mo <= 12 && dd >= 1 &&
			dd <= time.Date(y, time.Month(mo)+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if valid {
			e.Note("setting the date is ignored")
			c.SetAL(0)
		} else {
			c.SetAL(0xFF)
		}
	case 0x2D: // set time: validated, then ignored
		if c.CH() < 24 && c.CL() < 60 && c.DH() < 60 && c.DL() < 100 {
			e.Note("setting the time is ignored")
			c.SetAL(0)
		} else {
			c.SetAL(0xFF)
		}
	case 0x2C:
		t := e.Now()
		c.SetCH(byte(t.Hour()))
		c.SetCL(byte(t.Minute()))
		c.SetDH(byte(t.Second()))
		c.SetDL(byte(t.Nanosecond() / 10000000))
	case 0x2E:
		d.verify = c.AL() & 1
	case 0x54:
		c.SetAL(d.verify)
	case 0x30:
		c.SetAL(VersionMajor)
		c.SetAH(VersionMinor)
		c.R[cpu.BX] = 0xFF00
		c.R[cpu.CX] = 0
	case 0x33:
		switch c.AL() {
		case 0:
			c.SetDL(d.breakFlag)
		case 1:
			d.breakFlag = c.DL() & 1
		case 5:
			c.SetDL(byte(d.fs.cur + 1))
		case 6:
			c.R[cpu.BX] = VersionMinor<<8 | VersionMajor
			c.R[cpu.DX] = 0
		default:
			return hle.Unsupported("INT 21h AX=%04Xh", c.R[cpu.AX])
		}
	case 0x32:
		d.getDPB(e)
	case 0x34:
		c.SetSeg(cpu.ES, dataSeg)
		c.R[cpu.BX] = offInDOS
	case 0x36:
		drive := int(c.DL())
		if drive == 0 {
			drive = d.fs.cur + 1
		}
		if drive > 26 || d.fs.drives[drive-1] == "" {
			c.R[cpu.AX] = 0xFFFF
			return nil
		}
		// Reported geometry is fixed (32 KiB clusters, 1 GiB free of 2 GiB).
		e.Note("FAKE free space")
		c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX] = 64, 0x8000, 512, 0xFFFF
	case 0x37:
		switch c.AL() {
		case 0:
			c.SetDL('/')
			c.SetAL(0)
		case 1:
			c.SetAL(0)
		default:
			c.SetAL(0xFF)
		}
	case 0x38:
		if c.R[cpu.DX] == 0xFFFF { // set country: not supported
			d.fail(e, 2)
			return nil
		}
		d.countryInfo(e.DSDX())
		c.R[cpu.BX] = 1
		d.ok(e)
	case 0x39:
		d.result(e, d.mkdir(e.Mem.ASCIIZ(e.DSDX(), 128)))
	case 0x3A:
		d.result(e, d.rmdir(e.Mem.ASCIIZ(e.DSDX(), 128)))
	case 0x3B:
		d.result(e, d.chdir(e.Mem.ASCIIZ(e.DSDX(), 128)))
	case 0x3C, 0x5B:
		h, errc := d.open(e.Mem.ASCIIZ(e.DSDX(), 128), 2, true, true, ah == 0x5B)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.R[cpu.AX] = h
		d.ok(e)
	case 0x3D:
		h, errc := d.open(e.Mem.ASCIIZ(e.DSDX(), 128), c.AL(), false, false, false)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.R[cpu.AX] = h
		d.ok(e)
	case 0x3E:
		d.result(e, d.closeHandle(c.R[cpu.BX]))
	case 0x3F:
		buf := make([]byte, c.R[cpu.CX])
		n, errc := d.read(c.R[cpu.BX], buf)
		if errc == 0xFFFF {
			e.Idle()
			return cpu.ErrRetry
		}
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		e.Mem.SetBytes(e.DSDX(), buf[:n])
		c.R[cpu.AX] = uint16(n)
		d.ok(e)
	case 0x40:
		buf := e.Mem.Bytes(e.DSDX(), int(c.R[cpu.CX]))
		n, errc := d.write(c.R[cpu.BX], buf)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.R[cpu.AX] = uint16(n)
		d.ok(e)
	case 0x41:
		d.result(e, d.unlink(e.Mem.ASCIIZ(e.DSDX(), 128)))
	case 0x42:
		of, errc := d.handle(c.R[cpu.BX])
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		if of.f == nil {
			c.R[cpu.AX], c.R[cpu.DX] = 0, 0
			d.ok(e)
			return nil
		}
		off := int64(int32(uint32(c.R[cpu.CX])<<16 | uint32(c.R[cpu.DX])))
		if c.AL() == 0 {
			off = int64(uint32(c.R[cpu.CX])<<16 | uint32(c.R[cpu.DX]))
		}
		if c.AL() > 2 {
			d.fail(e, errInvalidFunc)
			return nil
		}
		pos, err := of.f.Seek(off, int(c.AL()))
		if err != nil {
			d.fail(e, 0x19)
			return nil
		}
		c.R[cpu.AX], c.R[cpu.DX] = uint16(pos), uint16(pos>>16)
		d.ok(e)
	case 0x43:
		d.attrib(e)
	case 0x44:
		return d.ioctl(e)
	case 0x45:
		of, errc := d.handle(c.R[cpu.BX])
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		h, errc := d.dupHandle(of)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.R[cpu.AX] = h
		d.ok(e)
	case 0x46:
		of, errc := d.handle(c.R[cpu.BX])
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		if _, e2 := d.handle(c.R[cpu.CX]); e2 == 0 {
			d.closeHandle(c.R[cpu.CX])
		}
		a, size := d.jft()
		if int(c.R[cpu.CX]) >= size {
			d.fail(e, errBadHandle)
			return nil
		}
		idx := e.Mem.R8(a + uint32(c.R[cpu.BX]))
		e.Mem.W8(a+uint32(c.R[cpu.CX]), idx)
		of.refs++
		d.ok(e)
	case 0x47:
		drive := int(c.DL())
		if drive == 0 {
			drive = d.fs.cur + 1
		}
		if drive > 26 || d.fs.drives[drive-1] == "" {
			d.fail(e, errBadDrive)
			return nil
		}
		p := strings.TrimPrefix(d.fs.cwd[drive-1], `\`)
		e.Mem.SetBytes(mem.Lin(e.Seg(cpu.DS), c.R[cpu.SI]), append([]byte(p), 0))
		c.R[cpu.AX] = 0x0100
		d.ok(e)
	case 0x48:
		seg, largest, errc := d.alloc(c.R[cpu.BX], d.psp)
		if errc != 0 {
			d.fail(e, errc)
			c.R[cpu.BX] = largest
			return nil
		}
		c.R[cpu.AX] = seg
		d.ok(e)
	case 0x49:
		d.result(e, d.free(e.Seg(cpu.ES)))
	case 0x4A:
		largest, errc := d.resize(e.Seg(cpu.ES), c.R[cpu.BX])
		if errc != 0 {
			d.fail(e, errc)
			c.R[cpu.BX] = largest
			return nil
		}
		d.ok(e)
	case 0x4C:
		d.terminate(c.AL(), false)
	case 0x4D:
		c.R[cpu.AX] = uint16(d.exitType)<<8 | uint16(d.exit)
		d.ok(e)
	case 0x4B:
		return d.exec(e)
	case 0x31:
		d.resize(d.psp, c.R[cpu.DX])
		d.terminate(c.AL(), true)
	case 0x4E:
		d.result(e, d.findFirst(e.Mem.ASCIIZ(e.DSDX(), 128), byte(c.R[cpu.CX])))
	case 0x4F:
		d.result(e, d.findNext())
	case 0x50:
		d.psp = c.R[cpu.BX]
	case 0x51, 0x62:
		c.R[cpu.BX] = d.psp
	case 0x52:
		c.SetSeg(cpu.ES, dataSeg)
		c.R[cpu.BX] = offLoL
	case 0x56:
		d.result(e, d.rename(e.Mem.ASCIIZ(e.DSDX(), 128), d.str(e.Seg(cpu.ES), c.R[cpu.DI])))
	case 0x57:
		d.fileTime(e)
	case 0x58:
		switch c.AL() {
		case 0:
			c.R[cpu.AX] = d.strategy
		case 1:
			d.strategy = c.R[cpu.BX]
		case 2:
			c.SetAL(0)
		case 3:
		default:
			d.fail(e, errInvalidFunc)
			return nil
		}
		d.ok(e)
	case 0x59:
		c.R[cpu.AX] = d.lastErr
		c.SetBH(1)
		c.SetBL(4)
		c.SetCH(1)
	case 0x5A:
		return hle.Unsupported("INT 21h AH=5Ah (create temporary file)")
	case 0x5F:
		if c.AL() == 0x02 { // get redirection list entry: none
			d.fail(e, errNoMore)
			return nil
		}
		return hle.Unsupported("INT 21h AX=%04Xh (network)", c.R[cpu.AX])
	case 0x60:
		drive, p, errc := d.fs.canon(d.str(e.Seg(cpu.DS), c.R[cpu.SI]), true)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		out := []byte{byte('A' + drive), ':'}
		out = append(out, p...)
		e.Mem.SetBytes(mem.Lin(e.Seg(cpu.ES), c.R[cpu.DI]), append(out, 0))
		d.ok(e)
	case 0x63:
		if c.AL() == 0 {
			c.SetSeg(cpu.DS, dataSeg)
			c.R[cpu.SI] = offDBCS
			c.SetAL(0)
		} else {
			c.SetAL(0xFF)
		}
	case 0x65:
		return d.nls(e)
	case 0x66:
		switch c.AL() {
		case 1:
			c.R[cpu.BX] = uint16(e.CP.Num)
			c.R[cpu.DX] = uint16(e.CP.Num)
			d.ok(e)
		case 2:
			if int(c.R[cpu.BX]) == e.CP.Num {
				d.ok(e)
			} else {
				d.fail(e, 0x02)
			}
		default:
			return hle.Unsupported("INT 21h AX=%04Xh", c.R[cpu.AX])
		}
	case 0x67:
		d.ok(e) // handle count: our tables are dynamic
	case 0x68, 0x6A:
		d.ok(e)
	case 0x69: // get/set serial number (same structure as 440Dh/66h)
		if c.AL() == 0 {
			drive, errc := d.ioctlDrive(c.BL())
			if errc != 0 {
				d.fail(e, errc)
				return nil
			}
			d.mediaID(e.DSDX(), drive)
		}
		d.ok(e)
	case 0x6C:
		d.extOpen(e)
	case 0x71:
		return d.lfn(e)
	case 0x73:
		c.R[cpu.AX] = 0x7300
		e.SetCF(true)
	default:
		return hle.Unsupported("INT 21h AH=%02Xh", ah)
	}
	return nil
}

func (d *DOS) countryInfo(a uint32) {
	m := d.e.Mem
	b := make([]byte, 34)
	b[0] = 0    // date format: USA
	b[2] = '$'  // currency
	b[7] = ','  // thousands separator
	b[9] = '.'  // decimal separator
	b[11] = '-' // date separator
	b[13] = ':' // time separator
	b[15] = 0   // currency format
	b[16] = 2   // currency decimals
	b[17] = 0   // 12-hour clock
	b[22] = ',' // data-list separator
	m.SetBytes(a, b)
	// Case-map routine pointer (offset 12h): a RETF in ROM.
	m.W16(a+0x12, d.retfOff())
	m.W16(a+0x14, hle.ROMSeg)
}

func (d *DOS) retfOff() uint16 {
	if d.retf == 0 {
		d.retf = d.e.Emit([]byte{0xCB})
	}
	return d.retf
}

func (d *DOS) nls(e *hle.Env) error {
	c := e.CPU
	switch c.AL() {
	case 0x01:
		a := mem.Lin(e.Seg(cpu.ES), c.R[cpu.DI])
		e.Mem.W8(a, 1)
		e.Mem.W16(a+1, 41)
		e.Mem.W16(a+3, 1)
		e.Mem.W16(a+5, uint16(e.CP.Num))
		d.countryInfo(a + 7)
		c.R[cpu.CX] = 41
	case 0x02, 0x04:
		a := mem.Lin(e.Seg(cpu.ES), c.R[cpu.DI])
		e.Mem.W8(a, c.AL())
		off, id := uint16(offUpper), uint16(offUpperID)
		if c.AL() == 0x04 {
			off, id = offFUpper, offFUpperID
		}
		if d.utf8[d.psp] {
			off = id // UTF-8 mode: 80h-FFh map to themselves
		}
		e.Mem.W16(a+1, off)
		e.Mem.W16(a+3, dataSeg)
		c.R[cpu.CX] = 5
	case 0x20:
		c.SetDL(d.fs.upper(c.DL()))
	case 0x21:
		a := e.DSDX()
		for i := 0; i < int(c.R[cpu.CX]); i++ {
			e.Mem.W8(a+uint32(i), d.fs.upper(e.Mem.R8(a+uint32(i))))
		}
	case 0x22:
		a := e.DSDX()
		for i := uint32(0); ; i++ {
			v := e.Mem.R8(a + i)
			if v == 0 {
				break
			}
			e.Mem.W8(a+i, d.fs.upper(v))
		}
	default:
		return hle.Unsupported("INT 21h AX=%04Xh (NLS)", c.R[cpu.AX])
	}
	d.ok(e)
	return nil
}

func (d *DOS) attrib(e *hle.Env) {
	c := e.CPU
	drive, dp, errc := d.fs.canon(e.Mem.ASCIIZ(e.DSDX(), 128), false)
	if errc != 0 {
		d.fail(e, errc)
		return
	}
	e.Note("%c:%s", 'A'+drive, dp)
	host, _, errc := d.fs.resolve(drive, dp, false)
	if errc != 0 {
		d.fail(e, errc)
		return
	}
	st, err := os.Stat(host)
	if err != nil {
		d.fail(e, osErr(err))
		return
	}
	switch c.AL() {
	case 0:
		_, name := splitDir(dp)
		a := attrOf(dirEntry{dos: name, host: hostBase(host), info: st})
		c.R[cpu.CX] = uint16(a)
		c.R[cpu.AX] = uint16(a)
	case 1:
		mode := st.Mode().Perm()
		if c.CL()&attrRO != 0 {
			mode &^= 0o222
		} else {
			mode |= 0o200
		}
		if err := os.Chmod(host, mode); err != nil {
			d.fail(e, errAccess)
			return
		}
		d.fs.invalidate()
	default:
		d.fail(e, errInvalidFunc)
		return
	}
	d.ok(e)
}

func hostBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (d *DOS) ioctl(e *hle.Env) error {
	c := e.CPU
	switch c.AL() {
	case 0x00:
		of, errc := d.handle(c.R[cpu.BX])
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		var info uint16
		switch of.dev {
		case devCON:
			info = 0x80D3
		case devNUL:
			info = 0x8084
		default:
			info = uint16(of.drive)
			if !of.written {
				info |= 0x40
			}
		}
		c.R[cpu.DX] = info
		c.R[cpu.AX] = info
	case 0x01: // set device information: DH must be 0 or 1 (MS-DOS IOCTL.ASM);
		// the flags themselves are not modelled
		if _, errc := d.handle(c.R[cpu.BX]); errc != 0 {
			d.fail(e, errc)
			return nil
		}
		if c.DH()&0xFE != 0 { // DH=01h is accepted too
			d.fail(e, errInvalidData)
			return nil
		}
	case 0x06: // input status
		of, errc := d.handle(c.R[cpu.BX])
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.SetAL(0xFF)
		if of.dev == devCON && !d.charAvailable() {
			c.SetAL(0)
		} else if of.f != nil {
			pos, _ := of.f.Seek(0, io.SeekCurrent)
			if st, err := of.f.Stat(); err == nil && pos >= st.Size() {
				c.SetAL(0)
			}
		}
	case 0x07: // output status
		c.SetAL(0xFF)
	case 0x08: // removable? Host directories are fixed media.
		if _, errc := d.ioctlDrive(c.BL()); errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.R[cpu.AX] = 1
	case 0x09: // device attribute word: local, not SUBSTed, not remote
		if _, errc := d.ioctlDrive(c.BL()); errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.R[cpu.DX] = 0
	case 0x0A:
		c.R[cpu.DX] = 0
	case 0x0D:
		return d.genericIOCTL(e)
	case 0x0E: // get logical drive map: one logical drive per device
		if _, errc := d.ioctlDrive(c.BL()); errc != 0 {
			d.fail(e, errc)
			return nil
		}
		c.SetAL(0)
	case 0x0F: // set logical drive map: nothing to remap
		if _, errc := d.ioctlDrive(c.BL()); errc != 0 {
			d.fail(e, errc)
			return nil
		}
	default:
		return hle.Unsupported("INT 21h AX=%04Xh (IOCTL)", c.R[cpu.AX])
	}
	d.ok(e)
	return nil
}

func (d *DOS) dupHandle(of *openFile) (uint16, uint16) {
	a, size := d.jft()
	for i := 0; i < size; i++ {
		if d.e.Mem.R8(a+uint32(i)) == 0xFF {
			for idx, s := range d.sft {
				if s == of {
					d.e.Mem.W8(a+uint32(i), byte(idx))
					of.refs++
					return uint16(i), 0
				}
			}
		}
	}
	return 0, errTooMany
}

func (d *DOS) fileTime(e *hle.Env) {
	c := e.CPU
	of, errc := d.handle(c.R[cpu.BX])
	if errc != 0 {
		d.fail(e, errc)
		return
	}
	switch c.AL() {
	case 0:
		var t time.Time
		if of.f != nil {
			if st, err := of.f.Stat(); err == nil {
				t = st.ModTime()
			}
		} else {
			t = e.Now()
		}
		tm, dt := dosTime(t)
		c.R[cpu.CX], c.R[cpu.DX] = tm, dt
	case 1:
		if of.f != nil {
			t := fromDOSTime(c.R[cpu.CX], c.R[cpu.DX])
			of.f.Sync()
			if err := os.Chtimes(of.host, t, t); err != nil {
				d.fail(e, errAccess)
				return
			}
			d.fs.invalidate()
		}
	default:
		d.fail(e, errInvalidFunc)
		return
	}
	d.ok(e)
}

func (d *DOS) extOpen(e *hle.Env) {
	c := e.CPU
	name := d.str(e.Seg(cpu.DS), c.R[cpu.SI])
	action := c.DL()
	mode := c.BL()
	h, errc := d.open(name, mode, false, false, false)
	switch {
	case errc == 0 && action&0x0F == 1:
		c.R[cpu.CX] = 1
	case errc == 0 && action&0x0F == 2:
		d.closeHandle(h)
		h, errc = d.open(name, mode, true, true, false)
		c.R[cpu.CX] = 3
	case errc == errFileNotFound && action&0xF0 == 0x10:
		h, errc = d.open(name, mode, true, true, false)
		c.R[cpu.CX] = 2
	case errc == 0:
		d.closeHandle(h)
		errc = errExists
	}
	if errc != 0 {
		d.fail(e, errc)
		return
	}
	c.R[cpu.AX] = h
	d.ok(e)
}

// parseFCB implements INT 21h/29h; it returns bytes consumed and AL.
func (d *DOS) parseFCB(src []byte, fcb uint32, flags byte) (int, byte) {
	m := d.e.Mem
	i := 0
	if flags&1 != 0 {
		for i < len(src) && strings.IndexByte(" \t:;,=+", src[i]) >= 0 {
			i++
		}
	} else {
		for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
			i++
		}
	}
	al := byte(0)
	if i+1 < len(src) && src[i+1] == ':' {
		l := d.e.CP.Upper(src[i])
		if l >= 'A' && l <= 'Z' {
			m.W8(fcb, l-'A'+1)
			if d.fs.drives[l-'A'] == "" {
				al = 0xFF
			}
			i += 2
		}
	} else if flags&2 == 0 {
		m.W8(fcb, 0)
	}
	field := func(off uint32, n int, keep bool) bool {
		wild := false
		j := 0
		if !keep || (i < len(src) && !isSep(src[i])) {
			for k := 0; k < n; k++ {
				m.W8(fcb+off+uint32(k), ' ')
			}
		}
		for i < len(src) && !isSep(src[i]) && src[i] != '.' {
			ch := d.e.CP.Upper(src[i])
			i++
			if ch == '*' {
				for ; j < n; j++ {
					m.W8(fcb+off+uint32(j), '?')
				}
				wild = true
				continue
			}
			if ch == '?' {
				wild = true
			}
			if j < n {
				m.W8(fcb+off+uint32(j), ch)
				j++
			}
		}
		return wild
	}
	w1 := field(1, 8, flags&4 != 0)
	w2 := false
	if i < len(src) && src[i] == '.' {
		i++
		w2 = field(9, 3, false)
	} else if flags&8 == 0 {
		for k := 0; k < 3; k++ {
			m.W8(fcb+9+uint32(k), ' ')
		}
	}
	if (w1 || w2) && al == 0 {
		al = 1
	}
	return i, al
}

func isSep(c byte) bool {
	return c <= ' ' || strings.IndexByte(`"/\[]<>|+=;,:`, c) >= 0
}

// int2F is the multiplex interrupt. Unknown services keep the registers
// unchanged, which is the "not installed" answer to installation checks.
func (d *DOS) int2F(e *hle.Env) error {
	c := e.CPU
	if ok, err := d.winOldAp(e); ok {
		return err
	}
	switch c.R[cpu.AX] {
	case 0x1680: // release time slice
		e.Idle()
		c.SetAL(0)
		return nil
	case 0x1600, 0x4300, 0x4A00, 0x1100, 0x1000, 0xB700, 0xAE00, 0x150B, 0x1500, 0x168F, 0x4F00, 0x4680, 0x1700:
	}
	e.Note("multiplex %04Xh: not installed", c.R[cpu.AX])
	return nil
}
