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
	revision     string
}

// apiServer records requests and returns the given response.
func apiServer(t *testing.T, status int, respBody string) *recordedRequest {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.method, rec.path, rec.query = r.Method, r.URL.Path, r.URL.Query()
		rec.body, rec.contentType = string(body), r.Header.Get("Content-Type")
		rec.revision = r.Header.Get("revision")
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

func TestAPICommandBuildsBodyFromFieldPairs(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	if _, err := runCommand(t, "api", "post", "/api/lists/",
		"-d", "data.type=list", "-d", "data.attributes.name=Newsletter"); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"data":{"attributes":{"name":"Newsletter"},"type":"list"}}` {
		t.Errorf("body = %q", rec.body)
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
	if body, err := readBody(nil); err != nil || body != nil {
		t.Errorf("empty: %v %q", err, body)
	}
	if body, err := readBody([]string{`{"x":1}`}); err != nil || string(body) != `{"x":1}` {
		t.Errorf("inline: %v %q", err, body)
	}

	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(`{"f":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if body, err := readBody([]string{"@" + path}); err != nil || string(body) != `{"f":2}` {
		t.Errorf("file: %v %q", err, body)
	}
	if _, err := readBody([]string{"@" + path + ".missing"}); err == nil || !strings.Contains(err.Error(), "reading body file") {
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
	if body, err := readBody([]string{"-"}); err != nil || string(body) != `{"s":3}` {
		t.Errorf("stdin: %v %q", err, body)
	}
}

func TestBuildBodyDotNotation(t *testing.T) {
	body, err := readBody([]string{
		"data.type=list",
		"data.attributes.name=Newsletter",
		`data.attributes.tags:=["a","b"]`,
		"data.attributes.count:=2",
		"data.attributes.active:=true",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"attributes":{"active":true,"count":2,"name":"Newsletter","tags":["a","b"]},"type":"list"}}`
	if string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

func TestBuildBodyStringValuesStayStrings(t *testing.T) {
	body, err := readBody([]string{"data.attributes.zip=01234", "data.attributes.flag=true"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"attributes":{"flag":"true","zip":"01234"}}}`
	if string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

func TestBuildBodyValueMayContainEquals(t *testing.T) {
	body, err := readBody([]string{"data.attributes.query=a=b"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"data":{"attributes":{"query":"a=b"}}}`; string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

func TestBuildBodyJSONValueEscapesDottedKeys(t *testing.T) {
	body, err := readBody([]string{`data.attributes.properties:={"my.key":1}`})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"data":{"attributes":{"properties":{"my.key":1}}}}`; string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

func TestBuildBodyErrors(t *testing.T) {
	cases := map[string][]string{
		"no equals":            {"data.type"},
		"empty path":           {"=x"},
		"bare colon path":      {":=1"},
		"empty segment":        {"data..type=x"},
		"invalid json value":   {"data.count:=notjson"},
		"duplicate field":      {"data.type=a", "data.type=b"},
		"value under value":    {"data.type=a", "data.type.sub=b"},
		"object over value":    {"data.type.sub=b", "data.type=a"},
		"pair after full body": {`{"x":1}`, "data.type=a"},
	}
	for name, pairs := range cases {
		if _, err := readBody(pairs); err == nil {
			t.Errorf("%s: readBody(%q) should error", name, pairs)
		}
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

func TestAPICommandRejectsBodyOnGET(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	if _, err := runCommand(t, "api", "/api/lists/", "-d", "x=y"); err == nil ||
		!strings.Contains(err.Error(), "not supported with GET") {
		t.Fatalf("err = %v", err)
	}
	if rec.method != "" {
		t.Error("no request must be sent for -d on GET")
	}
}

func TestAPICommandAcceptsFullURLForAPIHost(t *testing.T) {
	rec := apiServer(t, 200, `{"data":[]}`)
	// apiServer sets KLAVIYO_API_URL; a full URL for that host must work,
	// with its query merged in.
	full := os.Getenv("KLAVIYO_API_URL") + "/api/profiles?page%5Bsize%5D=2"
	if _, err := runCommand(t, "api", full); err != nil {
		t.Fatal(err)
	}
	if rec.path != "/api/profiles" || rec.query.Get("page[size]") != "2" {
		t.Errorf("request = %s query %v", rec.path, rec.query)
	}
	// A URL for any other host is an error, not a mangled request.
	if _, err := runCommand(t, "api", "https://evil.example/api/profiles"); err == nil ||
		!strings.Contains(err.Error(), `host "evil.example"`) {
		t.Fatalf("err = %v", err)
	}
	if rec.path != "/api/profiles" {
		t.Error("foreign-host URL must not produce a request")
	}
}
