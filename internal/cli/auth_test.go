package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klaviyo/klaviyo-cli/internal/config"
	"github.com/klaviyo/klaviyo-cli/internal/keyring"
)

var errNoKeychain = errors.New("no keychain available")

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
	// Two accounts plus the migrate hint (seedConfig keys are file-stored).
	if len(lines) != 3 {
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "* prod") {
		t.Errorf("default marker missing or unsorted: %q", lines[0])
	}
	if !strings.Contains(lines[0], "file") {
		t.Errorf("storage column missing: %q", lines[0])
	}
	if !strings.Contains(lines[2], "auth migrate") {
		t.Errorf("migrate hint missing: %q", lines[2])
	}
	if strings.Contains(out, "pk_") {
		t.Errorf("api key leaked:\n%s", out)
	}
}

func TestAuthListKeyringStoredHasNoHint(t *testing.T) {
	seedKeyringConfig(t)
	out, err := runCommand(t, "auth", "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "auth migrate") {
		t.Errorf("unexpected migrate hint:\n%s", out)
	}
	if !strings.Contains(out, "keyring") {
		t.Errorf("storage column missing:\n%s", out)
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
	keyring.MockInit()
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
	if !strings.Contains(out, "OS keychain") {
		t.Errorf("output missing storage location:\n%s", out)
	}
	cfg, _ := config.Load()
	acct := cfg.Accounts["prod"]
	if acct.APIKey != "" || acct.KeyStorage != config.KeyStorageKeyring || acct.ID != "A1" || acct.Organization != "Acme" {
		t.Errorf("stored account = %+v", acct)
	}
	if key, err := keyring.Get("prod"); err != nil || key != "pk_new" {
		t.Errorf("keyring key = %q, err = %v", key, err)
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

func TestAuthLoginInsecureStorageStoresInFile(t *testing.T) {
	keyring.MockInit()
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	accountsServer(t, 200, accountsBody)

	out, err := runCommand(t, "auth", "login", "--account", "prod", "--api-key", "pk_new", "--insecure-storage")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "config.toml") {
		t.Errorf("output missing file location:\n%s", out)
	}
	cfg, _ := config.Load()
	acct := cfg.Accounts["prod"]
	if acct.APIKey != "pk_new" || acct.KeyStorage != "" {
		t.Errorf("stored account = %+v", acct)
	}
	if _, err := keyring.Get("prod"); err != keyring.ErrNotFound {
		t.Errorf("key must not be in keyring, err = %v", err)
	}
}

func TestAuthLoginInsecureStorageRemovesOldKeyringKey(t *testing.T) {
	seedKeyringConfig(t)
	stubStdinTTY(t, false)
	accountsServer(t, 200, accountsBody)

	if _, err := runCommand(t, "auth", "login", "--account", "prod", "--api-key", "pk_new", "--insecure-storage"); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get("prod"); err != keyring.ErrNotFound {
		t.Errorf("stale keyring key must be removed, err = %v", err)
	}
	cfg, _ := config.Load()
	if acct := cfg.Accounts["prod"]; acct.APIKey != "pk_new" || acct.KeyStorage != "" {
		t.Errorf("stored account = %+v", acct)
	}
}

func TestAuthLoginFailsWhenKeychainUnavailable(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	accountsServer(t, 200, accountsBody)
	keyring.MockInitWithError(errNoKeychain)
	t.Cleanup(keyring.MockInit)

	_, err := runCommand(t, "auth", "login", "--account", "prod", "--api-key", "pk_new")
	if err == nil || !strings.Contains(err.Error(), "--insecure-storage") {
		t.Fatalf("err = %v", err)
	}
	cfg, _ := config.Load()
	if len(cfg.Accounts) != 0 {
		t.Errorf("account must not be stored when the keychain write fails: %+v", cfg.Accounts)
	}
}

func TestAuthLogoutRemovesKeyringKey(t *testing.T) {
	seedKeyringConfig(t)
	if _, err := runCommand(t, "auth", "logout", "prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get("prod"); err != keyring.ErrNotFound {
		t.Errorf("key must be removed from keyring, err = %v", err)
	}
	if key, err := keyring.Get("staging"); err != nil || key != "pk_2" {
		t.Errorf("other account's key must survive, key = %q, err = %v", key, err)
	}
}

func TestAuthMigrate(t *testing.T) {
	seedConfig(t)

	out, err := runCommand(t, "auth", "migrate")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `Moved key for "prod"`) || !strings.Contains(out, `Moved key for "staging"`) {
		t.Errorf("output = %q", out)
	}
	cfg, _ := config.Load()
	for name, wantKey := range map[string]string{"prod": "pk_1", "staging": "pk_2"} {
		acct := cfg.Accounts[name]
		if acct.APIKey != "" || acct.KeyStorage != config.KeyStorageKeyring {
			t.Errorf("%s account = %+v", name, acct)
		}
		if key, err := keyring.Get(name); err != nil || key != wantKey {
			t.Errorf("%s keyring key = %q, err = %v", name, key, err)
		}
	}

	// Second run has nothing to do.
	out, err = runCommand(t, "auth", "migrate")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No file-stored keys") {
		t.Errorf("output = %q", out)
	}
}

func TestAuthMigrateFailsWhenKeychainUnavailable(t *testing.T) {
	seedConfig(t)
	keyring.MockInitWithError(errNoKeychain)
	t.Cleanup(keyring.MockInit)

	_, err := runCommand(t, "auth", "migrate")
	if err == nil || !strings.Contains(err.Error(), "could not migrate") {
		t.Fatalf("err = %v", err)
	}
	// Keys must remain usable in the file.
	cfg, _ := config.Load()
	if cfg.Accounts["prod"].APIKey != "pk_1" {
		t.Errorf("prod account = %+v", cfg.Accounts["prod"])
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
