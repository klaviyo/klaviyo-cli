package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klaviyo/klaviyo-cli/internal/config"
)

func stubStdinTTY(t *testing.T, tty bool) {
	t.Helper()
	old := stdinIsTTY
	stdinIsTTY = func() bool { return tty }
	t.Cleanup(func() { stdinIsTTY = old })
}

// accountsServer serves GET /api/accounts/ and records the Authorization
// header it saw.
func accountsServer(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("KLAVIYO_API_URL", srv.URL)
	return srv, &gotAuth
}

const accountsBody = `{"data":[{"id":"A1","attributes":{"contact_information":{"organization_name":"Acme"}}}]}`

func TestAuthListShowsDefaultMarkerAndRedactsKeys(t *testing.T) {
	seedConfig(t)
	out, err := runCommand(t, "auth", "list")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "* prod") {
		t.Errorf("default marker missing or unsorted: %q", lines[0])
	}
	if strings.Contains(out, "pk_") {
		t.Errorf("api key leaked:\n%s", out)
	}
}

func TestAuthSwitch(t *testing.T) {
	seedConfig(t)
	if _, err := runCommand(t, "auth", "switch", "staging"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load()
	if cfg.DefaultAccount != "staging" {
		t.Errorf("default = %q", cfg.DefaultAccount)
	}
	if _, err := runCommand(t, "auth", "switch", "nope"); err == nil {
		t.Error("expected error for unknown account")
	}
}

func TestAuthLogout(t *testing.T) {
	seedConfig(t)
	out, err := runCommand(t, "auth", "logout", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "auth switch") {
		t.Errorf("expected no-default hint:\n%s", out)
	}
	cfg, _ := config.Load()
	if _, ok := cfg.Accounts["prod"]; ok {
		t.Error("prod still present after logout")
	}
	if cfg.DefaultAccount != "" {
		t.Errorf("default = %q, want cleared", cfg.DefaultAccount)
	}
	if _, err := runCommand(t, "auth", "logout", "nope"); err == nil {
		t.Error("expected error for unknown account")
	}
}

func TestAuthLoginRequiresFlagsNonInteractive(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	if _, err := runCommand(t, "auth", "login"); err == nil || !strings.Contains(err.Error(), "--account is required") {
		t.Errorf("err = %v", err)
	}
	if _, err := runCommand(t, "auth", "login", "--account", "x"); err == nil || !strings.Contains(err.Error(), "--api-key is required") {
		t.Errorf("err = %v", err)
	}
}

func TestAuthLoginVerifiesAndStores(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	_, gotAuth := accountsServer(t, 200, accountsBody)

	out, err := runCommand(t, "auth", "login", "--account", "prod", "--api-key", "pk_new")
	if err != nil {
		t.Fatal(err)
	}
	if *gotAuth != "Klaviyo-API-Key pk_new" {
		t.Errorf("Authorization = %q", *gotAuth)
	}
	if !strings.Contains(out, "Acme") || !strings.Contains(out, "A1") {
		t.Errorf("output missing org/id:\n%s", out)
	}
	cfg, _ := config.Load()
	acct := cfg.Accounts["prod"]
	if acct.APIKey != "pk_new" || acct.ID != "A1" || acct.Organization != "Acme" {
		t.Errorf("stored account = %+v", acct)
	}
	if cfg.DefaultAccount != "prod" {
		t.Error("first account must become default")
	}

	// A second login must not steal the default without --set-default.
	if _, err := runCommand(t, "auth", "login", "--account", "second", "--api-key", "pk_2"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load()
	if cfg.DefaultAccount != "prod" {
		t.Errorf("default = %q after second login", cfg.DefaultAccount)
	}
	if _, err := runCommand(t, "auth", "login", "--account", "third", "--api-key", "pk_3", "--set-default"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load()
	if cfg.DefaultAccount != "third" {
		t.Errorf("default = %q after --set-default", cfg.DefaultAccount)
	}
}

func TestAuthLoginRejectsBadKeyWithoutStoring(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	accountsServer(t, 401, `{"errors":[]}`)

	_, err := runCommand(t, "auth", "login", "--account", "prod", "--api-key", "pk_bad")
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err = %v", err)
	}
	cfg, _ := config.Load()
	if len(cfg.Accounts) != 0 {
		t.Errorf("rejected key must not be stored: %+v", cfg.Accounts)
	}
}

func TestAuthStatusReportsOrg(t *testing.T) {
	seedConfig(t)
	accountsServer(t, 200, accountsBody)

	out, err := runCommand(t, "auth", "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Authenticated to Acme (account A1)") {
		t.Errorf("output = %q", out)
	}
}

func TestVerifyKeyEmptyData(t *testing.T) {
	accountsServer(t, 200, `{"data":[]}`)
	if _, _, err := verifyKey(context.Background(), "pk"); err == nil || !strings.Contains(err.Error(), "no account returned") {
		t.Errorf("err = %v", err)
	}
}
