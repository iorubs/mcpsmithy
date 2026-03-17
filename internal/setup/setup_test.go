package setup

import (
	"context"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	eng := New()
	tools := eng.Tools()

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if _, ok := tools[toolConfigGuide]; !ok {
		t.Error("missing config_guide tool")
	}
	if _, ok := tools[toolConfigSection]; !ok {
		t.Error("missing config_section tool")
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name         string
		tool         string
		params       map[string]any
		wantErr      bool
		wantContains []string
	}{
		// config_guide
		{"guide returns overview", toolConfigGuide, nil, false, []string{"version"}},

		// config_section — valid
		{"section project", toolConfigSection, map[string]any{"section": "project"}, false, []string{"project", "Field Reference"}},
		{"section conventions", toolConfigSection, map[string]any{"section": "conventions"}, false, []string{"convention"}},
		{"section tools", toolConfigSection, map[string]any{"section": "tools"}, false, []string{"tool"}},
		{"case insensitive upper", toolConfigSection, map[string]any{"section": "PROJECT"}, false, []string{"project"}},
		{"case insensitive mixed", toolConfigSection, map[string]any{"section": "Tools"}, false, []string{"tool"}},

		// config_section — errors
		{"unknown section", toolConfigSection, map[string]any{"section": "banana"}, true, nil},
		{"empty section key", toolConfigSection, map[string]any{}, true, nil},
		{"nil params", toolConfigSection, nil, true, []string{"section"}},

		// unknown tool
		{"unknown tool", "nonexistent", nil, true, []string{"nonexistent"}},
	}
	eng := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := eng.Execute(context.Background(), tt.tool, tt.params)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				for _, s := range tt.wantContains {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("error should mention %q, got: %v", s, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out == "" {
				t.Fatal("expected non-empty output")
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(out, s) {
					t.Errorf("output missing %q", s)
				}
			}
		})
	}
}

func TestSectionsHaveContent(t *testing.T) {
	sections := append([]string{"guide"}, validSections...)
	for _, name := range sections {
		t.Run(name, func(t *testing.T) {
			content, err := readSection(name)
			if err != nil {
				t.Fatalf("failed to read section %q: %v", name, err)
			}
			if len(content) < 100 {
				t.Errorf("section %q seems too short (%d bytes)", name, len(content))
			}
		})
	}
}
