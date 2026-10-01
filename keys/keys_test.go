package keys

import (
	"testing"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
)

func TestRoundTrip(t *testing.T) {
	page, _ := cp.Get(866)
	src := "dir<Enter><F10><Alt-F1><Ctrl-PgDn><Shift-Tab><<x<Ctrl-O>я"
	steps, err := Parse(src, page)
	if err != nil {
		t.Fatal(err)
	}
	out := ""
	for _, s := range steps {
		out += Format(*s.Key, page)
	}
	if out != src {
		t.Errorf("round trip:\n got %s\nwant %s", out, src)
	}
}

func TestControlTokens(t *testing.T) {
	page, _ := cp.Get(437)
	steps, err := Parse("<wait:250ms><waitfor:C:\\><screen>", page)
	if err != nil || len(steps) != 3 || steps[0].Wait == 0 || steps[1].WaitFor != `C:\` || !steps[2].Screen {
		t.Fatalf("%+v %v", steps, err)
	}
	if _, err := Parse("<NoSuchKey>", page); err == nil {
		t.Error("unknown key accepted")
	}
}

func TestPaste(t *testing.T) {
	page, _ := cp.Get(437)
	got := Paste("a\r\nb\nc\rd\te\x01éЖ", page)
	enter, _ := Named("Enter", 0)
	tab, _ := Named("Tab", 0)
	var want []bios.KeyEvent
	ch := func(r rune) {
		k, _ := Char(r, page)
		want = append(want, k)
	}
	ch('a')
	want = append(want, enter)
	ch('b')
	want = append(want, enter)
	ch('c')
	want = append(want, enter) // a lone CR
	ch('d')
	want = append(want, tab)
	ch('e') // \x01 is dropped
	ch('é')
	ch('?') // Ж is not in CP437
	if len(got) != len(want) {
		t.Fatalf("%d keystrokes, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("key %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
