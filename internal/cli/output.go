package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/api"
)

// apiDoer is the client surface commands need; satisfied by *api.Client and
// stubbed in tests.
type apiDoer interface {
	Do(ctx context.Context, method, path string, query url.Values, body []byte) (*api.Response, error)
}

// printResponse pretty-prints a JSON response body and returns an error for
// non-2xx statuses (after printing the body, so API error details are shown).
func printResponse(cmd *cobra.Command, body []byte, status int) error {
	out := body
	var pretty bytes.Buffer
	if json.Indent(&pretty, body, "", "  ") == nil {
		out = pretty.Bytes()
	}
	if len(out) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("HTTP %d", status)
	}
	return nil
}
