package dos

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/unxed/go2dos/cp"
)

// Reversible names for host names that the OEM code page cannot hold
// (docs/NAMES.md).
//
// A DOS program that does not know UTF-8 (Volkov Commander, ...) sees a
// host name through the code page. A character that is not in the page
// (Cyrillic under CP437, an umlaut under CP866) cannot be shown, but the file
// must not lose it when the program copies, moves or renames it: the program
// hands back the name it was shown, and nothing else. So such a name is given
// out under an alias that is
//
//   - unique: STEM~HHHH, where HHHH is a hash of the whole host name; if the
//     alias is taken (by another name in the registry, or in the directory)
//     the next value is used, so two names that look the same never share one;
//   - known: the registry (nameReg) maps the alias back to the host name, for
//     the whole run and for all directories, so that creating the alias in
//     another directory (copy, move) makes the file with the original name.
//
// A name has two forms, the 8.3 alias (classic INT 21h calls, and the short
// name of the LFN calls) and the long form (INT 21h/71xx): the host name with
// '_' for every character not in the code page and ~HHHH before the extension.
// When the program changes only the extension of such a name (renames
// "STEM~HHHH.TXT" to "STEM~HHHH.DOC"), the new host name is the original base
// with the new extension. In UTF-8 mode (utf8names.go) nothing of this applies:
// the program has long UTF-8 names and the short names keep the numeric tail.

// nameRec is one host name known under an alias.
type nameRec struct {
	host              string // the whole host name
	hostBase, hostExt string // split at the last dot (see splitName)
	short, shortExt   string // 8.3 alias in DOS bytes, upper case: "_AO~A1B2", "TXT"
	long, longExt     string // long form in DOS bytes: "______ao~A1B2", "txt"
}

func (r *nameRec) shortName() string { return joinExt(r.short, r.shortExt) }
func (r *nameRec) longName() string  { return joinExt(r.long, r.longExt) }

func joinExt(base, ext string) string {
	if ext == "" {
		return base
	}
	return base + "." + ext
}

// splitName splits a name at its last dot; a dot in the first or last
// position does not start an extension.
func splitName(name string) (base, ext string) {
	if i := strings.LastIndexByte(name, '.'); i > 0 && i < len(name)-1 {
		return name[:i], name[i+1:]
	}
	return name, ""
}

// nameReg is the registry of aliases handed out in this run.
type nameReg struct {
	byHost  map[string]*nameRec
	byShort map[string]*nameRec // key: upper-case alias without extension
	byLong  map[string]*nameRec // key: upper-case long form without extension
}

func (f *fsys) registry() *nameReg {
	if f.names == nil {
		f.names = &nameReg{byHost: map[string]*nameRec{}, byShort: map[string]*nameRec{}, byLong: map[string]*nameRec{}}
	}
	return f.names
}

// key upper-cases DOS bytes (as the code page does) for comparing names.
func (f *fsys) key(s string) string {
	b := []byte(s)
	for i, c := range b {
		b[i] = f.e.CP.Upper(c)
	}
	return string(b)
}

// lossyName reports whether a host name has a character that is not in the
// code page (in the non-UTF-8 mode only).
func (f *fsys) lossyName(host string) bool {
	if f.utf8() {
		return false
	}
	_, ok := f.e.CP.Encode(host)
	return !ok
}

// validShortByte reports whether c may be part of an 8.3 alias.
func validShortByte(c byte) bool {
	return c > 0x20 && c != '.' && c != '~' && strings.IndexByte(`"*+,/:;<=>?[\]|`, c) < 0
}

// aliasParts builds the candidate alias of a host name; probe is added to
// the hash for the next try when the first candidate is taken.
func aliasParts(page *cp.Codepage, host string, probe int) (short, shortExt, long, longExt string) {
	h := fnv.New32a()
	h.Write([]byte(host))
	v := h.Sum32()
	tag := fmt.Sprintf("~%04X", uint16(v^v>>16)+uint16(probe))
	base, ext := splitName(host)
	var stem, lbase []byte
	for _, r := range base {
		b, ok := page.Byte(r)
		if !ok {
			lbase = append(lbase, '_')
			continue
		}
		lbase = append(lbase, b)
		if u := page.Upper(b); len(stem) < 3 && validShortByte(u) {
			stem = append(stem, u)
		}
	}
	if len(stem) == 0 {
		stem = []byte("_")
	}
	var sext, lext []byte
	for _, r := range ext {
		b, ok := page.Byte(r)
		if !ok {
			lext = append(lext, '_')
			continue
		}
		lext = append(lext, b)
		if u := page.Upper(b); len(sext) < 3 && validShortByte(u) {
			sext = append(sext, u)
		}
	}
	return string(stem) + tag, string(sext), string(lbase) + tag, string(lext)
}

