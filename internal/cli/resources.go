package cli

// resources_gen.go and ../api/revision_gen.go are generated from the vendored
// OpenAPI spec by klaviyo-cli-gen, an internal Klaviyo tool; regeneration
// lands via reviewed PRs when the spec updates.

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/klaviyo/klaviyo-cli/internal/api"
)

// queryParamSpec pairs an API query parameter name (e.g. "page[size]") with
// its CLI flag name (e.g. "page-size") and one-line help text.
type queryParamSpec struct {
	Name string
	Flag string
	Help string
}

// opSpec describes one generated API operation command.
type opSpec struct {
	Group      string
	Name       string
	Summary    string
	Method     string
	Path       string
	PathParams []string
	Query      []queryParamSpec
	HasBody    bool
	Paginated  bool
}

// maxPages bounds --paginate as a runaway-loop backstop.
const maxPages = 1000

// addResourceCmds registers one command group per spec tag, with one
// subcommand per generated operation (see resources_gen.go).
func addResourceCmds(root *cobra.Command) {
	groups := map[string]*cobra.Command{}
	for i := range resourceOps {
		op := &resourceOps[i]
		parent, ok := groups[op.Group]
		if !ok {
			parent = &cobra.Command{
				Use:   op.Group,
				Short: fmt.Sprintf("Manage %s", strings.ReplaceAll(op.Group, "-", " ")),
			}
			groups[op.Group] = parent
		}
		parent.AddCommand(newResourceOpCmd(op))
	}
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		root.AddCommand(groups[name])
	}
}

func newResourceOpCmd(op *opSpec) *cobra.Command {
	use := op.Name
	for _, p := range op.PathParams {
		use += fmt.Sprintf(" <%s>", p)
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: op.Summary,
		Long:  fmt.Sprintf("%s\n\nCalls %s %s.", op.Summary, op.Method, op.Path),
		Args:  cobra.ExactArgs(len(op.PathParams)),
	}

	queryFlags := make([]*string, len(op.Query))
	for i, q := range op.Query {
		queryFlags[i] = cmd.Flags().String(q.Flag, "", q.Help)
	}
	var data string
	if op.HasBody {
		cmd.Flags().StringVarP(&data, "data", "d", "", "request body: inline JSON, @file, or '-' for stdin")
	}
	var paginate bool
	if op.Paginated {
		cmd.Flags().BoolVar(&paginate, "paginate", false, "follow cursor pagination and merge all pages' data")
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		path := resolvePath(op.Path, op.PathParams, args)
		query := url.Values{}
		for i, q := range op.Query {
			if *queryFlags[i] != "" {
				query.Set(q.Name, *queryFlags[i])
			}
		}
		var body []byte
		if op.HasBody {
			var err error
			if body, err = readBody(data); err != nil {
				return err
			}
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		if paginate {
			return runPaginated(cmd, client, path, query)
		}
		resp, err := client.Do(cmd.Context(), op.Method, path, query, body)
		if err != nil {
			return err
		}
		return printResponse(cmd, resp)
	}
	return cmd
}

// resolvePath substitutes {param} placeholders with positional args.
func resolvePath(path string, params, args []string) string {
	for i, p := range params {
		path = strings.ReplaceAll(path, "{"+p+"}", url.PathEscape(args[i]))
	}
	return path
}

// runPaginated follows links.next until exhausted, merging every page's
// "data" array into a single {"data": [...]} document.
func runPaginated(cmd *cobra.Command, client apiDoer, path string, query url.Values) error {
	var merged []json.RawMessage
	nextLink := ""
	for page := 0; page < maxPages; page++ {
		resp, err := client.Do(cmd.Context(), "GET", path, query, nil)
		if err != nil {
			return err
		}
		if !resp.OK() {
			return printResponse(cmd, resp)
		}
		var parsed struct {
			Data  []json.RawMessage `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(resp.Body, &parsed); err != nil {
			return fmt.Errorf("parsing page %d: %w", page+1, err)
		}
		merged = append(merged, parsed.Data...)
		nextLink = parsed.Links.Next
		if nextLink == "" {
			break
		}
		next, err := url.Parse(nextLink)
		if err != nil {
			return fmt.Errorf("parsing next link: %w", err)
		}
		// Deliberately keep only path+query from the server-supplied next
		// link, never its scheme or host: following a full URL would let a
		// hostile response walk the Authorization header to another server.
		path = next.Path
		query, err = url.ParseQuery(next.RawQuery)
		if err != nil {
			return fmt.Errorf("parsing next link query: %w", err)
		}
	}
	if nextLink != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: stopped after %d pages with more results remaining\n", maxPages)
	}
	out, err := json.Marshal(map[string]any{"data": merged})
	if err != nil {
		return err
	}
	return printResponse(cmd, &api.Response{StatusCode: 200, Body: out})
}
