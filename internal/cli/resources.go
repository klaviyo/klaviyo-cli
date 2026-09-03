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
// its CLI flag name (e.g. "page-size"), one-line help text, and whether the
// spec marks the parameter required (enforced client-side).
type queryParamSpec struct {
	Name     string
	Flag     string
	Help     string
	Required bool
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
	Beta        bool // beta API operation: lives under `klaviyo beta`, sends the beta revision
}

// maxPages bounds --paginate as a runaway-loop backstop.
const maxPages = 1000

// addResourceCmds registers one command group per spec tag, with one
// subcommand per generated operation (see resources_gen.go), plus a `beta`
// parent holding the beta API groups (see resources_beta_gen.go).
func addResourceCmds(root *cobra.Command) {
	addGroupCmds(root, resourceOps)
	if len(betaResourceOps) > 0 {
		beta := &cobra.Command{
			Use:   "beta",
			Short: "Beta API commands",
			Long: "Beta API commands. These call Klaviyo's beta APIs, sending the beta\n" +
				"revision header (" + api.DefaultBetaRevision + ") unless --revision overrides it.\n" +
				"Beta APIs can change or disappear between revisions.",
		}
		addGroupCmds(beta, betaResourceOps)
		root.AddCommand(beta)
	}
}

// addGroupCmds mounts ops onto parent, one command group per op group.
func addGroupCmds(parent *cobra.Command, ops []opSpec) {
	groups := map[string]*cobra.Command{}
	for i := range ops {
		op := &ops[i]
		group, ok := groups[op.Group]
		if !ok {
			group = &cobra.Command{
				Use:   op.Group,
				Short: fmt.Sprintf("Manage %s", strings.ReplaceAll(op.Group, "-", " ")),
			}
			groups[op.Group] = group
		}
		group.AddCommand(newResourceOpCmd(op))
	}
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		parent.AddCommand(groups[name])
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
		Args:  cobra.MatchAll(cobra.ExactArgs(len(op.PathParams)), nonEmptyArgs(op.PathParams)),
	}

	queryFlags := make([]*string, len(op.Query))
	for i, q := range op.Query {
		queryFlags[i] = cmd.Flags().String(q.Flag, "", q.Help)
		if q.Required {
			// Fail fast in cobra instead of burning an API round-trip on a
			// guaranteed 400. Only errors on a flag name cobra doesn't know,
			// which can't happen for a flag registered one line up.
			_ = cmd.MarkFlagRequired(q.Flag)
		}
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
		cmd.Flags().BoolVar(&paginate, "paginate", false, "follow cursor pagination and merge all pages' data and included (meta is per-page and dropped)")
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		path := resolvePath(op.Path, op.PathParams, args)
		query := url.Values{}
		for i, q := range op.Query {
			if *queryFlags[i] != "" {
				val := *queryFlags[i]
				if q.Name == "page[cursor]" || q.Name == "page_cursor" {
					val = normalizeCursor(q.Name, val)
				}
				query.Set(q.Name, val)
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
		// Beta APIs require a beta revision header; --revision still wins.
		if op.Beta && opts.revision == "" {
			client.Revision = api.DefaultBetaRevision
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
// OAS file by its revision branch. Beta docs pages exist only on the docs
// site's current version, so beta docs links stay unversioned; the beta OAS
// file still pins to the beta revision branch.
const (
	docsURLFmt     = "https://developers.klaviyo.com/en/v%s/reference/%s"
	betaDocsURLFmt = "https://developers.klaviyo.com/en/reference/%s"
	oasURLFmt      = "https://github.com/klaviyo/openapi/blob/%s/openapi/stable/apis/%s.json"
	betaOASURLFmt  = "https://github.com/klaviyo/openapi/blob/%s/openapi/beta/apis/%s.json"
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
	if op.OpID == "" {
		return b.String()
	}
	if op.Beta {
		fmt.Fprintf(&b, "\n\nAPI docs:     "+betaDocsURLFmt, op.OpID)
		fmt.Fprintf(&b, "\nOpenAPI file: "+betaOASURLFmt, api.DefaultBetaRevision, op.OpID)
	} else {
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
// numbers and booleans reach the API as JSON numbers and booleans, and
// json-typed flags (whole attributes that don't flatten into typed flags,
// like a segment definition) carry any raw JSON value.
func convertBodyValue(typ, raw, flag string) (any, error) {
	switch typ {
	case "json":
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, fmt.Errorf("--%s: invalid JSON: %v", flag, err)
		}
		return v, nil
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

// nonEmptyArgs rejects empty (or whitespace-only) positional path
// parameters: an empty id would otherwise collapse /api/lists/{id} onto the
// collection route and silently return the full list, exit 0 — a classic
// footgun when a failed earlier command left a shell variable empty.
func nonEmptyArgs(params []string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		for i, a := range args {
			if strings.TrimSpace(a) == "" {
				return fmt.Errorf("<%s> must not be empty", params[i])
			}
		}
		return nil
	}
}

// normalizeCursor lets a cursor flag accept the whole links.next URL a
// previous response returned, extracting the actual cursor from it — the
// same courtesy Klaviyo's SDKs extend. A bare cursor passes through
// untouched (real cursors never contain "://"). name is the endpoint's
// cursor parameter ("page[cursor]" on most endpoints, "page_cursor" on
// reporting), with the common spelling as fallback.
func normalizeCursor(name, raw string) string {
	if !strings.HasPrefix(raw, "https://") && !strings.HasPrefix(raw, "http://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if v := q.Get(name); v != "" {
		return v
	}
	if v := q.Get("page[cursor]"); v != "" {
		return v
	}
	return raw
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
	// Non-nil so an empty merge marshals as "data": [] like the API's own
	// empty pages, not "data": null.
	merged := []json.RawMessage{}
	included := []json.RawMessage{}
	hasIncluded := false // any page carried an included key, even empty
	seenIncluded := map[string]bool{}
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
			Data     []json.RawMessage `json:"data"`
			Included []json.RawMessage `json:"included"`
			Links    struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(resp.Body, &parsed); err != nil {
			return fmt.Errorf("parsing page %d: %w", page+1, err)
		}
		merged = append(merged, parsed.Data...)
		// Merge included too — the same related resource legitimately
		// appears on several pages, so dedupe by JSON:API identity. An
		// empty included array still marks the key as present, so the
		// merged document keeps the shape of the pages it merged.
		if parsed.Included != nil {
			hasIncluded = true
		}
		for _, item := range parsed.Included {
			var ref struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}
			if json.Unmarshal(item, &ref) == nil && ref.Type != "" && ref.ID != "" {
				k := ref.Type + "/" + ref.ID
				if seenIncluded[k] {
					continue
				}
				seenIncluded[k] = true
			}
			included = append(included, item)
		}
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
	doc := map[string]any{"data": merged}
	if hasIncluded {
		doc["included"] = included
	}
	if nextLink != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: stopped after %d pages with more results remaining\n", maxPages)
		// Carry the last page's next link into the merged document, JSON:API
		// style: its presence marks the merge as incomplete, and its
		// page[cursor] is where a follow-up run resumes (--page-cursor).
		// Complete merges keep the plain {"data": [...]} shape.
		doc["links"] = map[string]string{"next": nextLink}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return printResponse(cmd, &api.Response{StatusCode: 200, Body: out})
}
