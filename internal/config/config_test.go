package config

import (
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
			"prod":    {ID: "AbC123", Organization: "Acme Inc"},
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
}
