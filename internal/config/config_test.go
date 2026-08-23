package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultAccount != "" || len(cfg.Accounts) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())

	in := &Config{
		DefaultAccount: "prod",
		Accounts: map[string]Account{
			"prod":    {ID: "AbC123", Organization: "Acme Inc", APIKey: "pk_secret"},
			"staging": {ID: "XyZ789", Organization: "Acme Staging"},
		},
	}
	if err := in.Save(); err != nil {
		t.Fatal(err)
	}

	out, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if out.DefaultAccount != "prod" {
		t.Errorf("DefaultAccount = %q", out.DefaultAccount)
	}
	if len(out.Accounts) != 2 {
		t.Fatalf("Accounts = %+v", out.Accounts)
	}
	if out.Accounts["prod"].Organization != "Acme Inc" {
		t.Errorf("prod org = %q", out.Accounts["prod"].Organization)
	}
	if out.Accounts["prod"].APIKey != "pk_secret" {
		t.Errorf("prod api key = %q", out.Accounts["prod"].APIKey)
	}
}

func TestSaveTightensExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission semantics")
	}
	dir := t.TempDir()
	t.Setenv("KLAVIYO_CONFIG_DIR", dir)

	// Simulate a pre-existing world-readable file and permissive dir, as an
	// editor or dotfiles script might leave them.
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Accounts: map[string]Account{"prod": {APIKey: "pk_secret"}}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 600", perm)
	}
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir mode = %o, want 700", perm)
	}
}

func TestSaveRefusesSymlinkedConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics")
	}
	dir := t.TempDir()
	t.Setenv("KLAVIYO_CONFIG_DIR", dir)

	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Accounts: map[string]Account{"prod": {APIKey: "pk_secret"}}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	// The rename replaces the symlink itself; the target is untouched.
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Errorf("symlink target was written through: %q", data)
	}
}
