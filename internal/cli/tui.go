package cli

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/klaviyo/klaviyo-cli/internal/tui"
)

func newTUICmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tui", Short: "Set up the optional interactive command browser"}
	cmd.AddCommand(&cobra.Command{
		Use: "setup", Short: "Install bundled, pinned OpenTUI dependencies (requires Bun >= 1.3.14)",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return tui.Setup() },
	})
	cmd.AddCommand(&cobra.Command{
		Use: "catalog", Hidden: true, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(commandTree(cmd.Root(), []string{}))
		},
	})
	return cmd
}

type tuiParam struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Opts      []string `json:"opts"`
	Secondary []string `json:"secondary"`
	Type      string   `json:"type"`
	Required  bool     `json:"required"`
	Multiple  bool     `json:"multiple"`
	Nargs     int      `json:"nargs"`
	Default   string   `json:"default"`
	IsFlag    bool     `json:"is_flag"`
	Help      string   `json:"help"`
	Metavar   string   `json:"metavar"`
}

type tuiNode struct {
	Name       string     `json:"name"`
	Path       []string   `json:"path"`
	Aliases    []string   `json:"aliases"`
	IsGroup    bool       `json:"isGroup"`
	Help       string     `json:"help"`
	Short      string     `json:"short"`
	Deprecated bool       `json:"deprecated"`
	Params     []tuiParam `json:"params"`
	Children   []tuiNode  `json:"children"`
}

// Read metadata only: no argument validation, authentication or command hooks run.
func commandTree(cmd *cobra.Command, path []string) tuiNode {
	cmd.InitDefaultHelpFlag()
	node := tuiNode{
		Name: cmd.Name(), Path: path, Aliases: cmd.Aliases, IsGroup: !cmd.Runnable(),
		Help: cmd.Long, Short: cmd.Short, Deprecated: cmd.Deprecated != "",
		Params: []tuiParam{}, Children: []tuiNode{},
	}
	for _, word := range strings.Fields(cmd.Use)[1:] {
		if !strings.HasPrefix(word, "<") && !strings.HasPrefix(word, "[") {
			continue
		}
		name := strings.Trim(word, "<>[].")
		nargs := 1
		if strings.Contains(word, "...") {
			nargs = -1
		}
		node.Params = append(node.Params, tuiParam{
			Kind: "argument", Name: name, Metavar: name, Type: "string", Nargs: nargs,
			Required: strings.HasPrefix(word, "<"), Opts: []string{}, Secondary: []string{},
		})
	}
	flags := pflag.NewFlagSet("catalog", pflag.ContinueOnError)
	flags.AddFlagSet(cmd.LocalFlags())
	flags.AddFlagSet(cmd.InheritedFlags())
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "tui" {
			return
		}
		// An argument and an inherited flag may share a name (auth logout account).
		for i := range node.Params {
			if node.Params[i].Kind == "argument" && node.Params[i].Name == f.Name {
				node.Params[i].Name = "arg:" + f.Name
			}
		}
		opts := []string{"--" + f.Name}
		if f.Shorthand != "" {
			opts = append(opts, "-"+f.Shorthand)
		}
		node.Params = append(node.Params, tuiParam{
			Kind: "option", Name: f.Name, Opts: opts, Secondary: []string{}, Type: f.Value.Type(),
			Required: len(f.Annotations[cobra.BashCompOneRequiredFlag]) > 0,
			Multiple: f.Value.Type() == "stringArray" || f.Value.Type() == "stringSlice",
			Nargs:    1, Default: f.DefValue, IsFlag: f.Value.Type() == "bool", Help: f.Usage,
		})
	})
	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "tui" || child.Name() == "completion" || child.Name() == "help" {
			continue
		}
		node.Children = append(node.Children, commandTree(child, append(append([]string{}, path...), child.Name())))
	}
	return node
}

func tuiArgs(args []string) ([]string, bool) {
	result := make([]string, 0, len(args))
	requested := false
	for i, arg := range args {
		if arg == "--" {
			return append(result, args[i:]...), requested
		}
		if arg == "--tui" {
			requested = true
		} else {
			result = append(result, arg)
		}
	}
	return result, requested
}
