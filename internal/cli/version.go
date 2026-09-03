package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionString is the one version line, shared by `klaviyo version` and
// the root --version/-v flag so all three forms print identically.
func versionString() string {
	return fmt.Sprintf("klaviyo %s (commit %s, built %s)\n", version, commit, date)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprint(cmd.OutOrStdout(), versionString())
		},
	}
}
