package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/klaviyo/klaviyo-cli/internal/api"
)

// stdoutIsTTY is stubbed in tests.
var stdoutIsTTY = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

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
	// Interactive terminals get list responses as tables; piped output and
	// errors stay raw JSON so scripts and agents see a stable format.
	if status >= 200 && status < 300 && cmd.OutOrStdout() == io.Writer(os.Stdout) && stdoutIsTTY() {
		if items, ok := parseResourceList(body); ok {
			renderTable(cmd.OutOrStdout(), items)
			return nil
		}
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

type resourceItem struct {
	ID         string         `json:"id"`
	Attributes map[string]any `json:"attributes"`
}

// parseResourceList reports whether body is a JSON:API list response.
func parseResourceList(body []byte) ([]resourceItem, bool) {
	var parsed struct {
		Data []resourceItem `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Data == nil {
		return nil, false
	}
	return parsed.Data, true
}

// Attribute columns shown first when present, in this order.
var preferredColumns = []string{"name", "email", "title", "subject", "status", "created", "updated"}

const maxAttrColumns = 4

// renderTable prints a JSON:API resource list as an aligned table: ID plus up
// to maxAttrColumns scalar attributes, preferring well-known fields.
func renderTable(w io.Writer, items []resourceItem) {
	if len(items) == 0 {
		fmt.Fprintln(w, "No results")
		return
	}
	cols := chooseColumns(items[0].Attributes)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	header := "ID"
	for _, c := range cols {
		header += "\t" + c
	}
	fmt.Fprintln(tw, header)
	for _, item := range items {
		row := item.ID
		for _, c := range cols {
			row += "\t" + cellValue(item.Attributes[c])
		}
		fmt.Fprintln(tw, row)
	}
	_ = tw.Flush()
}

func chooseColumns(attrs map[string]any) []string {
	var cols []string
	for _, c := range preferredColumns {
		if _, ok := attrs[c]; ok && len(cols) < maxAttrColumns {
			cols = append(cols, c)
		}
	}
	if len(cols) < maxAttrColumns {
		var rest []string
		for k, v := range attrs {
			switch v.(type) {
			case string, float64, bool:
				if !slicesContains(cols, k) {
					rest = append(rest, k)
				}
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			if len(cols) >= maxAttrColumns {
				break
			}
			cols = append(cols, k)
		}
	}
	return cols
}

func cellValue(v any) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if len(s) > 40 {
		s = s[:37] + "..."
	}
	return s
}

func slicesContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
