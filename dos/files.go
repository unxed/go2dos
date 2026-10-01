package dos

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/unxed/go2dos/mem"
)

type device int

const (
	devNone device = iota
	devCON
	devNUL
)

type openFile struct {
	f       *os.File
	dev     device
	name    string // DOS path or device name
	host    string
	drive   int
	refs    int
	mode    byte
	written bool
	owner   uint16
}

var deviceNames = map[string]device{
	"CON": devCON, "NUL": devNUL, "PRN": devNUL, "AUX": devNUL,
	"LPT1": devNUL, "LPT2": devNUL, "LPT3": devNUL, "COM1": devNUL, "COM2": devNUL,
}

// deviceFor reports whether the last component of a canonical path names a device.
func deviceFor(p string) (device, string) {
	_, n := splitDir(p)
	if i := strings.IndexByte(n, '.'); i >= 0 {
		n = n[:i]
	}
	if d, ok := deviceNames[n]; ok {
		return d, n
	}
	return devNone, ""
}

// jft returns the linear address of the current process's handle table.
func (d *DOS) jft() (uint32, int) {
	m := d.e.Mem
	a := mem.Lin(d.psp, 0)
	size := int(m.R16(a + 0x32))
	ptr := m.R32(a + 0x34)
	return mem.Lin(uint16(ptr>>16), uint16(ptr)), size
}

func (d *DOS) handle(h uint16) (*openFile, uint16) {
	a, size := d.jft()
	if int(h) >= size {
		return nil, errBadHandle
	}
	idx := d.e.Mem.R8(a + uint32(h))
	if idx == 0xFF || int(idx) >= len(d.sft) || d.sft[idx] == nil {
		return nil, errBadHandle
	}
	return d.sft[idx], 0
}

func (d *DOS) newHandle(of *openFile) (uint16, uint16) {
	a, size := d.jft()
	h := -1
	for i := 0; i < size; i++ {
		if d.e.Mem.R8(a+uint32(i)) == 0xFF {
			h = i
			break
		}
	}
	if h < 0 {
		return 0, errTooMany
	}
	idx := -1
	for i, s := range d.sft {
		if s == nil {
			idx = i
			break
		}
	}
	if idx < 0 {
		if len(d.sft) >= 255 {
			return 0, errTooMany
		}
		idx = len(d.sft)
		d.sft = append(d.sft, nil)
	}
	of.refs = 1
	of.owner = d.psp
	d.sft[idx] = of
	d.e.Mem.W8(a+uint32(h), byte(idx))
	return uint16(h), 0
}

func (d *DOS) closeHandle(h uint16) uint16 {
	a, _ := d.jft()
	of, errc := d.handle(h)
	if errc != 0 {
		return errc
	}
	idx := d.e.Mem.R8(a + uint32(h))
	d.e.Mem.W8(a+uint32(h), 0xFF)
	of.refs--
	if of.refs <= 0 {
		if of.f != nil {
			of.f.Close()
		}
		if of.written {
			d.fs.invalidate()
		}
		d.sft[idx] = nil
	}
	return 0
}

func (d *DOS) closeAll(psp uint16) {
	if psp == 0 {
		return
	}
	_, size := d.jft()
	for h := 0; h < size; h++ {
		if _, errc := d.handle(uint16(h)); errc == 0 && h > 4 {
			d.closeHandle(uint16(h))
		}
	}
}

func osErr(err error) uint16 {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, fs.ErrNotExist):
		return errFileNotFound
	case errors.Is(err, fs.ErrExist):
		return errExists
	case errors.Is(err, fs.ErrPermission):
		return errAccess
	}
	return errAccess
}

