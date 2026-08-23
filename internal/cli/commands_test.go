package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/api"
	"github.com/klaviyo/klaviyo-cli/internal/config"
)

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

func seedConfig(t *testing.T) {
	t.Helper()
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

func TestPrintResponseJQSkippedOnError(t *testing.T) {
	stubTable(t, false)
	opts.jq = ".data"
	t.Cleanup(func() { opts.jq = "" })

	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	resp := &api.Response{StatusCode: 404, Body: []byte(`{"errors":[{"detail":"not found"}]}`)}
	err := printResponse(cmd, resp)
	if err == nil || err.Error() != "HTTP 404" {
		t.Fatalf("err = %v, want HTTP 404", err)
	}
	if !strings.Contains(out.String(), "not found") {
		t.Errorf("error body must be printed raw, not jq-filtered:\n%s", out.String())
	}
}
