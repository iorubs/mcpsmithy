// Package setup implements the config-authoring MCP engine.
// It exposes exactly two tools (config_guide and config_section) and
// does not require a .mcpsmithy.yaml file.
package setup

import (
	"context"
	"embed"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/config/schema"
)

//go:embed sections/*.md
var sectionsFS embed.FS

const (
	toolConfigGuide   = "config_guide"
	toolConfigSection = "config_section"
)

// validSections lists the section names accepted by config_section.
var validSections = []string{"project", "conventions", "tools"}

// sectionTypes maps each config_section name to the schema type names
// that should be included in the field reference appended to the guide.
var sectionTypes = map[string]map[string]bool{
	"project":     {"Project": true, "ProjectSources": true, "LocalSource": true, "ScrapeSource": true, "GitSource": true, "HTTPSource": true, "PullPolicy": true},
	"conventions": {"Convention": true, "DocRef": true, "ConventionRelations": true},
	"tools":       {"Tool": true, "ToolParam": true, "ParamType": true, "ParamConstraints": true, "TemplateString": true, "BuiltinFunc": true},
}

// Engine implements server.Engine for setup mode.
type Engine struct {
	tools map[string]config.Tool
}

// New creates a setup Engine.
func New() *Engine {
	sectionParam := config.ToolParam{
		Name:        "section",
		Type:        config.ParamTypeString,
		Required:    true,
		Description: "Config section: project, conventions, or tools",
	}

	return &Engine{tools: map[string]config.Tool{
		toolConfigGuide: {
			Description: "Returns an overview of the .mcpsmithy.yaml config structure: " +
				"the four top-level keys, how they relate, and an annotated minimal example. " +
				"Call this first in any setup session.",
		},
		toolConfigSection: {
			Description: "Returns the full field reference for one config section. " +
				"Use after config_guide when writing or improving a specific section.",
			Params: []config.ToolParam{sectionParam},
		},
	}}
}

// Tools returns the two setup tool definitions.
func (e *Engine) Tools() map[string]config.Tool { return e.tools }

// Execute dispatches to the guide or section handler.
func (e *Engine) Execute(_ context.Context, name string, params map[string]any) (string, error) {
	switch name {
	case toolConfigGuide:
		return readSection("guide")
	case toolConfigSection:
		section, _ := params["section"].(string)
		if section == "" {
			return "", fmt.Errorf("missing required parameter: %q", "section")
		}
		section = strings.ToLower(section)
		var valid bool
		if slices.Contains(validSections, section) {
			valid = true
		}
		if !valid {
			return "", fmt.Errorf("unknown section: %q (valid: %s)", section, strings.Join(validSections, ", "))
		}
		guide, err := readSection(section)
		if err != nil {
			return "", err
		}

		// Append the field reference for this section's types.
		doc := schema.Describe(config.Config{}, "1", schema.ParseTypeDocs(config.TypesSources...))
		filtered := schema.FilterTypes(doc, sectionTypes[section])
		var b strings.Builder
		if err := fieldRefTmpl.Execute(&b, filtered); err == nil {
			if ref := b.String(); ref != "" {
				guide += "\n# Field Reference\n" + ref
			}
		}
		return guide, nil
	default:
		return "", fmt.Errorf("unknown tool: %q", name)
	}
}

func readSection(name string) (string, error) {
	data, err := sectionsFS.ReadFile("sections/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("unknown section: %q (valid: %s)", name, strings.Join(validSections, ", "))
	}
	return string(data), nil
}

// fieldRefTmpl produces a plain-text field reference from a SchemaDoc.
// The output is structured for LLM consumption — no fancy markdown tables,
// just clear headings and field listings.
var fieldRefTmpl = template.Must(template.New("fieldref").Parse(`
{{- range .Structs}}

## {{.Name}}
{{- if .Doc}}
{{.Doc}}
{{- end}}

Fields:
{{- range .Fields}}
- {{.YAMLName}} ({{.Type}}){{if eq .Required "yes"}} [required]{{end}}{{if and .Default (ne .Default "—")}} default={{.Default}}{{end}}{{if and .Description (ne .Description "—")}} — {{.Description}}{{end}}{{if .Min}} (min: {{.Min}}){{end}}{{if .OneOfGroups}} [mutually exclusive with {{range $i, $g := .OneOfGroups}}{{range $j, $p := $g.Peers}}{{if or $i $j}}, {{end}}{{$p}}{{end}}{{end}}]{{end}}
{{- end}}
{{- end}}
{{- range .Enums}}

## {{.Name}}
{{- if .Doc}}
{{.Doc}}
{{- end}}

Values:
{{- range .Values}}
- {{.Label}}{{if and .Doc (ne .Doc "—")}} — {{.Doc}}{{end}}
{{- end}}
{{- end}}
{{- range .Types}}

## {{.Name}}
{{- if .Doc}}
{{.Doc}}
{{- end}}
{{- end}}
`))
