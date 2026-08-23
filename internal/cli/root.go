// Package cli defines the klaviyo command tree.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/api"
	"github.com/klaviyo/klaviyo-cli/internal/config"
)

// Set at release time via goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type globalOpts struct {
	account  string
	apiKey   string
	revision string
	jq       string
}

var opts globalOpts

// Execute runs the root command, checking for newer releases in the
// background and printing a notice to stderr when one exists.
func Execute() error {
	var updateCh <-chan string
	if shouldCheckForUpdate() {
		updateCh = startUpdateCheck()
	}
	err := newRootCmd().Execute()
	if updateCh != nil {
		maybeNotifyUpdate(updateCh)
	}
	return err
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "klaviyo",
		Short: "Manage Klaviyo from the command line",
		Long: `The Klaviyo CLI wraps the Klaviyo API: authenticate once per account,
then call any endpoint or resource command against it.

Get started:
  klaviyo auth login          # store an API key for an account
  klaviyo api /api/metrics/   # make an authenticated request`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&opts.account, "account", "a", "", "named account to use (default: KLAVIYO_ACCOUNT env, then configured default)")
	pf.StringVar(&opts.apiKey, "api-key", "", "private API key, bypassing stored accounts (prefer KLAVIYO_API_KEY env)")
	pf.StringVar(&opts.revision, "revision", "", "API revision header (default "+api.DefaultRevision+")")
	pf.StringVar(&opts.jq, "jq", "", "filter the JSON response through a jq expression (built in, no jq install needed)")

	cobra.CheckErr(root.RegisterFlagCompletionFunc("account", completeAccountNames))

	root.AddCommand(newAuthCmd(), newAPICmd(), newVersionCmd())
	addResourceCmds(root)
	return root
}

// completeAccountNames shell-completes configured account names.
func completeAccountNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]string, 0, len(cfg.Accounts))
	for name, acct := range cfg.Accounts {
		names = append(names, fmt.Sprintf("%s\t%s", name, acct.Organization))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// resolveAccountName returns the effective account name, which may be empty
// when the key comes from a flag or environment variable.
func resolveAccountName(cfg *config.Config) string {
	if opts.account != "" {
		return opts.account
	}
	if env := os.Getenv("KLAVIYO_ACCOUNT"); env != "" {
		return env
	}
	return cfg.DefaultAccount
}

// resolveKey returns the API key to use, in precedence order:
// --api-key flag, KLAVIYO_API_KEY env, then the selected account's stored
// key.
func resolveKey() (string, error) {
	if opts.apiKey != "" {
		return opts.apiKey, nil
	}
	if env := os.Getenv("KLAVIYO_API_KEY"); env != "" {
		return env, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	name := resolveAccountName(cfg)
	if name == "" {
		return "", errors.New("no account configured; run `klaviyo auth login` or set KLAVIYO_API_KEY")
	}
	acct, ok := cfg.Accounts[name]
	if !ok {
		return "", fmt.Errorf("unknown account %q; run `klaviyo auth list`", name)
	}
	if acct.APIKey == "" {
		return "", fmt.Errorf("no stored key for account %q; re-run `klaviyo auth login --account %s`", name, name)
	}
	return acct.APIKey, nil
}

// newClient builds an API client for the selected account.
func newClient() (*api.Client, error) {
	key, err := resolveKey()
	if err != nil {
		return nil, err
	}
	return api.NewClient(key, opts.revision, version), nil
}
