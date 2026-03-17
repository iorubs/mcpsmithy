package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/config/schema"
)

// ConfigIndexData is passed to the config README template.
type ConfigIndexData struct {
	Versions []ConfigIndexEntry
}

// ConfigIndexEntry is a single version row in the config README.
type ConfigIndexEntry struct {
	Label   string
	Version string
}

var configOutDir = filepath.Join("docs", "user", "reference", "config")

// generateConfigDocs generates per-version config schema docs by walking
// config struct types via reflection and mcpsmithy struct tags. Each
// version gets its own markdown file and an index README links them all
func generateConfigDocs() error {
	if err := os.MkdirAll(configOutDir, 0o755); err != nil {
		return err
	}

	var indexEntries []ConfigIndexEntry

	for version, vs := range config.Versions {
		doc := schema.Describe(vs.RootType(), version, schema.ParseTypeDocs(vs.TypesSources()...))
		versionFuncs := template.FuncMap{
			"anchor": func(name string) string {
				return strings.ToLower(name)
			},
			"br": func(s string) string {
				return strings.ReplaceAll(s, "\n", "<br />")
			},
		}
		content, err := renderTemplate(filepath.Join("config", "version.md.tmpl"), versionFuncs, doc)
		if err != nil {
			return fmt.Errorf("rendering v%s: %w", version, err)
		}

		outFile := filepath.Join(configOutDir, fmt.Sprintf("v%s.md", version))
		if err := os.WriteFile(outFile, []byte(content), 0o644); err != nil {
			return err
		}

		label := fmt.Sprintf("Version %s", version)
		indexEntries = append(indexEntries, ConfigIndexEntry{Label: label, Version: version})
	}

	indexFuncs := template.FuncMap{
		"readSection": func(name string) (string, error) {
			data, err := os.ReadFile(filepath.Join("internal", "setup", "sections", fmt.Sprintf("%s.md", name)))
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	}
	indexContent, err := renderTemplate(filepath.Join("config", "readme.md.tmpl"), indexFuncs, ConfigIndexData{
		Versions: indexEntries,
	})
	if err != nil {
		return err
	}
	indexFile := filepath.Join(configOutDir, "README.md")
	if err := os.WriteFile(indexFile, []byte(indexContent), 0o644); err != nil {
		return err
	}

	return nil
}