// AliasFor returns the 8.3 alias and the long form that a host name gets in
// the code page page if it contains a character the page cannot hold (the
// first candidate: another name may have taken it, and then the next one is
// used). For tests and tools; names are DOS bytes.
func AliasFor(page *cp.Codepage, host string) (short, long string) {
	s, se, l, le := aliasParts(page, host, 0)
	return joinExt(s, se), joinExt(l, le)
}

// nameRecFor returns the registry record of a lossy host name, creating it.
// taken tells whether an 8.3 name is already in use in the directory being
// indexed; nil is returned if no free alias is found (never in practice).
func (f *fsys) nameRecFor(host string, taken func(dosName string) bool) *nameRec {
	reg := f.registry()
	if r := reg.byHost[host]; r != nil {
		if taken != nil && taken(r.shortName()) {
			return nil // a real file of this directory has the alias
		}
		return r
	}
	hostBase, hostExt := splitName(host)
	used := func(k string) bool { return reg.byShort[k] != nil || reg.byLong[k] != nil }
	for probe := 0; probe < 0x10000; probe++ {
		s, se, l, le := aliasParts(f.e.CP, host, probe)
		ks, kl := f.key(s), f.key(l)
		if used(ks) || (kl != ks && used(kl)) {
			continue
		}
		if taken != nil && taken(joinExt(s, se)) {
			continue
		}
		r := &nameRec{host: host, hostBase: hostBase, hostExt: hostExt, short: s, shortExt: se, long: l, longExt: le}
		reg.byHost[host] = r
		reg.byShort[ks] = r
		reg.byLong[kl] = r
		return r
	}
	return nil
}

// splitDOS splits a DOS name at its last dot like splitName.
func splitDOS(name string) (base, ext string) { return splitName(name) }

// hostNameFor gives the host name for a name that the program made up from
// an alias it was shown: the alias itself (the same file), or the alias with
// another extension (the base of the original, the new extension). long
// selects the long form instead of the 8.3 alias. ok is false for any other
// name. name is in DOS bytes.
func (f *fsys) hostNameFor(name string, long bool) (string, bool) {
	if f.names == nil || f.utf8() {
		return "", false
	}
	base, ext := splitDOS(name)
	k := f.key(base)
	var rec *nameRec
	var recExt string
	if long {
		if rec = f.names.byLong[k]; rec != nil {
			recExt = rec.longExt
		}
	} else if rec = f.names.byShort[k]; rec != nil {
		recExt = rec.shortExt
	}
	if rec == nil {
		return "", false
	}
	if f.key(ext) == f.key(recExt) {
		return rec.host, true
	}
	if ext == "" {
		return rec.hostBase, true
	}
	e := f.e.CP.Decode([]byte(ext))
	if rec.hostExt != "" && strings.ToLower(rec.hostExt) == rec.hostExt { // the old extension has no upper case: neither does the new one
		e = strings.ToLower(e)
	}
	return rec.hostBase + "." + e, true
}

// mapNew is hostNameFor for a name that a long-name call gave (as a host
// string decoded from the code page): the long form, then the 8.3 alias.
func (f *fsys) mapNew(name string) (string, bool) {
	enc, ok := f.e.CP.Encode(name)
	if !ok {
		return "", false
	}
	if h, ok := f.hostNameFor(string(enc), true); ok {
		return h, true
	}
	return f.hostNameFor(string(enc), false)
}

// recByLong finds the record whose long form is name (a host-style string
// decoded from the code page), with the same extension.
func (f *fsys) recByLong(name string) *nameRec {
	if f.names == nil || f.utf8() {
		return nil
	}
	enc, ok := f.e.CP.Encode(name)
	if !ok {
		return nil
	}
	base, ext := splitDOS(string(enc))
	rec := f.names.byLong[f.key(base)]
	if rec == nil || f.key(ext) != f.key(rec.longExt) {
		return nil
	}
	return rec
}

// registered returns the record of a host name that has an alias, or nil.
func (f *fsys) registered(host string) *nameRec {
	if f.names == nil || f.utf8() {
		return nil
	}
	return f.names.byHost[host]
}
