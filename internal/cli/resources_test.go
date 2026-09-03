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
		if op.Beta {
			t.Errorf("stable op %s marked Beta", key)
		}
	}
	if len(resourceOps) == 0 {
		t.Fatal("no generated operations; run go generate ./...")
	}
	betaSeen := map[string]bool{}
	for _, op := range betaResourceOps {
		key := op.Group + "/" + op.Name
		if betaSeen[key] {
			t.Errorf("duplicate beta command %s", key)
		}
		betaSeen[key] = true
		if !op.Beta {
			t.Errorf("beta op %s not marked Beta", key)
		}
	}
	if len(betaResourceOps) == 0 {
		t.Fatal("no generated beta operations; regenerate with -beta-spec")
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
			{"page[size]", "page-size", "page size", false},
			{"filter", "filter", "filter", false},
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

func TestRequiredQueryFlagEnforced(t *testing.T) {
	rec := apiServer(t, 200, `{"data":[]}`)
	op := &opSpec{
		Group: "widgets", Name: "list", Method: "GET", Path: "/api/widgets",
		Query: []queryParamSpec{{"filter", "filter", "filter", true}},
	}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `required flag(s) "filter" not set`) {
		t.Fatalf("err = %v, want required-flag error", err)
	}
	if len(rec.query) != 0 && rec.method != "" {
		t.Error("no request must be sent when a required flag is missing")
	}

	cmd = newResourceOpCmd(op)
	cmd.SetArgs([]string{"--filter", "equals(x,'y')"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := rec.query.Get("filter"); got != "equals(x,'y')" {
		t.Errorf("filter = %q", got)
	}
}

func TestCursorFlagAcceptsNextLink(t *testing.T) {
	next := "https://a.klaviyo.com/api/profiles?page%5Bsize%5D=2&page%5Bcursor%5D=bmV4dDo6abc"
	rec := apiServer(t, 200, `{"data":[]}`)
	op := &opSpec{
		Group: "widgets", Name: "list", Method: "GET", Path: "/api/widgets",
		Query: []queryParamSpec{{"page[cursor]", "page-cursor", "cursor", false}},
	}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs([]string{"--page-cursor", next})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := rec.query.Get("page[cursor]"); got != "bmV4dDo6abc" {
		t.Errorf("page[cursor] = %q, want extracted cursor", got)
	}
}

func TestNormalizeCursor(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"page[cursor]", "bmV4dDo6abc", "bmV4dDo6abc"}, // bare cursor untouched
		{"page[cursor]", "https://a.klaviyo.com/api/x?page%5Bcursor%5D=cur1", "cur1"},
		// Reporting endpoints name the param page_cursor; their links carry it.
		{"page_cursor", "https://a.klaviyo.com/api/x?page_cursor=cur2", "cur2"},
		// A page_cursor endpoint given a page[cursor]-style link still works.
		{"page_cursor", "https://a.klaviyo.com/api/x?page%5Bcursor%5D=cur3", "cur3"},
		// URL without any cursor param passes through (API will reject it).
		{"page[cursor]", "https://a.klaviyo.com/api/x?page%5Bsize%5D=2", "https://a.klaviyo.com/api/x?page%5Bsize%5D=2"},
	}
	for _, c := range cases {
		if got := normalizeCursor(c.name, c.raw); got != c.want {
			t.Errorf("normalizeCursor(%q, %q) = %q, want %q", c.name, c.raw, got, c.want)
		}
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

func TestResourceCmdRejectsEmptyPathArg(t *testing.T) {
	rec := apiServer(t, 200, `{"data":[]}`)
	op := &opSpec{Group: "widgets", Name: "get", Method: "GET", Path: "/api/widgets/{id}", PathParams: []string{"id"}}
	for _, arg := range []string{"", "  "} {
		cmd := newResourceOpCmd(op)
		cmd.SetArgs([]string{arg})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "<id> must not be empty") {
			t.Errorf("arg %q: err = %v, want empty-id error", arg, err)
		}
	}
	// An empty id must never reach the API: /api/widgets/{id} with "" is
	// the collection route, returning the full list for a targeted get.
	if rec.method != "" {
		t.Errorf("request sent: %s %s", rec.method, rec.path)
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

func TestJSONBodyFlag(t *testing.T) {
	op := &opSpec{
		Group: "segments", Name: "create", Method: "POST", Path: "/api/segments",
		HasBody: true, BodyType: "segment",
		Body: []bodyFieldSpec{
			{"definition", "definition", "json", "segment definition (JSON value) (required)"},
			{"name", "name", "string", "segment name (required)"},
		},
	}
	rec := apiServer(t, 200, `{}`)
	cmd := newResourceOpCmd(op)
	cmd.SetArgs([]string{"--name", "Winback", "--definition", `{"condition_groups":[{"conditions":[]}]}`})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"attributes":{"definition":{"condition_groups":[{"conditions":[]}]},"name":"Winback"},"type":"segment"}}`
	if rec.body != want {
		t.Errorf("body = %s, want %s", rec.body, want)
	}

	cmd = newResourceOpCmd(op)
	cmd.SetArgs([]string{"--definition", `{not json`})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("err = %v, want invalid JSON error", err)
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

func TestOpLongIncludesDescriptionAndDocLinks(t *testing.T) {
	op := &opSpec{
		Group: "lists", Name: "update", OpID: "update_list", Summary: "Update List",
		Description: "Update the name of a list.\n\nRate limits:\nBurst: 10/s",
		Method:      "PATCH", Path: "/api/lists/{id}", PathParams: []string{"id"},
	}
	long := newResourceOpCmd(op).Long
	for _, want := range []string{
		"Update List",
		"Update the name of a list.",
		"Calls PATCH /api/lists/{id}.",
		"https://developers.klaviyo.com/en/v" + api.DefaultRevision + "/reference/update_list",
		"https://github.com/klaviyo/openapi/blob/" + api.DefaultRevision + "/openapi/stable/apis/update_list.json",
	} {
		if !strings.Contains(long, want) {
			t.Errorf("Long missing %q:\n%s", want, long)
		}
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

func TestOpLongBetaLinks(t *testing.T) {
	op := &opSpec{
		Group: "campaigns", Name: "get-campaigns", OpID: "get_campaigns_beta",
		Summary: "Get Campaigns", Method: "GET", Path: "/api/campaigns", Beta: true,
	}
	long := newResourceOpCmd(op).Long
	for _, want := range []string{
		// Beta docs pages exist only on the docs site's current version.
		"https://developers.klaviyo.com/en/reference/get_campaigns_beta",
		"https://github.com/klaviyo/openapi/blob/" + api.DefaultBetaRevision + "/openapi/beta/apis/get_campaigns_beta.json",
	} {
		if !strings.Contains(long, want) {
			t.Errorf("Long missing %q:\n%s", want, long)
		}
	}
	if strings.Contains(long, "/en/v") {
		t.Errorf("beta docs link must not be versioned:\n%s", long)
	}
}

func TestBetaOpSendsBetaRevision(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	op := &opSpec{Group: "campaigns", Name: "get-campaigns", Method: "GET", Path: "/api/campaigns", Beta: true}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs(nil)
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.revision != api.DefaultBetaRevision {
		t.Errorf("revision = %q, want %q", rec.revision, api.DefaultBetaRevision)
	}
}

func TestBetaOpRespectsRevisionFlag(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	opts.revision = "2027-01-15.pre"
	t.Cleanup(func() { opts.revision = "" })
	op := &opSpec{Group: "campaigns", Name: "get-campaigns", Method: "GET", Path: "/api/campaigns", Beta: true}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs(nil)
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.revision != "2027-01-15.pre" {
		t.Errorf("revision = %q, want explicit override to win", rec.revision)
	}
}

func TestStableOpSendsDefaultRevision(t *testing.T) {
	rec := apiServer(t, 200, `{}`)
	op := &opSpec{Group: "widgets", Name: "list", Method: "GET", Path: "/api/widgets"}
	cmd := newResourceOpCmd(op)
	cmd.SetArgs(nil)
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.revision != api.DefaultRevision {
		t.Errorf("revision = %q, want %q", rec.revision, api.DefaultRevision)
	}
}

func TestBetaCommandsMountedUnderBetaParent(t *testing.T) {
	root := newRootCmd()
	betaCmd, _, err := root.Find([]string{"beta"})
	if err != nil || betaCmd.Use != "beta" {
		t.Fatalf("beta parent not found: %v", err)
	}
	// A beta group must exist under beta with GA-style CRUD names. cobra's
	// Find is lenient (unknown trailing args return the parent without
	// error), so assert on the resolved command's name, not just on err.
	cmd, _, err := root.Find([]string{"beta", "campaigns", "list"})
	if err != nil || cmd.Name() != "list" {
		t.Errorf("beta campaigns list resolved to %q (err %v)", cmd.Name(), err)
	}
	// A beta-only verb must not leak into the stable campaigns group.
	if cmd, _, _ := root.Find([]string{"campaigns", "clone-campaign"}); cmd != nil && cmd.Name() == "clone-campaign" {
		t.Error("beta command leaked into the stable campaigns group")
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
	// A complete merge keeps the plain {"data": [...]} shape — links.next
	// appears only when the page cap truncated the merge.
	if strings.Contains(out.String(), `"links"`) {
		t.Errorf("complete merge must not carry links: %s", out.String())
	}
}

func TestRunPaginatedMergesIncludedDeduped(t *testing.T) {
	doer := &fakeDoer{pages: []string{
		`{"data":[{"id":"1"}],"included":[{"type":"tag","id":"T1"},{"type":"tag","id":"T2"}],"links":{"next":"https://a.klaviyo.com/api/x?page%5Bcursor%5D=p2"}}`,
		`{"data":[{"id":"2"}],"included":[{"type":"tag","id":"T1"},{"type":"tag","id":"T3"}],"links":{"next":""}}`,
	}}
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	if err := runPaginated(cmd, doer, "/api/x", url.Values{}); err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Included []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"included"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	// T1 appears on both pages and must merge once.
	if len(parsed.Included) != 3 {
		t.Errorf("included = %+v, want 3 deduped items", parsed.Included)
	}

	// No included on any page: the key must stay absent.
	doer = &fakeDoer{pages: []string{`{"data":[{"id":"1"}],"links":{"next":""}}`}}
	out.Reset()
	if err := runPaginated(cmd, doer, "/api/x", url.Values{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "included") {
		t.Errorf("included key must be absent when no page had one:\n%s", out.String())
	}
}

func TestRunPaginatedEmptyResultKeepsArray(t *testing.T) {
	doer := &fakeDoer{pages: []string{`{"data":[],"links":{"next":""}}`}}
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetContext(context.Background())

	if err := runPaginated(cmd, doer, "/api/x", url.Values{}); err != nil {
		t.Fatal(err)
	}
	// data must stay an array on empty merges — {"data": null} breaks
	// consumers like --jq '.data[]'.
	if !strings.Contains(out.String(), `"data": []`) && !strings.Contains(out.String(), `"data":[]`) {
		t.Errorf("empty merge must marshal data as []:\n%s", out.String())
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
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
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
	// The capped merge must carry the continuation, JSON:API style, so
	// scripts can detect truncation and resume from links.next.
	var parsed struct {
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.Links.Next, "page%5Bcursor%5D=again") {
		t.Errorf("capped output missing links.next: %s", out.String())
	}
}
