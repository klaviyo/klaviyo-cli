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

// bodyFieldSpec is one flag-able request-body attribute: its dot path under
// data.attributes, CLI flag name, schema type (string, integer, number,
// boolean, or array-<scalar>), and one-line help text.
type bodyFieldSpec struct {
	Path string
	Flag string
	Type string
	Help string
}

// opSpec describes one generated API operation command.
type opSpec struct {
	Group       string
	Name        string
	OpID        string // spec operationId; keys the docs page and OAS file URLs
	Summary     string
	Description string // spec description, cleaned for terminal output
	Method      string
	Path        string
	PathParams  []string
	Query       []queryParamSpec
	HasBody     bool
	BodyType    string // JSON:API resource type, auto-filled into data.type
	Body        []bodyFieldSpec
	Paginated   bool
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
		Long:  opLong(op),
		Args:  cobra.ExactArgs(len(op.PathParams)),
	}

	queryFlags := make([]*string, len(op.Query))
	for i, q := range op.Query {
		queryFlags[i] = cmd.Flags().String(q.Flag, "", q.Help)
	}
	var data []string
	bodyStrs := make([]*string, len(op.Body))
	bodyArrs := make([]*[]string, len(op.Body))
	if op.HasBody {
		cmd.Flags().StringArrayVarP(&data, "data", "d", nil, dataFlagHelp)
		for i, f := range op.Body {
			if strings.HasPrefix(f.Type, "array") {
				bodyArrs[i] = cmd.Flags().StringArray(f.Flag, nil, f.Help)
			} else {
				bodyStrs[i] = cmd.Flags().String(f.Flag, "", f.Help)
			}
		}
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
			if body, err = assembleBody(cmd, op, args, data, bodyStrs, bodyArrs); err != nil {
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

// Reference links keyed by an operation's spec operationId: its page on the
// developer docs (URL slugs match operationIds 1:1) and its self-contained
// per-endpoint OAS file in the klaviyo/openapi repo. Both are pinned to the
// revision these commands were generated from — the docs by version prefix
// (the current revision's versioned URL redirects to the plain page), the
// OAS file by its revision branch.
const (
	docsURLFmt = "https://developers.klaviyo.com/en/v%s/reference/%s"
	oasURLFmt  = "https://github.com/klaviyo/openapi/blob/%s/openapi/stable/apis/%s.json"
)

// opLong builds a command's --help long text: summary, the operation's spec
// description, the HTTP call it makes, and reference links.
func opLong(op *opSpec) string {
	var b strings.Builder
	b.WriteString(op.Summary)
	if op.Description != "" {
		b.WriteString("\n\n" + op.Description)
	}
	fmt.Fprintf(&b, "\n\nCalls %s %s.", op.Method, op.Path)
	if op.OpID != "" {
		fmt.Fprintf(&b, "\n\nAPI docs:     "+docsURLFmt, api.DefaultRevision, op.OpID)
		fmt.Fprintf(&b, "\nOpenAPI file: "+oasURLFmt, api.DefaultRevision, op.OpID)
	}
	return b.String()
}

// assembleBody builds the request body from --data and the generated
// per-attribute body flags. A whole-body --data (inline JSON, @file, '-')
// is passed through untouched and cannot combine with body flags;
// otherwise flags and --data field pairs merge into one object (conflicts
// error), and data.type is auto-filled from the spec when absent. For
// PATCH updates of a resource, JSON:API also requires data.id to match the
// URL, so it is auto-filled from the id path argument when absent.
func assembleBody(cmd *cobra.Command, op *opSpec, args []string, data []string, bodyStrs []*string, bodyArrs []*[]string) ([]byte, error) {
	changed := false
	for _, f := range op.Body {
		if cmd.Flags().Changed(f.Flag) {
			changed = true
			break
		}
	}
	if isWholeBody(data) {
		if changed {
			return nil, fmt.Errorf("body attribute flags cannot be combined with a whole --data body; use repeatable -d path=value pairs instead")
		}
		return readBody(data)
	}
	root := map[string]any{}
	if err := applyPairs(root, data); err != nil {
		return nil, err
	}
	for i, f := range op.Body {
		if !cmd.Flags().Changed(f.Flag) {
			continue
		}
		var v any
		if strings.HasPrefix(f.Type, "array") {
			itemType := strings.TrimPrefix(f.Type, "array-")
			items := make([]any, 0, len(*bodyArrs[i]))
			for _, raw := range *bodyArrs[i] {
				item, err := convertBodyValue(itemType, raw, f.Flag)
				if err != nil {
					return nil, err
				}
				items = append(items, item)
			}
			v = items
		} else {
			var err error
			if v, err = convertBodyValue(f.Type, *bodyStrs[i], f.Flag); err != nil {
				return nil, err
			}
		}
		if err := setField(root, "data.attributes."+f.Path, v); err != nil {
			return nil, err
		}
	}
	if len(root) == 0 {
		return nil, nil
	}
	if d, ok := root["data"].(map[string]any); ok {
		if op.BodyType != "" {
			if _, has := d["type"]; !has {
				d["type"] = op.BodyType
			}
		}
		if id := bodyIDFromPath(op, args); id != "" {
			if _, has := d["id"]; !has {
				d["id"] = id
			}
		}
	}
	return json.Marshal(root)
}

// bodyIDFromPath returns the id path argument for PATCH operations on a
// single resource (path ending in /{id}), where JSON:API requires data.id
// to match the URL. Relationship endpoints ({id}/relationships/...) are
// excluded: their body ids name the related resources, not the path one.
func bodyIDFromPath(op *opSpec, args []string) string {
	if op.Method != "PATCH" || !strings.HasSuffix(op.Path, "/{id}") {
		return ""
	}
	for i, p := range op.PathParams {
		if p == "id" {
			return args[i]
		}
	}
	return ""
}

// convertBodyValue parses a flag value according to its schema type, so
// numbers and booleans reach the API as JSON numbers and booleans.
func convertBodyValue(typ, raw, flag string) (any, error) {
	switch typ {
	case "integer", "number":
		var n json.Number
		if err := json.Unmarshal([]byte(raw), &n); err != nil {
			return nil, fmt.Errorf("--%s: %q is not a valid %s", flag, raw, typ)
		}
		return n, nil
	case "boolean":
		switch raw {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return nil, fmt.Errorf("--%s: %q is not a valid boolean (true or false)", flag, raw)
	default:
		return raw, nil
	}
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
