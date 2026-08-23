// Package config reads and writes the CLI's on-disk configuration.
//
// Config lives at ~/.config/klaviyo/config.toml (override the directory with
// KLAVIYO_CONFIG_DIR). It holds account profiles including their private API
// keys, so it is written with 0600 permissions. OS keychain storage is
// planned: https://github.com/klaviyo/klaviyo-cli/issues/1
package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Account is one named Klaviyo account profile.
type Account struct {
	// ID is the Klaviyo account ID (from GET /api/accounts/).
	ID string `toml:"id,omitempty"`
	// Organization is the account's organization name, for display.
	Organization string `toml:"organization,omitempty"`
	// APIKey is the account's private API key.
	APIKey string `toml:"api_key,omitempty"`
}

// Config is the root of config.toml.
type Config struct {
	DefaultAccount string             `toml:"default_account,omitempty"`
	Accounts       map[string]Account `toml:"accounts,omitempty"`
}

// Dir returns the config directory, honoring KLAVIYO_CONFIG_DIR.
func Dir() (string, error) {
	if d := os.Getenv("KLAVIYO_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".config", "klaviyo"), nil
}

// Path returns the full path to config.toml.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads config.toml, returning an empty config if the file is absent.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{Accounts: map[string]Account{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cfg.Accounts == nil {
		cfg.Accounts = map[string]Account{}
	}
	return cfg, nil
}

// Save writes the config with restrictive permissions (0700 dir, 0600 file).
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
