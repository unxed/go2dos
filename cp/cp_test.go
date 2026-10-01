package cp

import "testing"

func TestScreenAndText(t *testing.T) {
	c, err := Get(866)
	if err != nil {
		t.Fatal(err)
	}
	if r := c.ScreenRune(0x01); r != '☺' {
		t.Errorf("screen 01 = %q", r)
	}
	if r := c.Rune(0x0D); r != '\r' {
		t.Errorf("text 0D = %q", r)
	}
	if r := c.ScreenRune(0x8F); r != 'П' {
		t.Errorf("866 8F = %q", r)
	}
	if b, ok := c.Byte('я'); !ok || b != 0xEF {
		t.Errorf("я = %02X %v", b, ok)
	}
	if u := c.Upper(0xA0); u != 0x80 { // а -> А
		t.Errorf("upper a0 = %02X", u)
	}
}

func TestDetectOverride(t *testing.T) {
	d, err := Detect(850)
	if err != nil || d.Num != 850 || d.Source != "override" {
		t.Fatalf("%+v %v", d, err)
	}
	if _, err := Detect(12345); err == nil {
		t.Fatal("unsupported override accepted")
	}
}
