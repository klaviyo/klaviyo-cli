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
	// One line per account (seedConfig keys are file-stored).
	if len(lines) != 2 {
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "* prod") {
		t.Errorf("default marker missing or unsorted: %q", lines[0])
	}
	if !strings.Contains(lines[0], "file") {
		t.Errorf("storage column missing: %q", lines[0])
	}
	if strings.Contains(out, "pk_") {
		t.Errorf("api key leaked:\n%s", out)
	}
}

func TestAuthListShowsKeyringStorage(t *testing.T) {
	seedKeyringConfig(t)
	out, err := runCommand(t, "auth", "list")
	if err != nil {
		t.Fatal(err)
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

func TestAuthLoginRequiresKeyNonInteractive(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	// The account name is no longer required (it defaults to the key's
	// organization), but some form of key is.
	for _, args := range [][]string{{"auth", "login"}, {"auth", "login", "--account", "x"}} {
		if _, err := runCommand(t, args...); err == nil || !strings.Contains(err.Error(), "--api-key or --api-key-stdin is required") {
			t.Errorf("%v: err = %v", args, err)
		}
	}
}

func TestAuthLoginKeyFromStdinAndOrgDefaultName(t *testing.T) {
	keyring.MockInit()
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	_, gotAuth := accountsServer(t, 200, `{"data":[{"id":"A9","attributes":{"contact_information":{"organization_name":"Acme Inc."}}}]}`)

	root := newRootCmd()
	root.SetArgs([]string{"auth", "login", "--api-key-stdin"})
	root.SetIn(strings.NewReader("pk_piped\n"))
	out := &strings.Builder{}
	root.SetOut(out)
	root.SetErr(out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if *gotAuth != "Klaviyo-API-Key pk_piped" {
		t.Errorf("Authorization = %q", *gotAuth)
	}
	cfg, _ := config.Load()
	if _, ok := cfg.Accounts["acme-inc"]; !ok {
		t.Errorf("account name must default to the org slug, got %v", cfg.Accounts)
	}
	if key, err := keyring.Get("acme-inc"); err != nil || key != "pk_piped" {
		t.Errorf("keyring key = %q, err = %v", key, err)
	}
}

func TestAuthLoginKeyFlagsMutuallyExclusive(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	stubStdinTTY(t, false)
	if _, err := runCommand(t, "auth", "login", "--api-key", "pk_x", "--api-key-stdin"); err == nil ||
		!strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("err = %v", err)
	}
}

func TestAccountSlug(t *testing.T) {
	cases := map[string]string{
		"Acme Inc.":      "acme-inc",
		"Klaviyo Demo":   "klaviyo-demo",
		"  weird -- Org": "weird-org",
		"日本語":            "",
	}
	for org, want := range cases {
		if got := accountSlug(org); got != want {
			t.Errorf("accountSlug(%q) = %q, want %q", org, got, want)
		}
	}
}

func TestAuthSwitchByID(t *testing.T) {
	seedConfig(t)
	cfg, _ := config.Load()
	staging := cfg.Accounts["staging"]
	if staging.ID == "" {
		t.Skip("seedConfig has no staging ID")
	}
	if _, err := runCommand(t, "auth", "switch", staging.ID); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.Load()
	if cfg.DefaultAccount != "staging" {
		t.Errorf("default = %q, want staging via ID", cfg.DefaultAccount)
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

func TestAuthLoginAccountShorthand(t *testing.T) {
	// The local --account shadows the global -a/--account; it must carry the
	// same shorthand so `-a` works on login like on every other command.
	f := newAuthLoginCmd().Flags().ShorthandLookup("a")
	if f == nil || f.Name != "account" {
		t.Fatalf("auth login -a = %+v, want --account shorthand", f)
	}
}

func TestAuthListAlignsLongNames(t *testing.T) {
	keyring.MockInit()
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	cfg := &config.Config{
		DefaultAccount: "prod",
		Accounts: map[string]config.Account{
			"prod":                   {ID: "A1", Organization: "Acme", APIKey: "pk_1"},
			"petconnect-innovations": {ID: "B2", Organization: "PetConnect Innovations", APIKey: "pk_2"},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	out, err := runCommand(t, "auth", "list")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	// The ID column must start at the same offset on every row, long name
	// or not (the old fixed %-20s column broke past 20 chars).
	if i, j := strings.Index(lines[0], "B2"), strings.Index(lines[1], "A1"); i != j || i < 0 {
		t.Errorf("ID columns misaligned (%d vs %d):\n%s", i, j, out)
	}
}
