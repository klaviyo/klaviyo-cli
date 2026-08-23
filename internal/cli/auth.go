package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/klaviyo/klaviyo-cli/internal/api"
	"github.com/klaviyo/klaviyo-cli/internal/auth"
	"github.com/klaviyo/klaviyo-cli/internal/config"
)

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
			TestAccount        bool `json:"test_account"`
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
		return "", "", fmt.Errorf("unexpected response verifying key: HTTP %d: %s", resp.StatusCode, truncate(string(resp.Body), 200))
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
	var setDefault bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store an API key for a named account",
		Long: `Store a private API key for a named account in the OS keychain.

The key is verified against the Klaviyo API before it is stored. The first
account added becomes the default; use --set-default (or ` + "`klaviyo auth switch`" + `)
to change it later.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			interactive := term.IsTerminal(int(os.Stdin.Fd()))

			if name == "" {
				if !interactive {
					return errors.New("--account is required when not running interactively")
				}
				fmt.Fprint(cmd.OutOrStdout(), "Account name [default]: ")
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
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

			if err := auth.SetKey(name, key); err != nil {
				return fmt.Errorf("storing key in keychain: %w", err)
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.Accounts[name] = config.Account{ID: id, Organization: org}
			if cfg.DefaultAccount == "" || setDefault {
				cfg.DefaultAccount = name
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %q (%s, account %s)\n", name, org, id)
			if cfg.DefaultAccount == name {
				fmt.Fprintf(cmd.OutOrStdout(), "%q is now the default account\n", name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "account", "", "name for this account profile (prompted if omitted)")
	cmd.Flags().StringVar(&key, "api-key", "", "private API key (prompted securely if omitted)")
	cmd.Flags().BoolVar(&setDefault, "set-default", false, "make this the default account")
	return cmd
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout <account>",
		Short: "Remove a stored account and its key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Accounts[name]; !ok {
				return fmt.Errorf("unknown account %q", name)
			}
			if err := auth.DeleteKey(name); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove keychain entry: %v\n", err)
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
			names := make([]string, 0, len(cfg.Accounts))
			for name := range cfg.Accounts {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				acct := cfg.Accounts[name]
				marker := " "
				if name == cfg.DefaultAccount {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %-20s %-8s %s\n", marker, name, acct.ID, acct.Organization)
			}
			return nil
		},
	}
}

func newAuthSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <account>",
		Short: "Set the default account",
		Args:  cobra.ExactArgs(1),
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