// open opens (or creates) a file by DOS path. mode is the INT 21h/3Dh access byte.
func (d *DOS) open(path []byte, mode byte, create, truncate, exclusive bool) (uint16, uint16) {
	drive, dp, errc := d.fs.canon(path, false)
	if errc != 0 {
		return 0, errc
	}
	if dev, name := deviceFor(dp); dev != devNone {
		return d.newHandle(&openFile{dev: dev, name: name, mode: mode})
	}
	d.e.Note("%c:%s", 'A'+drive, dp)
	host, exists, errc := d.fs.resolve(drive, dp, create)
	if errc != 0 {
		return 0, errc
	}
	if exists && exclusive {
		return 0, errExists
	}
	if exists {
		if st, err := os.Stat(host); err == nil && st.IsDir() {
			return 0, errAccess
		}
	}
	flag := os.O_RDONLY
	switch mode & 3 {
	case 1:
		flag = os.O_WRONLY
	case 2:
		flag = os.O_RDWR
	}
	if create {
		flag = os.O_RDWR | os.O_CREATE
		if truncate {
			flag |= os.O_TRUNC
		}
	}
	f, err := os.OpenFile(host, flag, 0o666)
	if err != nil && !create && flag != os.O_RDONLY && errors.Is(err, fs.ErrPermission) {
		return 0, errAccess
	}
	if err != nil {
		return 0, osErr(err)
	}
	if create {
		d.fs.invalidate()
	}
	h, errc := d.newHandle(&openFile{f: f, name: dp, host: host, drive: drive, mode: mode, written: create})
	if errc != 0 {
		f.Close()
	}
	return h, errc
}

func (d *DOS) read(h uint16, buf []byte) (int, uint16) {
	of, errc := d.handle(h)
	if errc != 0 {
		return 0, errc
	}
	switch of.dev {
	case devNUL:
		return 0, 0
	case devCON:
		return d.conRead(buf)
	}
	n, err := of.f.Read(buf)
	if err != nil && err != io.EOF {
		return n, errAccess
	}
	return n, 0
}

func (d *DOS) write(h uint16, buf []byte) (int, uint16) {
	of, errc := d.handle(h)
	if errc != 0 {
		return 0, errc
	}
	switch of.dev {
	case devNUL:
		return len(buf), 0
	case devCON:
		d.conWrite(buf)
		return len(buf), 0
	}
	if len(buf) == 0 { // DOS: a zero-length write truncates at the current position
		pos, _ := of.f.Seek(0, io.SeekCurrent)
		if err := of.f.Truncate(pos); err != nil {
			return 0, errAccess
		}
		of.written = true
		return 0, 0
	}
	n, err := of.f.Write(buf)
	of.written = true
	if err != nil {
		return n, errAccess
	}
	return n, 0
}

// --- directory search -----------------------------------------------------------

// The DTA's 21 reserved bytes hold the search state like real DOS does:
// 00 drive+1, 01-0B pattern (FCB form), 0C attributes, 0D-0E next entry,
// 0F-10 directory id, 11-12 magic.
const findMagic = 0x6F67 // "go"

