package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newAPICmd() *cobra.Command {
	var data []string
	var queries []string
	var paginate bool

	cmd := &cobra.Command{
		Use:   "api [method] <path>",
		Short: "Make an authenticated Klaviyo API request",
		Long: `Make a raw request against any Klaviyo API endpoint. The method defaults
to GET when only a path is given. Responses are pretty-printed JSON.

Request bodies are built from repeated -d path=value pairs, where dots in
the path nest objects and := assigns a JSON value instead of a string.
A single -d can instead supply the whole body: inline JSON, @file, or '-'
for stdin.

Examples:
  klaviyo api /api/metrics/
  klaviyo api /api/profiles/ -q 'filter=equals(email,"a@b.com")'
  klaviyo api POST /api/lists/ -d data.type=list -d data.attributes.name=Newsletter
  klaviyo api POST /api/events/ -d data.type=event -d 'data.attributes.properties:={"item":"shirt","count":2}'
  klaviyo api POST /api/events/ -d @event.json
  echo '{...}' | klaviyo api POST /api/lists/ -d -`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method, path := "GET", args[0]
			if len(args) == 2 {
				method, path = strings.ToUpper(args[0]), args[1]
			}

			body, err := readBody(data)
			if err != nil {
				return err
			}
			query, err := parseQueries(queries)
			if err != nil {
				return err
			}

			client, err := newClient()
			if err != nil {
				return err
			}
			if paginate {
				if method != "GET" {
					return fmt.Errorf("--paginate requires a GET request, got %s", method)
				}
				return runPaginated(cmd, client, path, query)
			}
			resp, err := client.Do(cmd.Context(), method, path, query, body)
			if err != nil {
				return err
			}
			return printResponse(cmd, resp)
		},
	}
	cmd.Flags().StringArrayVarP(&data, "data", "d", nil, dataFlagHelp)
	cmd.Flags().StringArrayVarP(&queries, "query", "q", nil, "query parameter as key=value (repeatable)")
	cmd.Flags().BoolVar(&paginate, "paginate", false, "follow cursor pagination and merge all pages' data and included (meta is per-page and dropped; GET only)")
	return cmd
}

const dataFlagHelp = "request body: repeatable path=value pairs rooted at the body's top level, so attributes need their full path (e.g. data.attributes.name=X; dots nest objects, ':=' assigns JSON values), or a single inline JSON, @file, or '-' for stdin"

// isWholeBody reports whether the --data values are a single argument
// supplying the whole body (stdin, @file, or inline JSON) rather than
// path=value field pairs.
func isWholeBody(data []string) bool {
	if len(data) != 1 {
		return false
	}
	d := strings.TrimSpace(data[0])
	return data[0] == "-" || strings.HasPrefix(data[0], "@") ||
		strings.HasPrefix(d, "{") || strings.HasPrefix(d, "[")
}

// readBody assembles the request body from the repeatable --data flag. A
// single value that is '-', @file, or an inline JSON document supplies the
// whole body; anything else is treated as path=value field pairs.
func readBody(data []string) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if isWholeBody(data) {
		switch d := data[0]; {
		case d == "-":
			body, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("reading body from stdin: %w", err)
			}
			return body, nil
		case strings.HasPrefix(d, "@"):
			body, err := os.ReadFile(d[1:])
			if err != nil {
				return nil, fmt.Errorf("reading body file: %w", err)
			}
			return body, nil
		default:
			return []byte(d), nil
		}
	}
	root := map[string]any{}
	if err := applyPairs(root, data); err != nil {
		return nil, err
	}
	return json.Marshal(root)
}

// applyPairs writes path=value field pairs into root as one nested JSON
// object. Dots in the path nest objects (data.attributes.name=x). '='
// assigns a string; ':=' assigns the raw JSON value (numbers, booleans,
// arrays, objects) — which is also the escape hatch for keys that
// themselves contain dots, by assigning a JSON object one level up.
func applyPairs(root map[string]any, pairs []string) error {
	for _, pair := range pairs {
		path, value, found := strings.Cut(pair, "=")
		if !found || path == "" || path == ":" {
			return fmt.Errorf("invalid body field %q; expected path=value or path:=json (or a single -d with inline JSON, @file, or '-')", pair)
		}
		var v any
		if strings.HasSuffix(path, ":") {
			path = strings.TrimSuffix(path, ":")
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				return fmt.Errorf("invalid JSON value for field %q: %w", path, err)
			}
		} else {
			v = value
		}
		if err := setField(root, path, v); err != nil {
			return err
		}
	}
	return nil
}

// setField writes value at the dot-separated path, creating intermediate
// objects. Conflicting or duplicate paths are errors rather than silent
// overwrites.
func setField(root map[string]any, path string, value any) error {
	segs := strings.Split(path, ".")
	m := root
	for i, seg := range segs {
		if seg == "" {
			return fmt.Errorf("invalid field path %q: empty segment", path)
		}
		if i == len(segs)-1 {
			if _, exists := m[seg]; exists {
				return fmt.Errorf("field %q conflicts with an earlier -d value", path)
			}
			m[seg] = value
			return nil
		}
		next, ok := m[seg]
		if !ok {
			child := map[string]any{}
			m[seg] = child
			m = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("field %q conflicts with the earlier value at %q", path, strings.Join(segs[:i+1], "."))
		}
		m = child
	}
	return nil
}

func parseQueries(queries []string) (url.Values, error) {
	if len(queries) == 0 {
		return nil, nil
	}
	values := url.Values{}
	for _, q := range queries {
		key, value, found := strings.Cut(q, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("invalid query parameter %q; expected key=value", q)
		}
		values.Add(key, value)
	}
	return values, nil
}
