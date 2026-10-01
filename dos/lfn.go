package dos

import (
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Long file names: INT 21h AH=71h (Windows 95 LFN API; RBIL "Int 21/AX=71xxh").
//
// Names in and out are bytes in the OEM code page; the host holds them in
// Unicode. A component is looked up by its exact host name, then without
// regard to case, then as the 8.3 alias that the classic calls see (fs.go,
// index). Names that do not fit the code page come out with '_' for each
// unmappable character (Unicode conversion flag, RBIL table 01780) and a
// name longer than 255 bytes is reported by its alias.

const (
	maxLFNName = 255 // longest file name (71A0h CX)
	maxLFNPath = 260 // longest path (71A0h DX)

	lfnFlags = 0x4006 // 71A0h: preserves case (2), Unicode names (4), LFN functions (4000h)
)

// lfnStr reads an ASCIZ long name or path.
func (d *DOS) lfnStr(seg, off uint16) []byte {
	return d.e.Mem.ASCIIZ(mem.Lin(seg, off), maxLFNPath+8)
}

// oem converts a host name to the code page; lossy is true if an unmappable
// character became '_'.
func (f *fsys) oem(s string) (out []byte, lossy bool) {
	for _, r := range s {
		b, ok := f.e.CP.Byte(r)
		if !ok {
			b, lossy = '_', true
		}
		out = append(out, b)
	}
	return out, lossy
}

// validLFNPart reports whether s can be a component of a long name.
func validLFNPart(s string) bool {
	if s == "" || len(s) > 4*maxLFNName {
		return false
	}
	for _, r := range s {
		if r < 0x20 || strings.ContainsRune(`<>|"*?`, r) {
			return false
		}
	}
	return true
}

// foldEq compares names the way a case-insensitive file system does.
func foldEq(a, b string) bool { return strings.ToLower(a) == strings.ToLower(b) }

// findName finds a component of a directory by long name, case-insensitive
// long name or 8.3 alias.
func (f *fsys) findName(hostDir, name string) (dirEntry, bool, uint16) {
	ix, errc := f.index(hostDir)
	if errc != 0 {
		return dirEntry{}, false, errc
	}
	if i, ok := ix.byHost[name]; ok {
		return ix.entries[i], true, 0
	}
	if i, ok := ix.byFold[strings.ToLower(name)]; ok {
		return ix.entries[i], true, 0
	}
	if enc, ok := f.e.CP.Encode(name); ok {
		for i, c := range enc {
			enc[i] = f.e.CP.Upper(c)
		}
		if i, ok := ix.byDOS[string(enc)]; ok {
			return ix.entries[i], true, 0
		}
	}
	return dirEntry{}, false, 0
}

// lfnPath is a resolved long-name path.
type lfnPath struct {
	drive  int
	host   string // host path of the target (it need not exist)
	exists bool
	parent string   // host directory of the last component ("" for the root itself)
	name   string   // last component as given (for a file that does not exist yet)
	entry  dirEntry // the target, if it exists and is not the root
	isRoot bool
}

// lfnResolve resolves a path in the code page: drive letter, absolute or
// relative to the current directory, '.' and '..', long names and aliases.
// A missing last component is not an error (exists is false); a missing
// directory is errPathNotFound.
func (f *fsys) lfnResolve(p []byte) (lfnPath, uint16) {
	drive := f.cur
	if len(p) >= 2 && p[1] == ':' {
		l := f.e.CP.Upper(p[0])
		if l < 'A' || l > 'Z' {
			return lfnPath{}, errBadDrive
		}
		drive = int(l - 'A')
		p = p[2:]
	}
	if f.drives[drive] == "" {
		return lfnPath{}, errBadDrive
	}
	root := f.drives[drive]
	s := strings.ReplaceAll(f.e.CP.Decode(p), "/", `\`)
	cur := root
	if !strings.HasPrefix(s, `\`) {
		h, _, errc := f.resolve(drive, f.cwd[drive], false)
		if errc != 0 {
			return lfnPath{}, errPathNotFound
		}
		cur = h
	}
	out := lfnPath{drive: drive, host: cur, exists: true, isRoot: cur == root}
	parts := strings.Split(s, `\`)
	for i, part := range parts {
		last := i == len(parts)-1
		switch part {
		case "", ".":
			continue
		case "..":
			if cur != root {
				cur = filepath.Dir(cur)
			}
			out = lfnPath{drive: drive, host: cur, exists: true, isRoot: cur == root}
			continue
		}
		part = strings.TrimRight(part, " .") // trailing blanks and dots do not belong to a name
		if !validLFNPart(part) {
			if last {
				return lfnPath{}, errFileNotFound
			}
			return lfnPath{}, errPathNotFound
		}
		ent, ok, errc := f.findName(cur, part)
		if errc != 0 {
			return lfnPath{}, errPathNotFound
		}
		if !ok {
			if !last {
				return lfnPath{}, errPathNotFound
			}
			return lfnPath{drive: drive, host: filepath.Join(cur, part), parent: cur, name: part}, 0
		}
		if !last && !ent.info.IsDir() {
			return lfnPath{}, errPathNotFound
		}
		out = lfnPath{drive: drive, host: filepath.Join(cur, ent.host), exists: true, parent: cur, name: ent.host, entry: ent}
		cur = out.host
	}
	return out, 0
}

// existing is lfnResolve for a target that must exist.
func (f *fsys) existing(p []byte) (lfnPath, uint16) {
	r, errc := f.lfnResolve(p)
	if errc == 0 && !r.exists {
		return r, errFileNotFound
	}
	return r, errc
}

// relComps splits a host path below the drive root into its components.
func (f *fsys) relComps(drive int, host string) []string {
	rel, err := filepath.Rel(f.drives[drive], host)
	if err != nil || rel == "." {
		return nil
	}
	return strings.Split(rel, string(filepath.Separator))
}

// shortOf returns the DOS path (\DIR\NAME.EXT, aliases) of an existing host path.
func (f *fsys) shortOf(drive int, host string) (string, uint16) {
	dir, out := f.drives[drive], ""
	for _, c := range f.relComps(drive, host) {
		ent, ok, errc := f.findName(dir, c)
		if errc != 0 || !ok {
			return "", errPathNotFound
		}
		out += `\` + ent.dos
		dir = filepath.Join(dir, ent.host)
	}
	if out == "" {
		out = `\`
	}
	return out, 0
}

// longOf returns the path of an existing host path in long names, in the code
// page. A component that does not fit (255 bytes) is given by its alias.
func (f *fsys) longOf(drive int, host string) ([]byte, bool, uint16) {
	dir, lossy := f.drives[drive], false
	out := []byte{}
	for _, c := range f.relComps(drive, host) {
		ent, ok, errc := f.findName(dir, c)
		if errc != 0 || !ok {
			return nil, false, errPathNotFound
		}
		name, l := f.oem(ent.host)
		if len(name) > maxLFNName {
			name, l = []byte(ent.dos), false
		}
		lossy = lossy || l
		out = append(append(out, '\\'), name...)
		dir = filepath.Join(dir, ent.host)
	}
	if len(out) == 0 {
		out = []byte{'\\'}
	}
	return out, lossy, 0
}

// plainPath is TRUENAME (7160h CL=0): the path made absolute and upper-cased,
// '.' and '..' resolved, names neither looked up nor shortened.
func (f *fsys) plainPath(p []byte) ([]byte, uint16) {
	drive := f.cur
	if len(p) >= 2 && p[1] == ':' {
		l := f.e.CP.Upper(p[0])
		if l < 'A' || l > 'Z' {
			return nil, errPathNotFound
		}
		drive = int(l - 'A')
		p = p[2:]
	}
	if f.drives[drive] == "" {
		return nil, errPathNotFound
	}
	s := strings.ReplaceAll(string(p), "/", `\`)
	if !strings.HasPrefix(s, `\`) {
		s = strings.TrimSuffix(f.cwd[drive], `\`) + `\` + s
	}
	var out []string
	for _, part := range strings.Split(s, `\`) {
		switch part {
		case "", ".":
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			up := []byte(part)
			for i, c := range up {
				up[i] = f.e.CP.Upper(c)
			}
			out = append(out, string(up))
		}
	}
	return append([]byte{byte('A' + drive), ':', '\\'}, strings.Join(out, `\`)...), 0
}

// --- wildcard matching ---------------------------------------------------------

// wildMatch matches name against a pattern with '*' and '?', ignoring case.
// "*.*" matches every name, with or without a dot.
func wildMatch(pat, name string) bool {
	if pat == "*.*" {
		return true
	}
	return wild([]rune(strings.ToLower(pat)), []rune(strings.ToLower(name)))
}

func wild(p, n []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			for i := 0; i <= len(n); i++ {
				if wild(p, n[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(n) == 0 {
				return false
			}
		default:
			if len(n) == 0 || unicode.ToLower(n[0]) != p[0] {
				return false
			}
		}
		p, n = p[1:], n[1:]
	}
	return len(n) == 0
}

// lfnMatch is one entry found by a long-name search.
type lfnMatch struct {
	long, short string // host name and DOS alias ("." and ".." for themselves)
	attr        byte
	info        os.FileInfo
}

// lfnSearch lists the entries of hostDir whose long name or alias matches
// pat. allow (CL) is the "at most" mask of hidden, system and directory
// attributes, must (CH) the attributes an entry has to have; a label-only
// search (CL=08h) yields the volume label.
func (d *DOS) lfnSearch(drive int, hostDir, pat string, allow, must byte) ([]lfnMatch, uint16) {
	f := d.fs
	if labelOnly(allow) {
		hostDir = f.drives[drive]
	}
	ix, errc := f.index(hostDir)
	if errc != 0 {
		return nil, errPathNotFound
	}
	var out []lfnMatch
	st, serr := os.Stat(hostDir)
	isRoot := hostDir == f.drives[drive]
	if isRoot && allow&attrLabel != 0 && f.labels[drive] != "" && serr == nil {
		lb := f.labelEntry(drive)
		if wildMatch(pat, lb) {
			out = append(out, lfnMatch{long: lb, short: lb, attr: attrLabel, info: st})
		}
	}
	if labelOnly(allow) {
		return out, 0
	}
	if !isRoot && serr == nil { // the root has no "." and ".."
		for _, n := range []string{".", ".."} {
			if wildMatch(pat, n) && attrDir&^allow == 0 && attrDir&must == must {
				out = append(out, lfnMatch{long: n, short: n, attr: attrDir, info: st})
			}
		}
	}
	for _, e := range ix.entries {
		if !wildMatch(pat, e.host) && !wildMatch(pat, e.dos) {
			continue
		}
		ea := attrOf(e)
		if ea&(attrHidden|attrSystem|attrDir)&^allow != 0 || ea&must != must {
			continue
		}
		out = append(out, lfnMatch{long: e.host, short: e.dos, attr: ea, info: e.info})
	}
	return out, 0
}

// fileTime64 is a Windows FILETIME: 100 ns intervals since 1601-01-01 UTC.
func fileTime64(t time.Time) uint64 {
	return uint64(t.Unix()+11644473600)*10000000 + uint64(t.Nanosecond()/100)
}

// putFindData writes the FindData record (RBIL table 01779) at a and returns
// the Unicode conversion flags. dosFmt selects MS-DOS date/time values for
// the file times (RBIL table 01778) instead of 64-bit file times.
func (d *DOS) putFindData(a uint32, m lfnMatch, dosFmt bool) uint16 {
	mm := d.e.Mem
	mm.SetBytes(a, make([]byte, 0x13E))
	mm.W32(a, uint32(m.attr))
	t := m.info.ModTime() // the host has no portable creation or access time
	for _, off := range []uint32{0x04, 0x0C, 0x14} {
		if dosFmt {
			tm, dt := dosTime(t)
			mm.W32(a+off, uint32(dt)<<16|uint32(tm))
		} else {
			ft := fileTime64(t)
			mm.W32(a+off, uint32(ft))
			mm.W32(a+off+4, uint32(ft>>32))
		}
	}
	size := uint64(m.info.Size())
	if m.info.IsDir() {
		size = 0
	}
	mm.W32(a+0x1C, uint32(size>>32))
	mm.W32(a+0x20, uint32(size))
	long, lossy := d.fs.oem(m.long)
	if len(long) > maxLFNName {
		long, lossy = []byte(m.short), false
	}
	mm.SetBytes(a+0x2C, append(long, 0))
	short := []byte(m.short)
	if len(short) > 13 {
		short = short[:13]
	}
	mm.SetBytes(a+0x130, append(short, 0))
	if lossy {
		return 1
	}
	return 0
}

// lfnFind is an open search (the "filefind handle").
type lfnFind struct {
	list []lfnMatch
	pos  int
}

// --- the INT 21h/71xx dispatcher -----------------------------------------------

func (d *DOS) lfn(e *hle.Env) error {
	c := e.CPU
	f := d.fs
	if d.noLFN {
		// Config.NoLFN: no 71xx function at all, not even the volume query
		// 71A0h, so the program sees no sign of long names.
		e.Note("LFN 71%02Xh not supported (NoLFN)", c.AL())
		c.R[cpu.AX] = 0x7100
		e.SetCF(true)
		return nil
	}
	switch al := c.AL(); al {
	case 0x0D: // reset drive
		d.ok(e)
	case 0x39: // mkdir
		r, errc := f.lfnResolve(d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX]))
		switch {
		case errc != 0:
			d.fail(e, errc)
		case r.exists:
			d.fail(e, errAccess)
		default:
			f.invalidate()
			if err := os.Mkdir(r.host, 0o777); err != nil {
				d.fail(e, errAccess)
				return nil
			}
			d.ok(e)
		}
	case 0x3A: // rmdir
		r, errc := f.existing(d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX]))
		if errc == 0 {
			if st, err := os.Stat(r.host); err != nil || !st.IsDir() {
				errc = errPathNotFound
			} else if r.isRoot {
				errc = errAccess
			} else if cwd, _, ec := f.resolve(r.drive, f.cwd[r.drive], false); ec == 0 && cwd == r.host {
				errc = errCurDir
			}
		}
		if errc == 0 {
			f.invalidate()
			if err := os.Remove(r.host); err != nil {
				errc = errAccess
			}
		}
		d.result(e, errc)
	case 0x3B: // chdir
		r, errc := f.existing(d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX]))
		if errc == 0 {
			if st, err := os.Stat(r.host); err != nil || !st.IsDir() {
				errc = errPathNotFound
			}
		}
		if errc == 0 {
			var sp string
			if sp, errc = f.shortOf(r.drive, r.host); errc == 0 {
				f.cwd[r.drive] = sp
			}
		}
		d.result(e, errc)
	case 0x41: // delete
		d.result(e, d.lfnDelete(e))
	case 0x43:
		return d.lfnAttr(e)
	case 0x47: // current directory, long names, without drive and leading backslash
		drive, errc := d.ioctlDrive(c.DL())
		if errc != 0 {
			d.fail(e, errBadDrive)
			return nil
		}
		host, _, errc := f.resolve(drive, f.cwd[drive], false)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		p, _, errc := f.longOf(drive, host)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		e.Mem.SetBytes(mem.Lin(e.Seg(cpu.DS), c.R[cpu.SI]), append(p[1:], 0))
		d.ok(e)
	case 0x4E, 0x4F, 0xA2:
		return d.lfnFindCall(e)
	case 0xA1: // find close
		h := c.R[cpu.BX]
		if _, ok := d.finds[h]; !ok {
			d.fail(e, errBadHandle)
			return nil
		}
		delete(d.finds, h)
		d.ok(e)
	case 0x56: // rename
		d.result(e, d.lfnRename(e))
	case 0x60:
		return d.lfnTrueName(e)
	case 0x6C:
		d.lfnOpen(e)
	case 0xA0: // volume information
		root, errc := f.lfnResolve(d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX]))
		if errc != 0 {
			d.fail(e, errBadDrive)
			return nil
		}
		name := []byte("FAT\x00")
		if n := int(c.R[cpu.CX]); n < len(name) {
			d.fail(e, errInvalidFunc)
			return nil
		}
		e.Mem.SetBytes(mem.Lin(e.Seg(cpu.ES), c.R[cpu.DI]), name)
		e.Note("%c:", 'A'+root.drive)
		c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX] = 0, lfnFlags, maxLFNName, maxLFNPath
		d.ok(e)
	default:
		// The documented answer of a DOS without that LFN function.
		e.Note("LFN 71%02Xh not supported", al)
		c.R[cpu.AX] = 0x7100
		e.SetCF(true)
	}
	return nil
}

// lfnDelete is 7141h.
func (d *DOS) lfnDelete(e *hle.Env) uint16 {
	c := e.CPU
	p := d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX])
	if c.R[cpu.SI]&1 == 0 { // no wildcards, attributes are ignored
		r, errc := d.fs.existing(p)
		if errc != 0 {
			return errc
		}
		if st, err := os.Stat(r.host); err == nil && st.IsDir() {
			return errAccess
		}
		d.fs.invalidate()
		return osErr(os.Remove(r.host))
	}
	dirPart, pat := splitLFN(p)
	r, errc := d.fs.existing(dirPart)
	if errc != 0 {
		return errc
	}
	list, errc := d.lfnSearch(r.drive, r.host, d.e.CP.Decode(pat), c.CL(), c.CH())
	if errc != 0 {
		return errc
	}
	n := 0
	for _, m := range list {
		if m.attr&(attrDir|attrLabel) != 0 {
			continue
		}
		d.fs.invalidate()
		if err := os.Remove(filepath.Join(r.host, m.long)); err != nil {
			return errAccess
		}
		n++
	}
	if n == 0 {
		return errFileNotFound
	}
	return 0
}

// splitLFN splits a path into its directory part (with the drive; the
// current directory if empty) and the last component.
func splitLFN(p []byte) (dir, last []byte) {
	i := len(p)
	for i > 0 && p[i-1] != '\\' && p[i-1] != '/' && !(i == 2 && p[1] == ':') {
		i--
	}
	dir, last = p[:i], p[i:]
	if len(dir) == 0 {
		dir = []byte(".")
	}
	return dir, last
}

// lfnRename is 7156h.
func (d *DOS) lfnRename(e *hle.Env) uint16 {
	c := e.CPU
	from, errc := d.fs.existing(d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX]))
	if errc != 0 {
		return errc
	}
	to, errc := d.fs.lfnResolve(d.lfnStr(e.Seg(cpu.ES), c.R[cpu.DI]))
	if errc != 0 {
		return errc
	}
	if from.drive != to.drive {
		return errNotSame
	}
	if from.isRoot || to.isRoot {
		return errAccess
	}
	if to.exists && !foldEq(filepath.Clean(from.host), filepath.Clean(to.host)) {
		return errAccess
	}
	d.fs.invalidate()
	return osErr(os.Rename(from.host, to.host))
}

// lfnOpen is 716Ch: BX access mode, CX attributes (ignored), DX action,
// DS:SI name. The action bits are in RBIL table 01781.
func (d *DOS) lfnOpen(e *hle.Env) {
	c := e.CPU
	name := d.lfnStr(e.Seg(cpu.DS), c.R[cpu.SI])
	if _, dp, errc := d.fs.canon(name, false); errc == 0 {
		if dev, dn := deviceFor(dp); dev != devNone {
			h, errc := d.newHandle(&openFile{dev: dev, name: dn, mode: c.BL()})
			if errc != 0 {
				d.fail(e, errc)
				return
			}
			c.R[cpu.AX], c.R[cpu.CX] = h, 1
			d.ok(e)
			return
		}
	}
	r, errc := d.fs.lfnResolve(name)
	if errc != 0 {
		d.fail(e, errc)
		return
	}
	action := c.DL()
	mode := c.BL() & 7
	if mode == 4 { // read-only without touching the access time
		mode = 0
	}
	var create, truncate, exclusive bool
	taken := uint16(1)
	switch {
	case !r.exists && action&0x10 != 0:
		create, truncate, taken = true, true, 2
	case !r.exists:
		d.fail(e, errFileNotFound)
		return
	case action&0x0F == 1:
	case action&0x0F == 2:
		create, truncate, taken = true, true, 3
	default: // only "create new": the file exists
		d.fail(e, errExists)
		return
	}
	e.Note("%s", r.host)
	h, errc := d.openHost(r.drive, string(name), r.host, r.exists, mode, create, truncate, exclusive)
	if errc != 0 {
		d.fail(e, errc)
		return
	}
	c.R[cpu.AX], c.R[cpu.CX] = h, taken
	d.ok(e)
}

// lfnFindCall is 714Eh, 714Fh and 71A2h.
func (d *DOS) lfnFindCall(e *hle.Env) error {
	c := e.CPU
	dosFmt := c.R[cpu.SI] == 1
	out := mem.Lin(e.Seg(cpu.ES), c.R[cpu.DI])
	if c.AL() == 0x4E {
		dirPart, pat := splitLFN(d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX]))
		r, errc := d.fs.existing(dirPart)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		allow, must := c.CL(), c.CH()
		e.Note("%s %s attr=%02X/%02X", r.host, d.e.CP.Decode(pat), allow, must)
		list, errc := d.lfnSearch(r.drive, r.host, d.e.CP.Decode(pat), allow, must)
		if errc != 0 {
			d.fail(e, errc)
			return nil
		}
		if len(list) == 0 {
			d.fail(e, errFileNotFound)
			return nil
		}
		if d.finds == nil {
			d.finds = map[uint16]*lfnFind{}
		}
		d.nextFind++
		if d.nextFind == 0 {
			d.nextFind = 1
		}
		h := d.nextFind
		d.finds[h] = &lfnFind{list: list, pos: 1}
		flags := d.putFindData(out, list[0], dosFmt)
		c.R[cpu.AX], c.R[cpu.CX] = h, flags
		d.ok(e)
		return nil
	}
	fd, ok := d.finds[c.R[cpu.BX]]
	if !ok {
		d.fail(e, errBadHandle)
		return nil
	}
	if fd.pos >= len(fd.list) {
		d.fail(e, errNoMore)
		return nil
	}
	flags := d.putFindData(out, fd.list[fd.pos], dosFmt)
	fd.pos++
	if c.AL() == 0xA2 {
		c.R[cpu.AX] = 0x71A2
	} else {
		c.SetAH(0x4F)
	}
	c.R[cpu.CX] = flags
	d.ok(e)
	return nil
}

// lfnAttr is 7143h.
func (d *DOS) lfnAttr(e *hle.Env) error {
	c := e.CPU
	r, errc := d.fs.existing(d.lfnStr(e.Seg(cpu.DS), c.R[cpu.DX]))
	if errc != 0 {
		d.fail(e, errc)
		return nil
	}
	st, err := os.Stat(r.host)
	if err != nil {
		d.fail(e, osErr(err))
		return nil
	}
	switch bl := c.BL(); bl {
	case 0:
		ent := r.entry
		if ent.info == nil { // the root or the current directory
			ent = dirEntry{host: hostBase(r.host), info: st}
		}
		c.R[cpu.CX] = uint16(attrOf(ent))
	case 1:
		mode := st.Mode().Perm()
		if c.CL()&attrRO != 0 {
			mode &^= 0o222
		} else {
			mode |= 0o200
		}
		if err := os.Chmod(r.host, mode); err != nil {
			d.fail(e, errAccess)
			return nil
		}
		d.fs.invalidate()
	case 2: // physical size: the file is not compressed
		sz := uint32(st.Size())
		c.R[cpu.AX], c.R[cpu.DX] = uint16(sz), uint16(sz>>16)
	case 3:
		t := fromDOSTime(c.R[cpu.CX], c.R[cpu.DI])
		if err := os.Chtimes(r.host, t, t); err != nil {
			d.fail(e, errAccess)
			return nil
		}
		d.fs.invalidate()
	case 4, 6, 8: // the host has one portable time: the last write
		tm, dt := dosTime(st.ModTime())
		if bl != 6 {
			c.R[cpu.CX] = tm
		}
		c.R[cpu.DI] = dt
		if bl == 8 {
			t := st.ModTime()
			c.R[cpu.SI] = uint16(t.Second()%2*100 + t.Nanosecond()/10000000)
		}
	case 5, 7: // access and creation times cannot be set portably: accepted, ignored
		e.Note("setting the access/creation time is ignored")
	default:
		d.fail(e, errInvalidFunc)
		return nil
	}
	d.ok(e)
	return nil
}

// lfnTrueName is 7160h: CL=0 canonical path, 1 short name, 2 long name.
// Its errors are 02h (invalid component in the directory path) and 03h
// (malformed path or invalid drive letter), RBIL.
func (d *DOS) lfnTrueName(e *hle.Env) error {
	c := e.CPU
	fail := func(errc uint16) error {
		switch errc {
		case errPathNotFound:
			errc = errFileNotFound
		case errBadDrive:
			errc = errPathNotFound
		}
		d.fail(e, errc)
		return nil
	}
	p := d.lfnStr(e.Seg(cpu.DS), c.R[cpu.SI])
	var out []byte
	switch c.CL() {
	case 0:
		var errc uint16
		if out, errc = d.fs.plainPath(p); errc != 0 {
			return fail(errc)
		}
	case 1, 2:
		r, errc := d.fs.lfnResolve(p)
		if errc != 0 {
			return fail(errc)
		}
		// A last component that does not exist yet is kept as given.
		base := r.host
		if !r.exists {
			base = r.parent
		}
		if c.CL() == 1 {
			sp, errc := d.fs.shortOf(r.drive, base)
			if errc != 0 {
				return fail(errc)
			}
			if !r.exists {
				up, _ := d.fs.oem(r.name)
				for i := range up {
					up[i] = d.e.CP.Upper(up[i])
				}
				sp = strings.TrimSuffix(sp, `\`) + `\` + trunc83(string(up))
			}
			out = append([]byte{byte('A' + r.drive), ':'}, sp...)
		} else {
			lp, _, errc := d.fs.longOf(r.drive, base)
			if errc != 0 {
				return fail(errc)
			}
			if !r.exists {
				nm, _ := d.fs.oem(r.name)
				if len(lp) > 1 {
					lp = append(lp, '\\')
				}
				lp = append(lp, nm...)
			}
			out = append([]byte{byte('A' + r.drive), ':'}, lp...)
		}
	default:
		d.fail(e, errInvalidFunc)
		return nil
	}
	if len(out) > maxLFNPath {
		d.fail(e, errPathNotFound)
		return nil
	}
	e.Mem.SetBytes(mem.Lin(e.Seg(cpu.ES), c.R[cpu.DI]), append(out, 0))
	d.ok(e)
	return nil
}
