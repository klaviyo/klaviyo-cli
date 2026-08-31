package cli

import (
	"strings"
	"testing"

	"github.com/klaviyo/klaviyo-cli/internal/config"
	"github.com/klaviyo/klaviyo-cli/internal/keyring"
)

func resetOpts(t *testing.T) {
	t.Helper()
	// Isolate from the developer's real environment; resolveKey treats an
	// empty value as unset.
	t.Setenv("KLAVIYO_API_KEY", "")
	t.Setenv("KLAVIYO_ACCOUNT", "")
	t.Cleanup(func() { opts = globalOpts{} })
}

func TestResolveKeyPrecedence(t *testing.T) {
	seedConfig(t)
	resetOpts(t)

	// Stored default account.
	opts = globalOpts{}
	if key, _ := resolveKey(); key != "pk_1" {
		t.Errorf("default account key = %q", key)
	}
	// --account flag beats default.
	opts = globalOpts{account: "staging"}
	if key, _ := resolveKey(); key != "pk_2" {
		t.Errorf("--account key = %q", key)
	}
	// KLAVIYO_ACCOUNT env beats default; --account beats env.
	t.Setenv("KLAVIYO_ACCOUNT", "staging")
	opts = globalOpts{}
	if key, _ := resolveKey(); key != "pk_2" {
		t.Errorf("env account key = %q", key)
	}
	opts = globalOpts{account: "prod"}
	if key, _ := resolveKey(); key != "pk_1" {
		t.Errorf("--account over env key = %q", key)
	}
	// KLAVIYO_API_KEY env beats stored accounts.
	t.Setenv("KLAVIYO_API_KEY", "pk_env")
	if key, _ := resolveKey(); key != "pk_env" {
		t.Errorf("env key = %q", key)
	}
	// --api-key flag beats everything.
	opts = globalOpts{apiKey: "pk_flag"}
	if key, _ := resolveKey(); key != "pk_flag" {
		t.Errorf("flag key = %q", key)
	}
}

func TestResolveKeyErrors(t *testing.T) {
	resetOpts(t)
	opts = globalOpts{}

	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	if _, err := resolveKey(); err == nil || !strings.Contains(err.Error(), "klaviyo auth login") {
		t.Errorf("no-config err = %v", err)
	}

	seedConfig(t)
	opts = globalOpts{account: "nope"}
	if _, err := resolveKey(); err == nil || !strings.Contains(err.Error(), "klaviyo auth list") {
		t.Errorf("unknown-account err = %v", err)
	}

	// Account exists but has no stored key.
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	acct := cfg.Accounts["prod"]
	acct.APIKey = ""
	cfg.Accounts["prod"] = acct
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	opts = globalOpts{account: "prod"}
	if _, err := resolveKey(); err == nil || !strings.Contains(err.Error(), "re-run") {
		t.Errorf("empty-key err = %v", err)
	}
}

func TestResolveKeyFromKeyring(t *testing.T) {
	seedConfig(t)
	resetOpts(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	acct := cfg.Accounts["prod"]
	acct.APIKey = ""
	acct.KeyStorage = config.KeyStorageKeyring
	cfg.Accounts["prod"] = acct
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	opts = globalOpts{account: "prod"}

	// Key missing from the keychain: point at auth login.
	if _, err := resolveKey(); err == nil || !strings.Contains(err.Error(), "missing from the OS keychain") {
		t.Errorf("missing-key err = %v", err)
	}

	if err := keyring.Set("prod", "pk_ring"); err != nil {
		t.Fatal(err)
	}
	if key, err := resolveKey(); err != nil || key != "pk_ring" {
		t.Errorf("key = %q, err = %v", key, err)
	}

	// Keychain unavailable: mention the KLAVIYO_API_KEY bypass.
	keyring.MockInitWithError(errNoKeychain)
	t.Cleanup(keyring.MockInit)
	if _, err := resolveKey(); err == nil || !strings.Contains(err.Error(), "KLAVIYO_API_KEY") {
		t.Errorf("unavailable err = %v", err)
	}
}
