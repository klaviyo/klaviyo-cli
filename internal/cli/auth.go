package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

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
	if opts.verbose {
		client.Verbose = os.Stderr
	}
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
	var keyStdin, setDefault, insecureStorage bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store an API key for a named account",
		Long: `Store a private API key for a named account.

The key is verified against the Klaviyo API before it is stored in the OS
keychain. If no keychain is available (common on headless Linux and in
containers), pass --insecure-storage to store the key in the CLI config file
(0600 permissions) instead. The account name defaults to the organization
name the key belongs to. The first account added becomes the default;
use --set-default (or ` + "`klaviyo auth switch`" + `) to change it later.

Login is interactive by default (it prompts for the key and name). For
scripts, CI, or agents, either set KLAVIYO_API_KEY for each run (no stored
account needed) or log in by piping the key, keeping it out of shell
history: printf '%s' "$KEY" | klaviyo auth login --api-key-stdin`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			interactive := stdinIsTTY()

			// Key first: verifying it also supplies the organization name
			// that the account name defaults to below.
			if keyStdin {
				raw, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				key = strings.TrimSpace(string(raw))
			}
			if key == "" && !keyStdin {
				if !interactive {
					return errors.New("--api-key or --api-key-stdin is required when not running interactively")
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

			if name == "" {
				def := accountSlug(org)
				if def == "" {
					def = "default"
				}
				if interactive && !keyStdin {
					// With --api-key-stdin the stream is already consumed,
					// so the org-derived default applies without a prompt.
					fmt.Fprintf(cmd.OutOrStdout(), "Account name [%s]: ", def)
					line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
					if err != nil {
						return err
					}
					name = strings.TrimSpace(line)
				}
				if name == "" {
					name = def
				}
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
	// This local --account (the name for the NEW profile) shadows the global
	// -a/--account (the account to use for requests); registering the same
	// shorthand keeps `-a` working here like it does on every other command.
	cmd.Flags().StringVarP(&name, "account", "a", "", "name for this account profile (defaults to the key's organization name)")
	cmd.Flags().StringVar(&key, "api-key", "", "private API key (prompted securely if omitted)")
	cmd.Flags().BoolVar(&keyStdin, "api-key-stdin", false, "read the private API key from stdin (for scripts and agents; keeps the key out of shell history)")
	cmd.Flags().BoolVar(&setDefault, "set-default", false, "make this the default account")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "store the key in the config file instead of the OS keychain")
	cmd.MarkFlagsMutuallyExclusive("api-key", "api-key-stdin")
	return cmd
}

// accountSlug derives a default account name from an organization name:
// "Acme Inc." -> "acme-inc". Empty when nothing usable survives.
func accountSlug(org string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(org) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
		} else {
			pendingHyphen = true
		}
	}
	return b.String()
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
			// tabwriter self-sizes the columns; the old fixed %-20s name
			// column misaligned as soon as a name outgrew it (org-derived
			// default names easily do).
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
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
				fmt.Fprintf(tw, "%s %s\t%s\t%s\t%s\n", marker, name, acct.ID, storage, acct.Organization)
			}
			return tw.Flush()
		},
	}
}

func newAuthSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "switch <account>",
		Short:             "Set the default account (by name or account ID)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAccountNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Accounts[name]; !ok {
				// Not a profile name: try the Klaviyo account ID (sorted
				// for a deterministic pick if two profiles share one).
				found := ""
				for _, n := range slices.Sorted(maps.Keys(cfg.Accounts)) {
					if cfg.Accounts[n].ID == name {
						found = n
						break
					}
				}
				if found == "" {
					return fmt.Errorf("unknown account %q; run `klaviyo auth list`", name)
				}
				name = found
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
