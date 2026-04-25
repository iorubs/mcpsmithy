package api

import (
	"os"
	"path/filepath"
	"testing"
)

// validConfig is the smallest YAML that passes v1 validation.
const validConfig = `version: "1"
project:
  name: test
  description: a test project
conventions:
  style:
    description: code style
tools:
  info:
    description: shows info
    template: "hello"
`

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{"valid", validConfig, false},
		{"unsupported version", `version: "99"`, true},
		{"invalid yaml", `{{{`, true},
		{"file not found", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfgPath string
			if tt.yaml != "" {
				dir := t.TempDir()
				cfgPath = filepath.Join(dir, ".mcpsmithy.yaml")
				if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				cfgPath = "/nonexistent/path/.mcpsmithy.yaml"
			}

			cfg, root, err := LoadConfig(cfgPath)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if root == "" {
				t.Fatal("expected non-empty root")
			}
		})
	}
}
