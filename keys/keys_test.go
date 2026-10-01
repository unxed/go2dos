package keys

import (
	"testing"

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
