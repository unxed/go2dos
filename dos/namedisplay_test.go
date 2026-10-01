package dos

import (
	"strings"
	"testing"

	"github.com/unxed/go2dos/bios"
)

func screenOf(rows ...string) *bios.Screen {
	w := 0
	for _, r := range rows {
		w = max(w, len(r))
	}
	s := &bios.Screen{Mode: 3, Cols: w, Rows: len(rows)}
	for _, r := range rows {
		r += strings.Repeat(" ", w-len(r))
		for i := 0; i < w; i++ {
			s.Cells = append(s.Cells, bios.Cell{Ch: r[i], Attr: 7, Rune: rune(r[i])})
		}
	}
	return s
}

// panelEntry is how Volkov Commander shows a name: base to 8, a blank, extension to 3.
func panelEntry(alias string) string {
	base, ext := splitName(alias)
	return base + strings.Repeat(" ", 8-len(base)) + " " + ext
}

func TestDisplayNames(t *testing.T) {
	h := newHarness(t, 437, Config{})
	reg := func(host string) string {
		r := h.d.fs.nameRecFor(host, nil)
		if r == nil {
			t.Fatal("no alias for", host)
		}
		return r.shortName()
	}
	a := reg("Привет.txt")
	b := reg("Очень длинное имя файла.docx")
	c := reg("世界.txt")
	d := reg("Ünïcödé")
	scr := screenOf(
		"#"+panelEntry(a)+"|"+strings.ToLower(panelEntry(b))+"#",
		"status: "+a+" 7 bytes",
		"status: "+strings.ToLower(c)+"|",
		"dir "+d+" <DIR>",
		"#"+panelEntry("ABC~0000.TXT")+"# ABC~0000.TXT",
	)
	h.d.DisplayNames(scr)
	got := strings.Split(scr.TextRows(), "\n")
	want := []string{
		"#Привет   txt|Очень д… do…#",
		"status: Привет.txt 7 bytes",
		"status: ??.txt    |",
		"dir Ünïcödé  <DIR>",
		"#ABC~0000 TXT# ABC~0000.TXT",
	}
	// The dotted name is replaced in the cells of its alias: padded with blanks.
	for i, w := range want {
		if strings.TrimRight(got[i], " ") != strings.TrimRight(w, " ") {
			t.Errorf("row %d:\n got %q\nwant %q", i, got[i], w)
		}
	}
	// The bytes of video memory (what the program sees) stay: the alias is still there.
	if got := cellBytes(scr.Cells[:scr.Cols], 1, 13); got != panelEntry(a) {
		t.Errorf("video bytes changed: %q", got)
	}
}

// Nothing to do in the UTF-8 mode and for a name that has no alias.
func TestDisplayNamesNoAlias(t *testing.T) {
	h := newHarness(t, 437, Config{})
	scr := screenOf("x ABC~A1B2.TXT y")
	h.d.DisplayNames(scr)
	if got := scr.Line(0); got != "x ABC~A1B2.TXT y" {
		t.Errorf("%q", got)
	}
}
