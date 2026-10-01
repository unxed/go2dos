package dos

import (
	"os"
	"path/filepath"
	"strings"
)

// Arguments and file names of the built-in COMMAND.COM: double quotes for names
// with spaces, and long names (and aliases of names the code page cannot show,
// names.go) in every file command, resolved the way INT 21h/71xx resolves them
// (lfn.go). Strings here are bytes of the code page.

// shArgs splits a command tail at blanks; "a b" is one argument (quotes dropped).
func shArgs(a string) []string {
	var out []string
	var cur strings.Builder
	in, have := false, false
	for i := 0; i < len(a); i++ {
		c := a[i]
		switch {
		case c == '"':
			in, have = !in, true
		case (c == ' ' || c == '\t') && !in:
			if have {
				out = append(out, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteByte(c)
			have = true
		}
	}
	if have {
		out = append(out, cur.String())
	}
	return out
}

// shOne is the only argument of a command (CD, MD, ...): quotes dropped, the
// rest of the line (blanks included) is the name, as the old shell took it.
func shOne(a string) string {
	if args := shArgs(a); len(args) == 1 {
		return args[0]
	}
	return strings.TrimSpace(a)
}

// shPath turns a path with long names into the DOS path the classic calls
// understand: an existing target (or the existing directory of a new one) is
// given by its 8.3 names or aliases. Anything else is returned as it came.
func (d *DOS) shPath(arg string) []byte {
	p := []byte(arg)
	r, errc := d.fs.lfnResolve(p)
	if errc != 0 || strings.ContainsAny(arg, "*?") {
		return p
	}
	prefix := string([]byte{byte('A' + r.drive), ':'})
	if r.exists {
		if s, errc := d.fs.shortOf(r.drive, r.host); errc == 0 {
			return []byte(prefix + s)
		}
		return p
	}
	if r.parent != "" {
		if s, errc := d.fs.shortOf(r.drive, r.parent); errc == 0 {
			return []byte(prefix + strings.TrimSuffix(s, `\`) + `\` + r.name)
		}
	}
	return p
}

// shLongName is the long name of a directory entry for DIR: "" if it is the
// 8.3 name itself (compared without case).
func (d *DOS) shLongName(en dirEntry) string {
	long, _ := d.fs.oem(en.host)
	if len(long) > maxLFNName {
		return ""
	}
	if strings.EqualFold(string(long), en.dos) {
		return ""
	}
	return string(long)
}

// shOpen opens a file for the shell's COPY and TYPE: the long name of an existing
// file through its alias (shPath), a new file, which may have a long name, at the
// host path that lfnResolve gives; devices (CON, NUL) go the classic way.
func (d *DOS) shOpen(arg string, create, trunc bool) (uint16, uint16) {
	mode := byte(0)
	if create {
		mode = 1
	}
	if _, dp, errc := d.fs.canon([]byte(arg), false); errc == 0 {
		if dev, _ := deviceFor(dp); dev != devNone {
			return d.open([]byte(arg), mode, create, trunc, false)
		}
	}
	if create {
		if r, errc := d.fs.lfnResolve([]byte(arg)); errc == 0 && !r.exists && !r.isRoot {
			return d.openHost(r.drive, arg, r.host, false, 1, true, true, false)
		}
	}
	return d.open(d.shPath(arg), mode, create, trunc, false)
}

// shMkdir makes a directory with a long name.
func (d *DOS) shMkdir(arg string) uint16 {
	r, errc := d.fs.lfnResolve([]byte(arg))
	if errc != 0 {
		return errc
	}
	if r.exists || r.isRoot {
		return errAccess
	}
	if d.fs.wp(r.host) {
		return errAccess
	}
	d.fs.invalidate()
	return osErr(os.Mkdir(r.host, 0o777))
}

// shRename renames an existing file or directory in its own directory; the new name
// may be long. An existing name is never overwritten.
func (d *DOS) shRename(from, to string) uint16 {
	src, errc := d.fs.existing([]byte(from))
	if errc != 0 {
		return errc
	}
	if src.isRoot || strings.ContainsAny(to, `\/:`) {
		return errAccess
	}
	dir, _, errc := d.fs.longOf(src.drive, src.parent)
	if errc != 0 {
		return errc
	}
	dst, errc := d.fs.lfnResolve(append(append(dir, '\\'), to...))
	if errc != 0 {
		return errc
	}
	if dst.exists && !foldEq(filepath.Clean(src.host), filepath.Clean(dst.host)) {
		return errAccess
	}
	if d.fs.wp(src.host) || d.fs.wp(dst.host) {
		return errAccess
	}
	d.fs.invalidate()
	return osErr(os.Rename(src.host, dst.host))
}
