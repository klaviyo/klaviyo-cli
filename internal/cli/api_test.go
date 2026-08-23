package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordedRequest struct {
	method, path string
	query        url.Values
	body         string
	contentType  string
}

// apiServer records requests and returns the given response.
func apiServer(t *testing.T, status int, respBody string) *recordedRequest {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.method, rec.path, rec.query = r.Method, r.URL.Path, r.URL.Query()
		rec.body, rec.contentType = string(body), r.Header.Get("Content-Type")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("KLAVIYO_API_URL", srv.URL)
	t.Setenv("KLAVIYO_API_KEY", "pk_test")
	return rec
}

func TestAPICommandDefaultsToGET(t *testing.T) {
	rec := apiServer(t, 200, `{"data":[]}`)
	if _, err := runCommand(t, "api", "/api/metrics/"); err != nil {
		t.Fatal(err)
	}
	if rec.method != "GET" || rec.path != "/api/metrics/" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestAPICommandUppercasesMethodAndSendsBody(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	if _, err := runCommand(t, "api", "post", "/api/events/", "-d", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if rec.method != "POST" {
		t.Errorf("method = %s", rec.method)
	}
	if rec.body != `{"a":1}` || !strings.Contains(rec.contentType, "vnd.api+json") {
		t.Errorf("body = %q, content-type = %q", rec.body, rec.contentType)
	}
}

func TestAPICommandSendsQueryParams(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	if _, err := runCommand(t, "api", "/api/profiles/", "-q", `filter=equals(email,"a@b.com")`); err != nil {
		t.Fatal(err)
	}
	if got := rec.query.Get("filter"); got != `equals(email,"a@b.com")` {
		t.Errorf("filter = %q", got)
	}
}

func TestAPICommandErrorBodySurfaces(t *testing.T) {
	apiServer(t, 404, `{"errors":[{"detail":"no such endpoint"}]}`)
	out, err := runCommand(t, "api", "/api/bogus/")
	if err == nil || err.Error() != "HTTP 404" {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "no such endpoint") {
		t.Errorf("error body not printed:\n%s", out)
	}
}

func TestAPICommandPaginateRejectsNonGET(t *testing.T) {
	apiServer(t, 200, `{}`)
	if _, err := runCommand(t, "api", "post", "/api/events/", "--paginate"); err == nil || !strings.Contains(err.Error(), "GET") {
		t.Errorf("err = %v", err)
	}
}

func TestReadBody(t *testing.T) {
	if body, err := readBody(""); err != nil || body != nil {
		t.Errorf("empty: %v %q", err, body)
	}
	if body, err := readBody(`{"x":1}`); err != nil || string(body) != `{"x":1}` {
		t.Errorf("inline: %v %q", err, body)
	}

	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(`{"f":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if body, err := readBody("@" + path); err != nil || string(body) != `{"f":2}` {
		t.Errorf("file: %v %q", err, body)
	}
	if _, err := readBody("@" + path + ".missing"); err == nil || !strings.Contains(err.Error(), "reading body file") {
		t.Errorf("missing file err = %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })
	if _, err := w.WriteString(`{"s":3}`); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	if body, err := readBody("-"); err != nil || string(body) != `{"s":3}` {
		t.Errorf("stdin: %v %q", err, body)
	}
}

func TestParseQueries(t *testing.T) {
	values, err := parseQueries([]string{"a=1", "a=2", "b="})
	if err != nil {
		t.Fatal(err)
	}
	if got := values["a"]; len(got) != 2 {
		t.Errorf("repeated key = %v", got)
	}
	if _, ok := values["b"]; !ok {
		t.Error("empty value should be allowed")
	}
	for _, bad := range []string{"novalue", "=v"} {
		if _, err := parseQueries([]string{bad}); err == nil {
			t.Errorf("parseQueries(%q) should error", bad)
		}
	}
}
