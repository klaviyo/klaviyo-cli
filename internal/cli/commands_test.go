package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/api"
	"github.com/klaviyo/klaviyo-cli/internal/config"
	"github.com/klaviyo/klaviyo-cli/internal/keyring"
)

// TestMain replaces the OS keychain with an in-memory mock so no test can
// touch the real keychain. Tests that need a fresh or failing keychain call
// keyring.MockInit / keyring.MockInitWithError themselves.
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func runCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// seedConfig writes a config with file-stored keys (the pre-keychain format)
// and resets the mock keychain.
func seedConfig(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	cfg := &config.Config{
		DefaultAccount: "prod",
		Accounts: map[string]config.Account{
			"prod":    {ID: "A1", Organization: "Acme", APIKey: "pk_1"},
			"staging": {ID: "B2", Organization: "Acme Staging", APIKey: "pk_2"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
}

// seedKeyringConfig writes a config whose keys live in the (mock) keychain.
func seedKeyringConfig(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	cfg := &config.Config{
		DefaultAccount: "prod",
		Accounts: map[string]config.Account{
			"prod":    {ID: "A1", Organization: "Acme", KeyStorage: config.KeyStorageKeyring},
			"staging": {ID: "B2", Organization: "Acme Staging", KeyStorage: config.KeyStorageKeyring},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	for name, key := range map[string]string{"prod": "pk_1", "staging": "pk_2"} {
		if err := keyring.Set(name, key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfigListRedactsKeys(t *testing.T) {
	seedConfig(t)
	out, err := runCommand(t, "config", "--list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "pk_1") || strings.Contains(out, "pk_2") {
		t.Errorf("api keys leaked in --list output:\n%s", out)
	}
	if !strings.Contains(out, "<redacted>") || !strings.Contains(out, "default_account = 'prod'") {
		t.Errorf("unexpected --list output:\n%s", out)
	}
}

func TestConfigSetDefaultAccount(t *testing.T) {
	seedConfig(t)
	if _, err := runCommand(t, "config", "--set", "default_account", "staging"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultAccount != "staging" {
		t.Errorf("default = %q, want staging", cfg.DefaultAccount)
	}
	if _, err := runCommand(t, "config", "--set", "default_account", "nope"); err == nil {
		t.Error("expected error for unknown account")
	}
	if _, err := runCommand(t, "config", "--set", "bogus_key", "x"); err == nil {
		t.Error("expected error for unknown key")
	}
	if _, err := runCommand(t, "config", "--unset", "default_account"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load()
	if cfg.DefaultAccount != "" {
		t.Errorf("default = %q after unset, want empty", cfg.DefaultAccount)
	}
}

func TestOpenNoBrowserPrintsURL(t *testing.T) {
	launched := false
	old := launchBrowser
	launchBrowser = func(string) error { launched = true; return nil }
	defer func() { launchBrowser = old }()

	out, err := runCommand(t, "open", "docs", "--no-browser")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "https://developers.klaviyo.com" {
		t.Errorf("output = %q", out)
	}
	if launched {
		t.Error("browser must not launch with --no-browser")
	}
	if _, err := runCommand(t, "open", "bogus"); err == nil {
		t.Error("expected error for unknown shortcut")
	}
}

func TestRenderTablePrefersKnownColumns(t *testing.T) {
	items := []resourceItem{
		{ID: "1", Attributes: map[string]any{"name": "Newsletter", "opt_in_process": "double", "created": "2026-01-01"}},
		{ID: "2", Attributes: map[string]any{"name": "VIPs", "opt_in_process": "single", "created": "2026-02-01"}},
	}
	out := &bytes.Buffer{}
	renderTable(out, items)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3:\n%s", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], "ID") || !strings.Contains(lines[0], "name") {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.Contains(lines[1], "Newsletter") {
		t.Errorf("row = %q", lines[1])
	}
}

func TestTableColumnsScanAllRows(t *testing.T) {
	// phone_number is null in row 1 but set in row 2: the column must
	// appear regardless of row order.
	items := []resourceItem{
		{ID: "1", Attributes: map[string]any{"phone_number": nil}},
		{ID: "2", Attributes: map[string]any{"phone_number": "+15550001111"}},
	}
	out := &bytes.Buffer{}
	renderTable(out, items)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if !strings.Contains(lines[0], "phone_number") {
		t.Errorf("header = %q, want phone_number column", lines[0])
	}
	if !strings.Contains(lines[2], "+15550001111") {
		t.Errorf("row 2 = %q", lines[2])
	}
}

func TestTableObjectColumnsFillLastAsJSON(t *testing.T) {
	// An object-valued attribute earns a column only when scalar candidates
	// leave room, rendered as compact JSON.
	only := []resourceItem{
		{ID: "1", Attributes: map[string]any{"send_strategy": map[string]any{"method": "static"}}},
	}
	out := &bytes.Buffer{}
	renderTable(out, only)
	s := out.String()
	if !strings.Contains(s, "send_strategy") || !strings.Contains(s, `{"method":"static"}`) {
		t.Errorf("object-only attributes must still render:\n%s", s)
	}

	// With four scalars available, the object is crowded out.
	crowded := []resourceItem{
		{ID: "1", Attributes: map[string]any{
			"a": "1", "b": "2", "c": "3", "d": "4",
			"send_strategy": map[string]any{"method": "static"},
		}},
	}
	out.Reset()
	renderTable(out, crowded)
	if strings.Contains(out.String(), "send_strategy") {
		t.Errorf("object must not displace scalar columns:\n%s", out.String())
	}
}

func stubTable(t *testing.T, render bool) {
	t.Helper()
	old := shouldRenderTable
	shouldRenderTable = func(io.Writer) bool { return render }
	t.Cleanup(func() { shouldRenderTable = old })
}

func TestPrintResponseNonTerminalStaysJSON(t *testing.T) {
	stubTable(t, false)
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	resp := &api.Response{StatusCode: 200, Body: []byte(`{"data":[{"id":"1","attributes":{"name":"x"}}]}`)}
	if err := printResponse(cmd, resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"data"`) {
		t.Errorf("expected JSON output, got:\n%s", out.String())
	}
}

func TestPrintResponseRendersTableOnTerminal(t *testing.T) {
	stubTable(t, true)
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	resp := &api.Response{StatusCode: 200, Body: []byte(`{"data":[{"id":"1","attributes":{"name":"Newsletter"}}]}`)}
	if err := printResponse(cmd, resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ID") || !strings.Contains(out.String(), "Newsletter") {
		t.Errorf("expected table output, got:\n%s", out.String())
	}
	if strings.Contains(out.String(), `"data"`) {
		t.Errorf("table output should not be JSON:\n%s", out.String())
	}
}

func TestPrintResponseSingleResourceStaysJSON(t *testing.T) {
	stubTable(t, true)
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	resp := &api.Response{StatusCode: 200, Body: []byte(`{"data":{"id":"1","attributes":{"name":"x"}}}`)}
	if err := printResponse(cmd, resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"data"`) {
		t.Errorf("single resource should print as JSON:\n%s", out.String())
	}
}

func TestTableFooterShowsNextCursor(t *testing.T) {
	stubTable(t, true)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	body := `{"data":[{"id":"1","attributes":{"name":"x"}}],` +
		`"links":{"next":"https://a.klaviyo.com/api/profiles?page%5Bsize%5D=2&page%5Bcursor%5D=bmV4dDo6abc"}}`
	if err := printResponse(cmd, &api.Response{StatusCode: 200, Body: []byte(body)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "--page-cursor bmV4dDo6abc") {
		t.Errorf("stderr missing continuation hint:\n%s", errOut.String())
	}
	if strings.Contains(out.String(), "bmV4dDo6abc") {
		t.Errorf("cursor must not pollute stdout table:\n%s", out.String())
	}

	// Last page: no links.next, no hint.
	errOut.Reset()
	last := `{"data":[{"id":"1","attributes":{"name":"x"}}],"links":{"next":""}}`
	if err := printResponse(cmd, &api.Response{StatusCode: 200, Body: []byte(last)}); err != nil {
		t.Fatal(err)
	}
	if errOut.String() != "" {
		t.Errorf("no hint expected on the last page:\n%s", errOut.String())
	}
}

func TestPrintResponseJQEmptySuccessBody(t *testing.T) {
	stubTable(t, false)
	opts.jq = ".data.id"
	t.Cleanup(func() { opts.jq = "" })

	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	// tags update returns 204 No Content: --jq must print nothing and
	// succeed, not fail a request that worked.
	if err := printResponse(cmd, &api.Response{StatusCode: 204, Body: nil}); err != nil {
		t.Fatalf("err = %v, want nil on empty success body", err)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestPrintResponseJQErrorBodyGoesToStderr(t *testing.T) {
	stubTable(t, false)
	opts.jq = ".data.id"
	t.Cleanup(func() { opts.jq = "" })

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	resp := &api.Response{StatusCode: 404, Body: []byte(`{"errors":[{"detail":"not found"}]}`)}
	err := printResponse(cmd, resp)
	if err == nil || err.Error() != "HTTP 404" {
		t.Fatalf("err = %v, want HTTP 404", err)
	}
	// With --jq set, stdout is reserved for the filtered result: a script's
	// ID=$(... --jq .data.id) must capture nothing on error, not the blob.
	if out.String() != "" {
		t.Errorf("stdout must stay empty on error with --jq:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "not found") {
		t.Errorf("error body must be printed raw to stderr, not jq-filtered:\n%s", errOut.String())
	}
}
