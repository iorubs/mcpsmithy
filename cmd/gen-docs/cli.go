package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/alecthomas/kong"
	"github.com/operator-assistant/mcpsmithy/internal/commands"
)

// CommandData holds a Kong node with pre-filtered flags, args, and subcommands
// for template rendering. The embedded *kong.Node provides Name, Help, Parent, etc.
type CommandData struct {
	*kong.Node
	Flags       []*kong.Flag
	Args        []*kong.Value
	Subcommands []CommandData
}

// CLIData is passed to the CLI reference template.
type CLIData struct {
	Name     string
	Help     string
	Flags    []*kong.Flag
	Commands []CommandData
}

var cliOutDir = filepath.Join("docs", "user", "reference", "server")

// generateCLIDocs generates a single CLI reference page from the Kong parser model.
func generateCLIDocs() error {
	var cli commands.CLI
	parser, err := kong.New(&cli,
		kong.Description("Project-agnostic MCP tool server. Reads .mcpsmithy.yaml and serves MCP tools over stdio."),
	)
	if err != nil {
		return fmt.Errorf("creating Kong parser: %w", err)
	}

	if err := os.MkdirAll(cliOutDir, 0o755); err != nil {
		return err
	}

	var cmds []CommandData
	for _, node := range parser.Model.Children {
		cmds = append(cmds, buildCommandData(node))
	}

	funcs := template.FuncMap{
		"flagType": func(f *kong.Flag) string {
			return f.Value.Target.Type().Kind().String()
		},
	}
	content, err := renderTemplate(filepath.Join("cli", "readme.md.tmpl"), funcs, CLIData{
		Name:     "mcpsmithy",
		Help:     parser.Model.Help,
		Flags:    parser.Model.Flags,
		Commands: cmds,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cliOutDir, "README.md"), []byte(content), 0o644)
}

// buildCommandData builds the template-ready CommandData for one Kong node.
func buildCommandData(node *kong.Node) CommandData {
	var subs []CommandData
	for _, child := range node.Children {
		if child.Type != kong.CommandNode {
			continue
		}
		subs = append(subs, CommandData{
			Node:  child,
			Flags: child.Flags,
			Args:  child.Positional,
		})
	}
	return CommandData{
		Node:        node,
		Args:        node.Positional,
		Flags:       node.Flags,
		Subcommands: subs,
	}
}
