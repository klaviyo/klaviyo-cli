// Package config reads and writes the CLI's on-disk configuration.
//
// Config lives at ~/.config/klaviyo/config.toml (override the directory with
// KLAVIYO_CONFIG_DIR). It holds account profiles; private API keys live in
// the OS keychain (internal/keyring) by default, or inline in this file when
// stored with --insecure-storage, so it is written with 0600 permissions.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// KeyStorageKeyring marks an account whose API key lives in the OS keychain
// rather than in this file.
const KeyStorageKeyring = "keyring"

// Account is one named Klaviyo account profile.
type Account struct {
	// ID is the Klaviyo account ID (from GET /api/accounts/).
	ID string `toml:"id,omitempty"`
	// Organization is the account's organization name, for display.
	Organization string `toml:"organization,omitempty"`
	// APIKey is the account's private API key when stored in this file;
	// empty when the key lives in the OS keychain.
	APIKey string `toml:"api_key,omitempty"`
	// KeyStorage is KeyStorageKeyring when the key is in the OS keychain,
	// empty for file storage (the pre-keychain format).
	KeyStorage string `toml:"key_storage,omitempty"`
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

// Save writes the config with restrictive permissions (0700 dir, 0600 file),
// enforced even when the directory or file already exists with looser modes
// (os.MkdirAll and os.WriteFile only apply modes at creation). The write is
// atomic — exclusive temp file then rename — so an interrupt cannot leave a
// truncated config, and a symlinked config.toml is never written through.
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("securing %s: %w", dir, err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
