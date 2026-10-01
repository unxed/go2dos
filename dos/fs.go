package dos

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/unxed/go2dos/hle"
)

// DOS error codes.
const (
	errInvalidFunc  = 0x01
	errFileNotFound = 0x02
	errPathNotFound = 0x03
	errTooMany      = 0x04
	errAccess       = 0x05
	errBadHandle    = 0x06
	errBadMCB       = 0x09
	errNoMem        = 0x08
	errBadDrive     = 0x0F
	errCurDir       = 0x10
	errNotSame      = 0x11
	errNoMore       = 0x12
	errExists       = 0x50
)

func errText(c uint16) string {
	switch c {
	case errFileNotFound:
		return "file not found"
	case errPathNotFound:
		return "path not found"
	case errAccess:
		return "access denied"
	case errBadDrive:
		return "invalid drive"
	case errNoMem:
		return "insufficient memory"
	}
	return fmt.Sprintf("error %02Xh", c)
}

// DOS file attributes.
const (
	attrRO     = 0x01
	attrHidden = 0x02
	attrSystem = 0x04
	attrLabel  = 0x08
	attrDir    = 0x10
	attrArch   = 0x20
)

type dirEntry struct {
	dos  string // NAME.EXT, upper case, in the DOS code page
	host string // host file name
	info fs.FileInfo
}

type dirIndex struct {
	entries []dirEntry
	byDOS   map[string]int
}

type fsys struct {
	e      *hle.Env
	drives [26]string // host roots; "" if not mapped
	cur    int        // current drive (0 = A)
	cwd    [26]string // per drive: "\" or "\DIR\SUB"
	cache  map[string]*dirIndex
	dirIDs map[string]uint16
	dirs   []string
}

func newFS(e *hle.Env, cfg Config) (*fsys, error) {
	f := &fsys{e: e, cache: map[string]*dirIndex{}, dirIDs: map[string]uint16{}}
	if len(cfg.Drives) == 0 {
		return nil, fmt.Errorf("no drives configured")
	}
	for l, root := range cfg.Drives {
		l = byte(strings.ToUpper(string(l))[0])
		if l < 'A' || l > 'Z' {
			return nil, fmt.Errorf("bad drive letter %q", l)
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			return nil, fmt.Errorf("drive %c: %s is not a directory", l, abs)
		}
		f.drives[l-'A'] = abs
	}
	for i := range f.cwd {
		f.cwd[i] = `\`
	}
	cur := cfg.Current
	if cur == 0 {
		cur = 'C'
	}
	if f.drives[cur-'A'] == "" {
		return nil, fmt.Errorf("current drive %c: is not mapped", cur)
	}
	f.cur = int(cur - 'A')
	return f, nil
}

func (f *fsys) invalidate() { f.cache = map[string]*dirIndex{} }

// valid83 reports whether a DOS name (already upper case) is a plain 8.3 name.
func valid83(n []byte) bool {
	if len(n) == 0 {
		return false
	}
	dot := -1
	for i, c := range n {
		switch {
		case c == '.':
			if dot >= 0 || i == 0 {
				return false
			}
			dot = i
		case c < 0x21 || strings.IndexByte(`"*+,/:;<=>?[\]|`, c) >= 0:
			return false
		case c >= 'a' && c <= 'z':
			return false
		}
	}
	if dot < 0 {
		return len(n) <= 8
	}
	return dot <= 8 && len(n)-dot-1 <= 3 && len(n)-dot-1 >= 1
}

func clean83(b []byte, max int) []byte {
	var out []byte
	for _, c := range b {
		if c < 0x21 || c == '.' || strings.IndexByte(`"*+,/:;<=>?[\]|`, c) >= 0 {
			continue
		}
		out = append(out, c)
		if len(out) == max {
			break
		}
	}
	return out
}