func (d *DOS) findFirst(path []byte, attr byte) uint16 {
	drive, dp, errc := d.fs.canon(path, true)
	if errc != 0 {
		return errc
	}
	dir, name := splitDir(dp)
	d.e.Note("%c:%s attr=%02X", 'A'+drive, dp, attr)
	if attr == attrLabel {
		return errNoMore
	}
	host, _, errc := d.fs.resolve(drive, dir, false)
	if errc != 0 {
		return errPathNotFound
	}
	if _, errc := d.fs.index(host); errc != 0 {
		return errPathNotFound
	}
	m := d.e.Mem
	a := d.dta
	m.W8(a, byte(drive+1))
	pat := fcbName(name)
	m.SetBytes(a+1, pat[:])
	m.W8(a+0x0C, attr)
	m.W16(a+0x0D, 0)
	m.W16(a+0x0F, d.fs.dirID(host))
	m.W16(a+0x11, findMagic)
	if dir == `\` {
		m.W16(a+0x0D, 2) // the root has no "." and ".."
	}
	if errc := d.findNext(); errc != 0 {
		if errc == errNoMore {
			return errFileNotFound
		}
		return errc
	}
	return 0
}

func (d *DOS) findNext() uint16 {
	m := d.e.Mem
	a := d.dta
	if m.R16(a+0x11) != findMagic {
		return errNoMore
	}
	id := int(m.R16(a + 0x0F))
	if id >= len(d.fs.dirs) {
		return errNoMore
	}
	host := d.fs.dirs[id]
	ix, errc := d.fs.index(host)
	if errc != 0 {
		return errNoMore
	}
	var pat [11]byte
	copy(pat[:], m.Bytes(a+1, 11))
	attr := m.R8(a + 0x0C)
	for pos := int(m.R16(a + 0x0D)); ; pos++ {
		var e dirEntry
		var ea byte
		switch {
		case pos < 2:
			name := "."
			if pos == 1 {
				name = ".."
			}
			st, err := os.Stat(host)
			if err != nil {
				continue
			}
			e = dirEntry{dos: name, host: name, info: st}
			ea = attrDir
		case pos-2 < len(ix.entries):
			e = ix.entries[pos-2]
			ea = attrOf(e)
		default:
			return errNoMore
		}
		var fn [11]byte
		if e.dos == "." || e.dos == ".." {
			for i := range fn {
				fn[i] = ' '
			}
			copy(fn[:], e.dos)
		} else {
			fn = fcbName(e.dos)
		}
		if !fcbMatch(pat, fn) {
			continue
		}
		if ea&(attrHidden|attrSystem|attrDir)&^attr != 0 {
			continue
		}
		m.W16(a+0x0D, uint16(pos+1))
		m.W8(a+0x15, ea)
		tm, dt := dosTime(e.info.ModTime())
		m.W16(a+0x16, tm)
		m.W16(a+0x18, dt)
		size := uint32(e.info.Size())
		if e.info.IsDir() {
			size = 0
		}
		m.W32(a+0x1A, size)
		nb := make([]byte, 13)
		copy(nb, e.dos)
		m.SetBytes(a+0x1E, nb)
		return 0
	}
}

// --- directories ------------------------------------------------------------------

func (d *DOS) mkdir(path []byte) uint16 {
	drive, dp, errc := d.fs.canon(path, false)
	if errc != 0 {
		return errc
	}
	host, exists, errc := d.fs.resolve(drive, dp, true)
	if errc != 0 {
		return errc
	}
	if exists {
		return errAccess
	}
	d.fs.invalidate()
	if err := os.Mkdir(host, 0o777); err != nil {
		return errAccess
	}
	return 0
}

func (d *DOS) rmdir(path []byte) uint16 {
	drive, dp, errc := d.fs.canon(path, false)
	if errc != 0 {
		return errc
	}
	if d.fs.cwd[drive] == dp {
		return errCurDir
	}
	host, _, errc := d.fs.resolve(drive, dp, false)
	if errc != 0 {
		return errPathNotFound
	}
	d.fs.invalidate()
	if err := os.Remove(host); err != nil {
		return errAccess
	}
	return 0
}

func (d *DOS) chdir(path []byte) uint16 {
	drive, dp, errc := d.fs.canon(path, false)
	if errc != 0 {
		return errc
	}
	host, _, errc := d.fs.resolve(drive, dp, false)
	if errc != 0 {
		return errPathNotFound
	}
	if st, err := os.Stat(host); err != nil || !st.IsDir() {
		return errPathNotFound
	}
	d.fs.cwd[drive] = dp
	return 0
}

func (d *DOS) unlink(path []byte) uint16 {
	drive, dp, errc := d.fs.canon(path, false)
	if errc != 0 {
		return errc
	}
	host, _, errc := d.fs.resolve(drive, dp, false)
	if errc != 0 {
		return errc
	}
	if st, err := os.Stat(host); err == nil && st.IsDir() {
		return errAccess
	}
	d.fs.invalidate()
	return osErr(os.Remove(host))
}

func (d *DOS) rename(from, to []byte) uint16 {
	d1, p1, errc := d.fs.canon(from, false)
	if errc != 0 {
		return errc
	}
	d2, p2, errc := d.fs.canon(to, false)
	if errc != 0 {
		return errc
	}
	if d1 != d2 {
		return errNotSame
	}
	h1, _, errc := d.fs.resolve(d1, p1, false)
	if errc != 0 {
		return errc
	}
	h2, exists, errc := d.fs.resolve(d2, p2, true)
	if errc != 0 {
		return errPathNotFound
	}
	if exists && !strings.EqualFold(filepath.Clean(h1), filepath.Clean(h2)) {
		return errAccess
	}
	d.fs.invalidate()
	return osErr(os.Rename(h1, h2))
}
