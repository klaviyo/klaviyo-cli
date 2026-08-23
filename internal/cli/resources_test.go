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
	pages []string
	calls []string
}

func (f *fakeDoer) Do(_ context.Context, _, path string, query url.Values, _ []byte) (*api.Response, error) {
	f.calls = append(f.calls, path+"?"+query.Encode())
	page := f.pages[0]
	f.pages = f.pages[1:]
	return &api.Response{StatusCode: 200, Body: []byte(page)}, nil
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
