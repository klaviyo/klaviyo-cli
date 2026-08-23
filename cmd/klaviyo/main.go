// Command klaviyo is the Klaviyo CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/klaviyo/klaviyo-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
