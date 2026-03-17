// Command gen-docs generates Markdown reference documentation for
// mcpsmithy — both CLI (from the Kong command tree) and config
// schema (from Go struct types and comments via go/ast).
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// tmplDir is the path to the template directory, relative to the repo root.
// gen-docs is always run from the repo root, so this is stable.
var tmplDir = filepath.Join("cmd", "gen-docs", "templates")

func main() {
	if err := generateCLIDocs(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating CLI docs: %v\n", err)
		os.Exit(1)
	}

	if err := generateConfigDocs(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating config docs: %v\n", err)
		os.Exit(1)
	}
}

// renderTemplate parses the named template file under tmplDir and executes
// it against data, returning the rendered string. Callers supply their own
// template functions via funcs (may be nil).
func renderTemplate(name string, funcs template.FuncMap, data any) (string, error) {
	path := filepath.Join(tmplDir, name)
	tmpl, err := template.New(filepath.Base(path)).Funcs(funcs).ParseFiles(path)
	if err != nil {
		return "", fmt.Errorf("parsing template %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", path, err)
	}
	return buf.String(), nil
}
