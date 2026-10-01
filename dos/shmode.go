package dos

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/unxed/go2dos/bios"
)

// CLS and MODE CON of the built-in COMMAND.COM.
//
//	CLS                              clear the screen (the terminal too in console mode)
//	MODE CON                         show the size of the text window
//	MODE CON COLS=100 LINES=40       set it (columns 80-255, lines 25-255, at most 32768 cells)
//
// The size is that of DOS-HOST/TEXTWIN (textwin.go). A program that asked for size
// events (AL=11h) gets the FF00h key, as for a host window change.

func (d *DOS) shCLS() {
	if d.OnCLS != nil {
		d.OnCLS()
	} else {
		d.b.Video.Clear()
	}
}

func (d *DOS) shMode(st *shellState, args string) {
	f := shArgs(args)
	if len(f) == 0 || !eqASCII(strings.TrimSuffix(f[0], ":"), "CON") {
		d.shPrint("Only MODE CON [COLS=n] [LINES=n] is supported by the built-in COMMAND.COM" + crlf)
		st.level = 1
		return
	}
	cols, rows := d.b.Video.Size()
	set := false
	for _, a := range f[1:] {
		k, v, ok := strings.Cut(a, "=")
		n, err := strconv.Atoi(v)
		switch {
		case !ok || err != nil:
			d.shPrint("Invalid parameter - " + a + crlf)
			st.level = 1
			return
		case eqASCII(k, "COLS"):
			cols = n
		case eqASCII(k, "LINES"):
			rows = n
		default:
			d.shPrint("Invalid parameter - " + a + crlf)
			st.level = 1
			return
		}
		set = true
	}
	if set {
		if err := d.b.Video.SetTextSize(cols, rows); err != nil {
			d.shPrint(err.Error() + crlf)
			st.level = 1
			return
		}
		if d.winWatch {
			d.b.PushKey(bios.ResizeKey)
		}
		return
	}
	d.shPrint(fmt.Sprintf("%sStatus for device CON:%s----------------------%s    Lines:%10d%s    Columns:%8d%s",
		crlf, crlf, crlf, rows, crlf, cols, crlf))
}
