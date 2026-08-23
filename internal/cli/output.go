package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/klaviyo/klaviyo-cli/internal/api"
)

// stdoutIsTTY is stubbed in tests.
var stdoutIsTTY = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// shouldRenderTable decides whether list responses render as tables: only
// when writing to a real interactive terminal, so piped output stays raw
// JSON. Stubbed in tests.
var shouldRenderTable = func(w io.Writer) bool {
	return w == io.Writer(os.Stdout) && stdoutIsTTY()
}

// apiDoer is the client surface commands need; satisfied by *api.Client and
// stubbed in tests.
type apiDoer interface {
	Do(ctx context.Context, method, path string, query url.Values, body []byte) (*api.Response, error)
}

// printResponse renders a JSON response — pretty-printed, filtered through
// --jq when set, or as a table for list responses on interactive terminals —
// and returns an error for non-2xx statuses (after printing the body, so API
// error details are shown).
func printResponse(cmd *cobra.Command, resp *api.Response) error {
	w := cmd.OutOrStdout()
	if opts.jq != "" && resp.OK() {
		return applyJQ(w, resp.Body, opts.jq, shouldRenderTable(w))
	}
	if resp.OK() && shouldRenderTable(w) {
		if items, ok := parseResourceList(resp.Body); ok {
			renderTable(w, items)
			return nil
		}
	}
	out := resp.Body
	var pretty bytes.Buffer
	if json.Indent(&pretty, resp.Body, "", "  ") == nil {
		out = pretty.Bytes()
	} else if shouldRenderTable(w) {
		// Not valid JSON, going to a terminal: neutralize control bytes so
		// a hostile response cannot inject escape sequences. (Valid JSON
		// keeps control characters \u-escaped, so the pretty path is safe.)
		out = []byte(sanitizeTerminal(string(out)))
	}
	if len(out) > 0 {
		fmt.Fprintln(w, string(out))
	}
	if !resp.OK() {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// applyJQ runs a jq expression over a JSON body. Following jq/gh conventions,
// string results print raw and other values print as JSON, one result per
// line. Raw strings going to a terminal are sanitized against escape
// injection; piped output is byte-faithful.
func applyJQ(w io.Writer, body []byte, expr string, toTerminal bool) error {
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
			if toTerminal {
				s = sanitizeTerminal(s)
			}
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
// to maxAttrColumns scalar attributes, preferring well-known fields. Cell
// values are attacker-influenced (profile names, campaign subjects) and this
// path only runs on terminals, so cells are sanitized against escape
// injection.
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
		row := sanitizeTerminal(item.ID)
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
	for _, k := range slices.Sorted(maps.Keys(attrs)) {
		if len(cols) >= maxAttrColumns {
			break
		}
		switch attrs[k].(type) {
		case string, float64, bool:
			if !slices.Contains(cols, k) {
				cols = append(cols, k)
			}
		}
	}
	return cols
}

func cellValue(v any) string {
	if v == nil {
		return ""
	}
	return sanitizeTerminal(truncate(fmt.Sprintf("%v", v), 40))
}

// truncate shortens s to at most n runes, appending "..." when cut.
// Rune-based so multi-byte UTF-8 is never split.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-3]) + "..."
}

// sanitizeTerminal renders control characters as visible escapes (e.g. ESC
// becomes \x1b) so server-supplied text cannot inject terminal escape
// sequences. Tabs and newlines pass through where they are meaningful.
func sanitizeTerminal(s string) string {
	if !strings.ContainsFunc(s, isUnsafeRune) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if isUnsafeRune(r) {
			fmt.Fprintf(&b, "\\x%02x", r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isUnsafeRune(r rune) bool {
	return (r < 0x20 && r != '\t' && r != '\n') || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}
