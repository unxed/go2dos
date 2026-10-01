package dos

import (
	"reflect"
	"testing"
)

func TestShArgs(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{`a b`, []string{"a", "b"}},
		{`"A long name.txt" "Other dir"`, []string{"A long name.txt", "Other dir"}},
		{`  x   "y z"  `, []string{"x", "y z"}},
		{`"" b`, []string{"", "b"}},
		{`pre"fix ed"post`, []string{"prefix edpost"}},
		{``, nil},
	} {
		if got := shArgs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("shArgs(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := shOne(`"My dir"`); got != "My dir" {
		t.Errorf("shOne quoted: %q", got)
	}
	if got := shOne(` my dir `); got != "my dir" {
		t.Errorf("shOne bare: %q", got)
	}
}
