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
		if err := applyJQ(out, body, c.expr, false); err != nil {
			t.Fatalf("applyJQ(%q): %v", c.expr, err)
		}
		if out.String() != c.want {
			t.Errorf("applyJQ(%q) = %q, want %q", c.expr, out.String(), c.want)
		}
	}
}

func TestApplyJQErrors(t *testing.T) {
	if err := applyJQ(&bytes.Buffer{}, []byte(`{}`), ".data[(", false); err == nil || !strings.Contains(err.Error(), "invalid --jq") {
		t.Errorf("bad expression error = %v", err)
	}
	if err := applyJQ(&bytes.Buffer{}, []byte("not json"), ".", false); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Errorf("non-JSON body error = %v", err)
	}
}

func TestApplyJQSanitizesTerminalStrings(t *testing.T) {
	body := []byte(`{"name":"Bob\u001b[2K"}`)
	term := &bytes.Buffer{}
	if err := applyJQ(term, body, ".name", true); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(term.String(), 0x1b) {
		t.Errorf("terminal output contains raw ESC: %q", term.String())
	}
	piped := &bytes.Buffer{}
	if err := applyJQ(piped, body, ".name", false); err != nil {
		t.Fatal(err)
	}
	if !strings.ContainsRune(piped.String(), 0x1b) {
		t.Errorf("piped output must stay byte-faithful: %q", piped.String())
	}
}

func TestSanitizeTerminal(t *testing.T) {
	if got := sanitizeTerminal("Bob\x1b[2K\x07"); got != `Bob\x1b[2K\x07` {
		t.Errorf("sanitizeTerminal = %q", got)
	}
	if got := sanitizeTerminal("plain\ttext\n"); got != "plain\ttext\n" {
		t.Errorf("tab/newline must pass through: %q", got)
	}
}

func TestTruncateIsRuneSafe(t *testing.T) {
	if got := truncate("héllo wörld exträ", 10); got != "héllo w..." {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
}