// index lists a host directory and assigns DOS names: names that are valid
// 8.3 in the code page keep their (upper-cased) name, the rest get NAME~N.EXT
// aliases. The result is deterministic for a given directory content.
func (f *fsys) index(hostDir string) (*dirIndex, uint16) {
	if ix, ok := f.cache[hostDir]; ok {
		return ix, 0
	}
	list, err := os.ReadDir(hostDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errPathNotFound
		}
		return nil, errAccess
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	ix := &dirIndex{byDOS: map[string]int{}}
	type pending struct {
		host string
		up   []byte
		info fs.FileInfo
	}
	var later []pending
	for _, de := range list {
		info, err := de.Info()
		if err != nil {
			continue
		}
		enc, ok := f.e.CP.Encode(de.Name())
		up := make([]byte, len(enc))
		for i, c := range enc {
			up[i] = f.e.CP.Upper(c)
		}
		if ok && valid83(up) {
			if _, dup := ix.byDOS[string(up)]; !dup {
				ix.add(dirEntry{dos: string(up), host: de.Name(), info: info})
				continue
			}
		}
		later = append(later, pending{de.Name(), up, info})
	}
	for _, p := range later {
		base, ext := p.up, []byte(nil)
		if i := strings.LastIndexByte(string(p.up), '.'); i > 0 {
			base, ext = p.up[:i], p.up[i+1:]
		}
		b := clean83(base, 6)
		if len(b) == 0 {
			b = []byte("_")
		}
		x := clean83(ext, 3)
		for n := 1; ; n++ {
			suffix := fmt.Sprintf("~%d", n)
			stem := b
			if len(stem)+len(suffix) > 8 {
				stem = stem[:8-len(suffix)]
			}
			name := string(stem) + suffix
			if len(x) > 0 {
				name += "." + string(x)
			}
			if _, dup := ix.byDOS[name]; !dup {
				ix.add(dirEntry{dos: name, host: p.host, info: p.info})
				break
			}
		}
	}
	f.cache[hostDir] = ix
	return ix, 0
}

func (ix *dirIndex) add(e dirEntry) {
	ix.byDOS[e.dos] = len(ix.entries)
	ix.entries = append(ix.entries, e)
}

func attrOf(e dirEntry) byte {
	var a byte
	if e.info.IsDir() {
		a |= attrDir
	} else {
		a |= attrArch
	}
	if e.info.Mode().Perm()&0o200 == 0 {
		a |= attrRO
	}
	if strings.HasPrefix(e.host, ".") {
		a |= attrHidden
	}
	return a
}

