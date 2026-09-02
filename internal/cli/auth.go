package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/klaviyo/klaviyo-cli/internal/api"
	"github.com/klaviyo/klaviyo-cli/internal/config"
	"github.com/klaviyo/klaviyo-cli/internal/keyring"
)

// stdinIsTTY is stubbed in tests.
var stdinIsTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate and manage accounts",
	}
	cmd.AddCommand(
		newAuthLoginCmd(),
		newAuthLogoutCmd(),
		newAuthListCmd(),
		newAuthSwitchCmd(),
		newAuthStatusCmd(),
	)
	return cmd
}

// accountsResponse is the shape of GET /api/accounts/ we care about.
type accountsResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			ContactInformation struct {
				OrganizationName string `json:"organization_name"`
			} `json:"contact_information"`
		} `json:"attributes"`
	} `json:"data"`
}

// verifyKey checks the key against GET /api/accounts/ and returns the
// account ID and organization name.
func verifyKey(ctx context.Context, key string) (id, org string, err error) {
	client := api.NewClient(key, opts.revision, version)
	resp, err := client.Do(ctx, "GET", "/api/accounts/", nil, nil)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return "", "", errors.New("API key was rejected (401/403); check the key and its scopes")
	}
	if !resp.OK() {
		return "", "", fmt.Errorf("unexpected response verifying key: HTTP %d: %s",
			resp.StatusCode, sanitizeTerminal(truncate(string(resp.Body), 200)))
	}
	var parsed accountsResponse
	if err := json.Unmarshal(resp.Body, &parsed); err != nil {
		return "", "", fmt.Errorf("parsing accounts response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return "", "", errors.New("key verified but no account returned")
	}
	acct := parsed.Data[0]
	return acct.ID, acct.Attributes.ContactInformation.OrganizationName, nil
}

func newAuthLoginCmd() *cobra.Command {
	var name, key string
	var setDefault, insecureStorage bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store an API key for a named account",
		Long: `Store a private API key for a named account.

The key is verified against the Klaviyo API before it is stored in the OS
keychain. If no keychain is available (common on headless Linux and in
containers), pass --insecure-storage to store the key in the CLI config file
(0600 permissions) instead. The first account added becomes the default;
use --set-default (or ` + "`klaviyo auth switch`" + `) to change it later.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			interactive := stdinIsTTY()

			if name == "" {
				if !interactive {
					return errors.New("--account is required when not running interactively")
				}
				fmt.Fprint(cmd.OutOrStdout(), "Account name [default]: ")
				line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if err != nil {
					return err
				}
				name = strings.TrimSpace(line)
				if name == "" {
					name = "default"
				}
			}

			if key == "" {
				if !interactive {
					return errors.New("--api-key is required when not running interactively")
				}
				fmt.Fprint(cmd.OutOrStdout(), "Private API key (input hidden): ")
				raw, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return err
				}
				key = strings.TrimSpace(string(raw))
			}
			if key == "" {
				return errors.New("no API key provided")
			}

			id, org, err := verifyKey(cmd.Context(), key)
			if err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acct := config.Account{ID: id, Organization: org}
			if insecureStorage {
				acct.APIKey = key
				// Drop any key a previous keyring-backed login left behind.
				if cfg.Accounts[name].KeyStorage == config.KeyStorageKeyring {
					if err := keyring.Delete(name); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove old key from OS keychain: %v\n", err)
					}
				}
			} else {
				if err := keyring.Set(name, key); err != nil {
					return fmt.Errorf("storing key in OS keychain: %w (pass --insecure-storage to store it in the config file instead)", err)
				}
				acct.KeyStorage = config.KeyStorageKeyring
			}
			cfg.Accounts[name] = acct
			if cfg.DefaultAccount == "" || setDefault {
				cfg.DefaultAccount = name
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %q (%s, account %s)\n", name, org, id)
			if insecureStorage {
				path, _ := config.Path()
				fmt.Fprintf(cmd.OutOrStdout(), "Key stored in %s\n", path)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Key stored in the OS keychain")
			}
			if cfg.DefaultAccount == name {
				fmt.Fprintf(cmd.OutOrStdout(), "%q is now the default account\n", name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "account", "", "name for this account profile (prompted if omitted)")
	cmd.Flags().StringVar(&key, "api-key", "", "private API key (prompted securely if omitted)")
	cmd.Flags().BoolVar(&setDefault, "set-default", false, "make this the default account")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "store the key in the config file instead of the OS keychain")
	return cmd
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "logout <account>",
		Short:             "Remove a stored account and its key",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAccountNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			acct, ok := cfg.Accounts[name]
			if !ok {
				return fmt.Errorf("unknown account %q", name)
			}
			if acct.KeyStorage == config.KeyStorageKeyring {
				if err := keyring.Delete(name); err != nil {
					// Still remove the profile; don't strand the user.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove key from OS keychain: %v\n", err)
				}
			}
			delete(cfg.Accounts, name)
			if cfg.DefaultAccount == name {
				cfg.DefaultAccount = ""
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged out of %q\n", name)
			if cfg.DefaultAccount == "" && len(cfg.Accounts) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No default account set; run `klaviyo auth switch <account>`")
			}
			return nil
		},
	}
}

func newAuthListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Accounts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No accounts configured; run `klaviyo auth login`")
				return nil
			}
			for _, name := range slices.Sorted(maps.Keys(cfg.Accounts)) {
				acct := cfg.Accounts[name]
				marker := " "
				if name == cfg.DefaultAccount {
					marker = "*"
				}
				storage := "file"
				if acct.KeyStorage == config.KeyStorageKeyring {
					storage = "keyring"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %-20s %-8s %-8s %s\n", marker, name, acct.ID, storage, acct.Organization)
			}
			return nil
		},
	}
}

func newAuthSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "switch <account>",
		Short:             "Set the default account",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAccountNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Accounts[name]; !ok {
				return fmt.Errorf("unknown account %q; run `klaviyo auth list`", name)
			}
			cfg.DefaultAccount = name
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default account is now %q\n", name)
			return nil
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Verify credentials for the selected account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, err := resolveKey()
			if err != nil {
				return err
			}
			id, org, err := verifyKey(cmd.Context(), key)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Authenticated to %s (account %s)\n", org, id)
			return nil
		},
	}
}
