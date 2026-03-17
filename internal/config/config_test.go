package config

import (
	"strings"
	"testing"

	"github.com/operator-assistant/mcpsmithy/internal/config/schema"
)

// TestDocsComplete verifies every type, field, and enum value reachable from
// Config has a doc comment. Fails fast when someone adds a field without docs.
func TestDocsComplete(t *testing.T) {
	doc := schema.Describe(Config{}, "1", schema.ParseTypeDocs(TypesSources...))

	for _, s := range doc.Structs {
		if s.Doc == "" {
			t.Errorf("struct %s: missing doc comment", s.Name)
		}
		for _, f := range s.Fields {
			if f.Description == "—" {
				t.Errorf("struct %s, field %s: missing doc comment", s.Name, f.YAMLName)
			}
		}
	}
	for _, e := range doc.Enums {
		if e.Doc == "" {
			t.Errorf("enum %s: missing doc comment", e.Name)
		}
		for _, v := range e.Values {
			if v.Doc == "—" {
				t.Errorf("enum %s, value %q: missing doc comment", e.Name, v.Label)
			}
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "routes version 1",
			yaml: `
version: "1"
project:
  name: test
  description: test project
conventions:
  c:
    scope: "*"
    description: d
tools:
  t:
    description: d
    template: hello
`,
		},
		{
			name:    "unsupported version",
			yaml:    `version: "99"`,
			wantErr: `unsupported config version "99"`,
		},
		{
			name:    "malformed YAML",
			yaml:    `{bad yaml`,
			wantErr: "parsing YAML",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
