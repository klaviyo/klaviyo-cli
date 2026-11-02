package cli

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestTUIArgs(t *testing.T) {
	for _, tt := range []struct {
		args, want []string
		requested  bool
	}{
		{[]string{"--tui", "profiles", "list"}, []string{"profiles", "list"}, true},
		{[]string{"profiles", "--tui", "get", "id", "--help"}, []string{"profiles", "get", "id", "--help"}, true},
		{[]string{"api", "--", "--tui"}, []string{"api", "--", "--tui"}, false},
		{[]string{"--tui", "api", "--", "--tui", ""}, []string{"api", "--", "--tui", ""}, true},
	} {
		got, requested := tuiArgs(tt.args)
		if !reflect.DeepEqual(got, tt.want) || requested != tt.requested {
			t.Fatalf("tuiArgs(%q) = %q, %v", tt.args, got, requested)
		}
	}
}

func TestCommandTree(t *testing.T) {
	root := newRootCmd()
	var checkCollisions func(*cobra.Command)
	checkCollisions = func(cmd *cobra.Command) {
		if cmd != root && cmd.LocalNonPersistentFlags().Lookup("tui") != nil {
			t.Fatalf("--tui collides with %s", cmd.CommandPath())
		}
		for _, child := range cmd.Commands() {
			checkCollisions(child)
		}
	}
	checkCollisions(root)
	tree := commandTree(root, []string{})
	var get *tuiNode
	var walk func(tuiNode)
	walk = func(node tuiNode) {
		names := map[string]bool{}
		for _, p := range node.Params {
			if names[p.Name] {
				t.Fatalf("duplicate field %s in %v", p.Name, node.Path)
			}
			names[p.Name] = true
		}
		if reflect.DeepEqual(node.Path, []string{"profiles", "get"}) {
			get = &node
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(tree)
	if get == nil {
		t.Fatal("generated profiles get missing from catalog")
	}
	if get.IsGroup || get.Help == "" {
		t.Fatalf("invalid command metadata: %+v", get)
	}
	params := map[string]tuiParam{}
	for _, p := range get.Params {
		params[p.Name] = p
	}
	if !params["id"].Required || params["id"].Kind != "argument" {
		t.Fatal("required path argument missing")
	}
	if params["account"].Kind != "option" || params["help"].Type != "bool" {
		t.Fatal("inherited/help flags missing")
	}
	if _, ok := params["tui"]; ok {
		t.Fatal("launch flag must not recurse")
	}
	for _, child := range tree.Children {
		if child.Name == "tui" {
			t.Fatal("management commands must not appear in browser")
		}
	}
}

func TestCatalogDoesNotRunHooks(t *testing.T) {
	root := &cobra.Command{Use: "test", PersistentPreRun: func(*cobra.Command, []string) { t.Fatal("hook ran") }}
	child := &cobra.Command{Use: "action <id>", Run: func(*cobra.Command, []string) { t.Fatal("command ran") }}
	child.Flags().StringArray("data", nil, "body values")
	if err := child.MarkFlagRequired("data"); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(child)
	tree := commandTree(root, []string{})
	for _, p := range tree.Children[0].Params {
		if p.Name == "data" && (!p.Multiple || !p.Required) {
			t.Fatalf("invalid repeatable required flag: %+v", p)
		}
	}
}
