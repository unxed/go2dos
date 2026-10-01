package dos

import "strings"

// CLIP of the built-in COMMAND.COM: the DOS clipboard (the server of clip.go, so the same
// text that WinOldAp programs and, in the terminal front end, the system clipboard see).
//
//	CLIP            print the text (OEM, CR LF; a final CR LF is added if it lacks one)
//	CLIP text       set the text to the rest of the line
//	CLIP < file     set the text to the content of the file (up to a 0 or Ctrl-Z)
//	CLIP /C         clear it
//
// Output can be redirected (CLIP > file); there are no pipes in this shell.
func (d *DOS) shClip(st *shellState, args string) {
	if d.clip == nil {
		d.shPrint("No clipboard in this session" + crlf)
		st.level = 1
		return
	}
	a := strings.TrimSpace(args)
	switch {
	case st.stdinRedir && a == "":
		b := d.shReadAll(0, clipMax)
		d.shClipSet(st, string(b))
	case eqASCII(a, "/C"):
		d.shClipSet(st, "")
	case a != "":
		d.shClipSet(st, a)
	default:
		b, ok := d.clipBytes()
		if !ok {
			return // empty: nothing to print
		}
		if n := len(b); n < 2 || b[n-2] != '\r' || b[n-1] != '\n' {
			b = append(b, '\r', '\n')
		}
		d.write(1, b)
	}
}

// shClipSet puts text (bytes of the code page, CR LF or LF line ends) on the clipboard.
func (d *DOS) shClipSet(st *shellState, text string) {
	if i := strings.IndexAny(text, "\x00\x1a"); i >= 0 {
		text = text[:i]
	}
	s := strings.ReplaceAll(d.e.CP.Decode([]byte(text)), "\r\n", "\n")
	if d.clip.SetText(s) != nil {
		d.shPrint("Cannot set the clipboard" + crlf)
		st.level = 1
	}
}

// shReadAll reads handle h to its end (at most max bytes).
func (d *DOS) shReadAll(h uint16, max int) []byte {
	var out []byte
	buf := make([]byte, 4096)
	for len(out) < max {
		n, errc := d.read(h, buf)
		if errc != 0 || n == 0 {
			break
		}
		out = append(out, buf[:n]...)
	}
	return out
}
