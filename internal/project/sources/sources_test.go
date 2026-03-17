package sources

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/operator-assistant/mcpsmithy/internal/config"
)

func TestSkipFetch(t *testing.T) {
	existingDir := t.TempDir()
	nonExistentDir := filepath.Join(t.TempDir(), "does-not-exist")

	tests := []struct {
		name    string
		policy  config.PullPolicy
		destDir string
		want    bool
	}{
		{"always, dir exists", config.PullPolicyAlways, existingDir, false},
		{"always, dir missing", config.PullPolicyAlways, nonExistentDir, false},
		{"never, dir exists", config.PullPolicyNever, existingDir, true},
		{"never, dir missing", config.PullPolicyNever, nonExistentDir, true},
		{"ifNotPresent, dir exists", config.PullPolicyIfNotPresent, existingDir, true},
		{"ifNotPresent, dir missing", config.PullPolicyIfNotPresent, nonExistentDir, false},
		{"empty defaults to ifNotPresent, dir exists", "", existingDir, true},
		{"empty defaults to ifNotPresent, dir missing", "", nonExistentDir, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skipFetch(tt.policy, tt.destDir)
			if got != tt.want {
				t.Errorf("skipFetch(%q, %q) = %v; want %v", tt.policy, tt.destDir, got, tt.want)
			}
		})
	}
}

func TestResolvePolicy(t *testing.T) {
	tests := []struct {
		name      string
		perSource config.PullPolicy
		global    config.PullPolicy
		want      config.PullPolicy
	}{
		{"per-source wins over global", config.PullPolicyAlways, config.PullPolicyNever, config.PullPolicyAlways},
		{"empty per-source falls back to global", "", config.PullPolicyNever, config.PullPolicyNever},
		{"per-source never overrides always global", config.PullPolicyNever, config.PullPolicyAlways, config.PullPolicyNever},
		{"both empty returns empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePolicy(tt.perSource, tt.global)
			if got != tt.want {
				t.Errorf("resolvePolicy(%q, %q) = %q; want %q", tt.perSource, tt.global, got, tt.want)
			}
		})
	}
}

func TestReadFS(t *testing.T) {
	fsys := fstest.MapFS{
		"guide.md":     &fstest.MapFile{Data: []byte("# Guide")},
		"reference.md": &fstest.MapFile{Data: []byte("# Reference")},
		"internal.go":  &fstest.MapFile{Data: []byte("package main")},
	}

	tests := []struct {
		name      string
		globs     []string
		prefix    string
		wantCount int
		wantErr   bool
	}{
		{"single glob", []string{"*.md"}, "", 2, false},
		{"prefix set", []string{"*.md"}, "https://example.com", 2, false},
		{"no match", []string{"*.yaml"}, "", 0, false},
		{"multiple globs", []string{"*.md", "*.go"}, "", 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docs, err := readFS(fsys, tt.globs, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Fatalf("readFS() error = %v; wantErr %v", err, tt.wantErr)
			}
			if len(docs) != tt.wantCount {
				t.Errorf("readFS() returned %d docs; want %d", len(docs), tt.wantCount)
			}
			if tt.prefix != "" {
				for _, d := range docs {
					if len(d.Source) == 0 || d.Source[:len(tt.prefix)] != tt.prefix {
						t.Errorf("expected Source to start with prefix %q, got %q", tt.prefix, d.Source)
					}
				}
			}
		})
	}
}
