package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestApplyJQ(t *testing.T) {
	body := []byte(`{"data":[{"id":"1","attributes":{"email":"a@b.com"}},{"id":"2","attributes":{"email":"c@d.com"}}]}`)

	cases := []struct{ expr, want string }{
		{".data[].id", "1\n2\n"},                           // strings print raw
		{".data[].attributes.email", "a@b.com\nc@d.com\n"}, // nested access
		{".data | length", "2\n"},                          // non-strings print as JSON
		{"[.data[].id]", `["1","2"]` + "\n"},               // arrays print compact
	}
	for _, c := range cases {
		out := &bytes.Buffer{}
		if err := applyJQ(out, body, c.expr); err != nil {
			t.Fatalf("applyJQ(%q): %v", c.expr, err)
		}
		if out.String() != c.want {
			t.Errorf("applyJQ(%q) = %q, want %q", c.expr, out.String(), c.want)
		}
	}
}

func TestApplyJQErrors(t *testing.T) {
	if err := applyJQ(&bytes.Buffer{}, []byte(`{}`), ".data[("); err == nil || !strings.Contains(err.Error(), "invalid --jq") {
		t.Errorf("bad expression error = %v", err)
	}
	if err := applyJQ(&bytes.Buffer{}, []byte("not json"), "."); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Errorf("non-JSON body error = %v", err)
	}
}
