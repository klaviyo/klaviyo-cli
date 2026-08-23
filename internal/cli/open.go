package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"

	"github.com/spf13/cobra"
)

// openShortcuts maps `klaviyo open` shortcuts to URLs, in the spirit of
// `stripe open`.
var openShortcuts = map[string]string{
	"dashboard": "https://www.klaviyo.com/dashboard",
	"campaigns": "https://www.klaviyo.com/campaigns",
	"flows":     "https://www.klaviyo.com/flows",
	"lists":     "https://www.klaviyo.com/lists",
	"profiles":  "https://www.klaviyo.com/profiles",
	"templates": "https://www.klaviyo.com/email-templates",
	"forms":     "https://www.klaviyo.com/forms",
	"analytics": "https://www.klaviyo.com/analytics/dashboard",
	"settings":  "https://www.klaviyo.com/settings",
	"api-keys":  "https://www.klaviyo.com/settings/account/api-keys",
	"docs":      "https://developers.klaviyo.com",
	"api":       "https://developers.klaviyo.com/en/reference/api_overview",
}

// launchBrowser is stubbed in tests.
var launchBrowser = func(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func newOpenCmd() *cobra.Command {
	var list, noBrowser bool

	cmd := &cobra.Command{
		Use:   "open <shortcut>",
		Short: "Open Klaviyo dashboard or docs pages in your browser",
		Long: `Open a Klaviyo dashboard or documentation page in your browser.

Examples:
  klaviyo open dashboard
  klaviyo open api-keys
  klaviyo open docs --no-browser   # print the URL instead of opening it
  klaviyo open --list`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return sortedShortcuts(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list || len(args) == 0 {
				for _, name := range sortedShortcuts() {
					fmt.Fprintf(cmd.OutOrStdout(), "%-12s %s\n", name, openShortcuts[name])
				}
				return nil
			}
			url, ok := openShortcuts[args[0]]
			if !ok {
				return fmt.Errorf("unknown shortcut %q; run `klaviyo open --list`", args[0])
			}
			if noBrowser {
				fmt.Fprintln(cmd.OutOrStdout(), url)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Opening %s\n", url)
			return launchBrowser(url)
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "list available shortcuts")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the URL instead of opening the browser")
	return cmd
}

func sortedShortcuts() []string {
	names := make([]string, 0, len(openShortcuts))
	for name := range openShortcuts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
