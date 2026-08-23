package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/api"
)

// apiDoer is the client surface commands need; satisfied by *api.Client and
// stubbed in tests.
type apiDoer interface {
	Do(ctx context.Context, method, path string, query url.Values, body []byte) (*api.Response, error)
}

// printResponse renders a JSON response body — pretty-printed, or filtered
// through --jq when set — and returns an error for non-2xx statuses (after
// printing the body, so API error details are shown).
func printResponse(cmd *cobra.Command, body []byte, status int) error {
	if opts.jq != "" && status >= 200 && status < 300 {
		if err := applyJQ(cmd.OutOrStdout(), body, opts.jq); err != nil {
			return err
		}
		return nil
	}
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

// applyJQ runs a jq expression over a JSON body. Following jq/gh conventions,
// string results print raw and other values print as JSON, one result per
// line.
func applyJQ(w io.Writer, body []byte, expr string) error {
	query, err := gojq.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid --jq expression: %w", err)
	}
	var input any
	if err := json.Unmarshal(body, &input); err != nil {
		return fmt.Errorf("--jq requires a JSON response: %w", err)
	}
	iter := query.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, isErr := v.(error); isErr {
			return fmt.Errorf("jq: %w", err)
		}
		if s, isStr := v.(string); isStr {
			fmt.Fprintln(w, s)
			continue
		}
		out, err := json.Marshal(v)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(out))
	}
}
