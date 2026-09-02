package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/api"
)

func TestResolvePath(t *testing.T) {
	got := resolvePath("/api/profile-bulk-import-jobs/{job_id}/lists", []string{"job_id"}, []string{"abc 123"})
	want := "/api/profile-bulk-import-jobs/abc%20123/lists"
	if got != want {
		t.Errorf("resolvePath = %q, want %q", got, want)
	}
}

func TestGeneratedCommandsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, op := range resourceOps {
		key := op.Group + "/" + op.Name
		if seen[key] {
			t.Errorf("duplicate command %s", key)
		}
		seen[key] = true
	}
	if len(resourceOps) == 0 {
		t.Fatal("no generated operations; run go generate ./...")
	}
}

type fakeDoer struct {
	pages    []string
	statuses []int
	repeat   string // when set, every call returns this page
	calls    []string
}

func (f *fakeDoer) Do(_ context.Context, _, path string, query url.Values, _ []byte) (*api.Response, error) {
	f.calls = append(f.calls, path+"?"+query.Encode())
	if f.repeat != "" {
		return &api.Response{StatusCode: 200, Body: []byte(f.repeat)}, nil
	}
	status := 200
	if len(f.statuses) > 0 {
		status = f.statuses[0]
		f.statuses = f.statuses[1:]
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return &api.Response{StatusCode: status, Body: []byte(page)}, nil
}

func TestResourceCmdMapsFlagsToQueryParams(t *testing.T) {
	rec := apiServer(t, 200, `{"data":[]}`)
	op := &opSpec{
		Group: "widgets", Name: "list", Method: "GET", Path: "/api/widgets",
		Query: []queryParamSpec{
			{"page[size]", "page-size", "page size"},
			{"filter", "filter", "filter"},
		},
		Paginated: true,
	}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs([]string{"--page-size", "5"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := rec.query.Get("page[size]"); got != "5" {
		t.Errorf("page[size] = %q", got)
	}
	if _, present := rec.query["filter"]; present {
		t.Error("unset flag must not become a query param")
	}
}

func TestResourceCmdRequiresPathArgs(t *testing.T) {
	apiServer(t, 200, `{}`)
	op := &opSpec{Group: "widgets", Name: "get", Method: "GET", Path: "/api/widgets/{id}", PathParams: []string{"id"}}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("missing path arg must error")
	}
}

func TestResourceCmdSendsBody(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	op := &opSpec{Group: "widgets", Name: "create", Method: "POST", Path: "/api/widgets", HasBody: true}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs([]string{"-d", `{"data":{"type":"widget"}}`})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.method != "POST" || rec.body != `{"data":{"type":"widget"}}` {
		t.Errorf("request = %s body %q", rec.method, rec.body)
	}
	if !strings.Contains(rec.contentType, "vnd.api+json") {
		t.Errorf("content-type = %q", rec.contentType)
	}
}

func TestResourceCmdBuildsBodyFromFieldPairs(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	op := &opSpec{Group: "widgets", Name: "create", Method: "POST", Path: "/api/widgets", HasBody: true}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs([]string{"-d", "data.type=widget", "-d", "data.attributes.size:=3"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"data":{"attributes":{"size":3},"type":"widget"}}` {
		t.Errorf("body = %q", rec.body)
	}
}

// widgetCreateOp is a HasBody op with generated body-attribute flags.
func widgetCreateOp() *opSpec {
	return &opSpec{
		Group: "widgets", Name: "create", Method: "POST", Path: "/api/widgets",
		HasBody: true, BodyType: "widget",
		Body: []bodyFieldSpec{
			{"name", "name", "string", "widget name"},
			{"size", "size", "integer", "widget size"},
			{"active", "active", "boolean", "whether active"},
			{"tags", "tags", "array-string", "tags"},
			{"location.city", "location.city", "string", "city"},
		},
	}
}

func runWidgetCreate(t *testing.T, args ...string) (*recordedRequest, error) {
	t.Helper()
	rec := apiServer(t, 200, `{}`)
	cmd := newResourceOpCmd(widgetCreateOp())
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return rec, cmd.Execute()
}

func TestBodyFlagsBuildTypedBodyAndInjectType(t *testing.T) {
	rec, err := runWidgetCreate(t,
		"--name", "Widget", "--size", "3", "--active", "true",
		"--tags", "a", "--tags", "b", "--location.city", "Boston")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"attributes":{"active":true,"location":{"city":"Boston"},"name":"Widget","size":3,"tags":["a","b"]},"type":"widget"}}`
	if rec.body != want {
		t.Errorf("body = %s, want %s", rec.body, want)
	}
}

func TestBodyFlagsMergeWithDataPairs(t *testing.T) {
	rec, err := runWidgetCreate(t, "--name", "Widget", "-d", `data.relationships.tags.data:=[{"type":"tag","id":"T1"}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"attributes":{"name":"Widget"},"relationships":{"tags":{"data":[{"id":"T1","type":"tag"}]}},"type":"widget"}}`
	if rec.body != want {
		t.Errorf("body = %s, want %s", rec.body, want)
	}
}

func TestBodyFlagsRespectExplicitType(t *testing.T) {
	rec, err := runWidgetCreate(t, "--name", "W", "-d", "data.type=custom-widget")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.body, `"type":"custom-widget"`) {
		t.Errorf("explicit data.type must win: %s", rec.body)
	}
}

func TestBodyFlagConflictsWithDataPair(t *testing.T) {
	if _, err := runWidgetCreate(t, "--name", "A", "-d", "data.attributes.name=B"); err == nil {
		t.Error("conflicting flag and -d pair must error")
	}
}

func TestBodyFlagsRejectWholeBodyData(t *testing.T) {
	if _, err := runWidgetCreate(t, "--name", "A", "-d", `{"data":{}}`); err == nil ||
		!strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("err = %v", err)
	}
}

func TestBodyFlagsValidateTypes(t *testing.T) {
	if _, err := runWidgetCreate(t, "--size", "big"); err == nil || !strings.Contains(err.Error(), "not a valid integer") {
		t.Errorf("size err = %v", err)
	}
	if _, err := runWidgetCreate(t, "--active", "yes"); err == nil || !strings.Contains(err.Error(), "not a valid boolean") {
		t.Errorf("active err = %v", err)
	}
}

func TestBodyFlagsAloneStillInjectType(t *testing.T) {
	// -d dot pairs without flags also get data.type injected.
	rec, err := runWidgetCreate(t, "-d", "data.attributes.name=Widget")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.body, `"type":"widget"`) {
		t.Errorf("type not injected: %s", rec.body)
	}
}

func TestUpdateBodyInjectsIDFromPathArg(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	op := &opSpec{
		Group: "widgets", Name: "update", Method: "PATCH", Path: "/api/widgets/{id}",
		PathParams: []string{"id"},
		HasBody:    true, BodyType: "widget",
		Body: []bodyFieldSpec{{"name", "name", "string", "widget name"}},
	}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs([]string{"W123", "--name", "Widget"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"attributes":{"name":"Widget"},"id":"W123","type":"widget"}}`
	if rec.body != want {
		t.Errorf("body = %s, want %s", rec.body, want)
	}
}

func TestRunPaginatedMergesPages(t *testing.T) {
	doer := &fakeDoer{pages: []string{
		`{"data":[{"id":"1"}],"links":{"next":"https://a.klaviyo.com/api/profiles?page%5Bcursor%5D=abc"}}`,
		`{"data":[{"id":"2"},{"id":"3"}],"links":{"next":""}}`,
	}}
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	if err := runPaginated(cmd, doer, "/api/profiles", url.Values{}); err != nil {
		t.Fatal(err)
	}
	if len(doer.calls) != 2 {
		t.Fatalf("calls = %v, want 2", doer.calls)
	}
	if !strings.Contains(doer.calls[1], "cursor") {
		t.Errorf("second call missing cursor: %q", doer.calls[1])
	}
	var parsed struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Data) != 3 {
		t.Errorf("merged data = %d items, want 3", len(parsed.Data))
	}
}

func TestRunPaginatedStopsOnErrorPage(t *testing.T) {
	doer := &fakeDoer{pages: []string{
		`{"data":[{"id":"1"}],"links":{"next":"https://a.klaviyo.com/api/x?page%5Bcursor%5D=abc"}}`,
		`{"errors":[{"detail":"rate limited"}]}`,
	}}
	doer.statuses = []int{200, 429}
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	err := runPaginated(cmd, doer, "/api/x", url.Values{})
	if err == nil || err.Error() != "HTTP 429" {
		t.Fatalf("err = %v", err)
	}
	if len(doer.calls) != 2 {
		t.Errorf("calls = %d, want 2 (must stop on error page)", len(doer.calls))
	}
	if !strings.Contains(out.String(), "rate limited") {
		t.Errorf("error body not surfaced:\n%s", out.String())
	}
}

func TestRunPaginatedIgnoresNextLinkHost(t *testing.T) {
	doer := &fakeDoer{pages: []string{
		`{"data":[],"links":{"next":"https://evil.example/api/x?page%5Bcursor%5D=z"}}`,
		`{"data":[],"links":{"next":""}}`,
	}}
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetContext(context.Background())

	if err := runPaginated(cmd, doer, "/api/x", url.Values{}); err != nil {
		t.Fatal(err)
	}
	// The second request must go to the configured client with only the
	// path — the hostile host in links.next is discarded.
	if !strings.HasPrefix(doer.calls[1], "/api/x?") {
		t.Errorf("second call = %q", doer.calls[1])
	}
}

func TestRunPaginatedWarnsAtPageCap(t *testing.T) {
	doer := &fakeDoer{repeat: `{"data":[{"id":"1"}],"links":{"next":"https://a.klaviyo.com/api/x?page%5Bcursor%5D=again"}}`}
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	errOut := &bytes.Buffer{}
	cmd.SetErr(errOut)
	cmd.SetContext(context.Background())

	if err := runPaginated(cmd, doer, "/api/x", url.Values{}); err != nil {
		t.Fatal(err)
	}
	if len(doer.calls) != maxPages {
		t.Errorf("calls = %d, want %d", len(doer.calls), maxPages)
	}
	if !strings.Contains(errOut.String(), "stopped after") {
		t.Errorf("expected truncation warning, got: %q", errOut.String())
	}
}