// canon turns a DOS path (bytes in the code page) into a drive and an
// absolute upper-case path such as \DIR\FILE.EXT. Wildcards are allowed in
// the last component when wild is true.
func (f *fsys) canon(p []byte, wild bool) (int, string, uint16) {
	drive := f.cur
	if len(p) >= 2 && p[1] == ':' {
		l := f.e.CP.Upper(p[0])
		if l < 'A' || l > 'Z' {
			return 0, "", errBadDrive
		}
		drive = int(l - 'A')
		p = p[2:]
	}
	if f.drives[drive] == "" {
		return 0, "", errBadDrive
	}
	s := strings.ReplaceAll(string(p), "/", `\`)
	if !strings.HasPrefix(s, `\`) {
		s = strings.TrimSuffix(f.cwd[drive], `\`) + `\` + s
	}
	var out []string
	parts := strings.Split(s, `\`)
	for i, part := range parts {
		last := i == len(parts)-1
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			continue
		}
		if strings.Trim(part, ".") == "" { // "...": parent of parent in some DOSes; reject
			return 0, "", errPathNotFound
		}
		up := make([]byte, len(part))
		for j := 0; j < len(part); j++ {
			up[j] = f.e.CP.Upper(part[j])
		}
		name := string(up)
		if strings.ContainsAny(name, "*?") && !(wild && last) {
			if last {
				return 0, "", errFileNotFound
			}
			return 0, "", errPathNotFound
		}
		// DOS silently truncates to 8.3.
		base, ext := name, ""
		if k := strings.LastIndexByte(name, '.'); k > 0 {
			base, ext = name[:k], name[k+1:]
		}
		if len(base) > 8 {
			base = base[:8]
		}
		if len(ext) > 3 {
			ext = ext[:3]
		}
		if ext != "" {
			base += "." + ext
		}
		out = append(out, base)
	}
	return drive, `\` + strings.Join(out, `\`), 0
}

// lookup finds a DOS name in a host directory.
func (f *fsys) lookup(hostDir, name string) (dirEntry, bool, uint16) {
	ix, errc := f.index(hostDir)
	if errc != 0 {
		return dirEntry{}, false, errc
	}
	if i, ok := ix.byDOS[name]; ok {
		return ix.entries[i], true, 0
	}
	return dirEntry{}, false, 0
}

// resolve maps a canonical DOS path to a host path. If the last component
// does not exist and create is true, a host name is derived from the DOS
// name; otherwise errFileNotFound is returned.
func (f *fsys) resolve(drive int, dpath string, create bool) (string, bool, uint16) {
	host := f.drives[drive]
	parts := strings.Split(strings.TrimPrefix(dpath, `\`), `\`)
	if dpath == `\` {
		return host, true, 0
	}
	for i, part := range parts {
		e, ok, errc := f.lookup(host, part)
		if errc != 0 {
			return "", false, errPathNotFound
		}
		last := i == len(parts)-1
		if !ok {
			if !last {
				return "", false, errPathNotFound
			}
			if !create {
				return "", false, errFileNotFound
			}
			return filepath.Join(host, f.e.CP.Decode([]byte(part))), false, 0
		}
		if !last && !e.info.IsDir() {
			return "", false, errPathNotFound
		}
		host = filepath.Join(host, e.host)
	}
	return host, true, 0
}

func (f *fsys) readFile(host string) ([]byte, error) { return os.ReadFile(host) }

// splitDir splits a canonical path into directory and last component.
func splitDir(p string) (string, string) {
	i := strings.LastIndexByte(p, '\\')
	dir := p[:i]
	if dir == "" {
		dir = `\`
	}
	return dir, p[i+1:]
}

// fcbName converts NAME.EXT to the 11-byte FCB form, expanding '*'.
func fcbName(n string) [11]byte {
	var r [11]byte
	for i := range r {
		r[i] = ' '
	}
	base, ext := n, ""
	if i := strings.LastIndexByte(n, '.'); i >= 0 {
		base, ext = n[:i], n[i+1:]
	}
	fill := func(dst []byte, s string) {
		for i := 0; i < len(dst) && i < len(s); i++ {
			if s[i] == '*' {
				for j := i; j < len(dst); j++ {
					dst[j] = '?'
				}
				return
			}
			dst[i] = s[i]
		}
	}
	fill(r[:8], base)
	fill(r[8:], ext)
	return r
}

func fcbMatch(pat, name [11]byte) bool {
	for i := range pat {
		if pat[i] != '?' && pat[i] != name[i] {
			return false
		}
	}
	return true
}

func dosTime(t time.Time) (tm, dt uint16) {
	if t.Year() < 1980 {
		t = time.Date(1980, 1, 1, 0, 0, 0, 0, t.Location())
	}
	tm = uint16(t.Hour())<<11 | uint16(t.Minute())<<5 | uint16(t.Second()/2)
	dt = uint16(t.Year()-1980)<<9 | uint16(t.Month())<<5 | uint16(t.Day())
	return
}

func fromDOSTime(tm, dt uint16) time.Time {
	return time.Date(int(dt>>9)+1980, time.Month(dt>>5&15), int(dt&31),
		int(tm>>11), int(tm>>5&63), int(tm&31)*2, 0, time.Local)
}

func (f *fsys) dirID(host string) uint16 {
	if id, ok := f.dirIDs[host]; ok {
		return id
	}
	id := uint16(len(f.dirs))
	f.dirs = append(f.dirs, host)
	f.dirIDs[host] = id
	return id
}
