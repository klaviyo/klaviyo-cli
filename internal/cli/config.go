package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/config"
)

// settableKeys whitelists config keys manageable via `config --set/--unset`.
// Account profiles themselves are managed by `auth` commands.
var settableKeys = map[string]bool{"default_account": true}

func newConfigCmd() *cobra.Command {
	var list, edit bool
	var setKey, unsetKey bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long: `Manage the CLI configuration file (` + "`klaviyo config --list`" + ` shows its
location and contents; API keys are redacted in output).

Examples:
  klaviyo config --list
  klaviyo config --set default_account prod
  klaviyo config --unset default_account
  klaviyo config -e   # open the config file in your editor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case list:
				return runConfigList(cmd)
			case setKey:
				if len(args) != 2 {
					return errors.New("--set requires a key and a value, e.g. --set default_account prod")
				}
				return runConfigSet(cmd, args[0], args[1])
			case unsetKey:
				if len(args) != 1 {
					return errors.New("--unset requires a key, e.g. --unset default_account")
				}
				return runConfigSet(cmd, args[0], "")
			case edit:
				return runConfigEdit(cmd)
			default:
				return cmd.Help()
			}
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "print the config file path and contents (keys redacted)")
	cmd.Flags().BoolVar(&setKey, "set", false, "set a config key: --set <key> <value>")
	cmd.Flags().BoolVar(&unsetKey, "unset", false, "clear a config key: --unset <key>")
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "open the config file in your editor")
	return cmd
}

func runConfigList(cmd *cobra.Command) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for name, acct := range cfg.Accounts {
		if acct.APIKey != "" {
			acct.APIKey = "<redacted>"
			cfg.Accounts[name] = acct
		}
	}
	out, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "# %s\n%s", path, out)
	return nil
}

func runConfigSet(cmd *cobra.Command, key, value string) error {
	if !settableKeys[key] {
		return fmt.Errorf("unknown config key %q (settable: default_account); accounts are managed with `klaviyo auth`", key)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if key == "default_account" {
		if _, ok := cfg.Accounts[value]; value != "" && !ok {
			return fmt.Errorf("unknown account %q; run `klaviyo auth list`", value)
		}
		cfg.DefaultAccount = value
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	if value == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Unset %s\n", key)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
	}
	return nil
}

func runConfigEdit(cmd *cobra.Command) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	edit := exec.Command(editor, path)
	edit.Stdin, edit.Stdout, edit.Stderr = os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr()
	return edit.Run()
}
