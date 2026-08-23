package cli

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newAPICmd() *cobra.Command {
	var data string
	var queries []string

	cmd := &cobra.Command{
		Use:   "api [method] <path>",
		Short: "Make an authenticated Klaviyo API request",
		Long: `Make a raw request against any Klaviyo API endpoint. The method defaults
to GET when only a path is given. Responses are pretty-printed JSON.

Examples:
  klaviyo api /api/metrics/
  klaviyo api /api/profiles/ -q 'filter=equals(email,"a@b.com")'
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
			resp, err := client.Do(cmd.Context(), method, path, query, body)
			if err != nil {
				return err
			}
			return printResponse(cmd, resp.Body, resp.StatusCode)
		},
	}
	cmd.Flags().StringVarP(&data, "data", "d", "", "request body: inline JSON, @file, or '-' for stdin")
	cmd.Flags().StringArrayVarP(&queries, "query", "q", nil, "query parameter as key=value (repeatable)")
	return cmd
}

func readBody(data string) ([]byte, error) {
	switch {
	case data == "":
		return nil, nil
	case data == "-":
		body, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading body from stdin: %w", err)
		}
		return body, nil
	case strings.HasPrefix(data, "@"):
		body, err := os.ReadFile(data[1:])
		if err != nil {
			return nil, fmt.Errorf("reading body file: %w", err)
		}
		return body, nil
	default:
		return []byte(data), nil
	}
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
