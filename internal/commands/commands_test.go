package commands

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectRoot(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".mcpsmithy.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}

	tests := []struct {
		name    string
		config  string
		want    string
		wantErr bool
	}{
		{"with config", cfgPath, dir, false},
		{"fallback to cwd", "nonexistent-file.yaml", cwd, false},
		{"config is directory", dir, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &CLI{Config: tt.config}
			root, err := cli.ProjectRoot()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			want, err := filepath.EvalSymlinks(tt.want)
			if err != nil {
				t.Fatalf("EvalSymlinks(%q): %v", tt.want, err)
			}
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatalf("EvalSymlinks(%q): %v", root, err)
			}
			if root != want {
				t.Errorf("got %q, want %q", root, want)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   LogLevel
		wantLvl slog.Level
	}{
		{"debug", LogLevelDebug, slog.LevelDebug},
		{"info", LogLevelInfo, slog.LevelInfo},
		{"warn", LogLevelWarn, slog.LevelWarn},
		{"error", LogLevelError, slog.LevelError},
		{"default", "", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLogLevel(tt.level); got != tt.wantLvl {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.level, got, tt.wantLvl)
			}
		})
	}
}

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

			cli := &CLI{Config: cfgPath}
			cfg, root, err := cli.LoadConfig()

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
