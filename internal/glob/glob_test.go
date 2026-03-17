package glob_test

import (
	"testing"
	"testing/fstest"

	"github.com/operator-assistant/mcpsmithy/internal/glob"
)

func TestToRegexp(t *testing.T) {
	tests := []struct {
		pattern string
		match   string
		want    bool
	}{
		{"**/*.md", "docs/index.md", true},
		{"**/*.md", "README.md", false}, // no directory prefix
		{"docs/**/*.md", "docs/api/methods.md", true},
		{"docs/**/*.md", "docs/index.md", false}, // no extra level
		{"**/*.go", "internal/config.go", true},
		{"**/*.go", "internal/config.md", false},
		{"src/foo", "src/foo", true},
		{"src/foo", "src/bar", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.match, func(t *testing.T) {
			re := glob.ToRegexp(tt.pattern)
			if got := re.MatchString(tt.match); got != tt.want {
				t.Errorf("ToRegexp(%q).MatchString(%q) = %v; want %v", tt.pattern, tt.match, got, tt.want)
			}
		})
	}
}

func TestWalkFS(t *testing.T) {
	fsys := fstest.MapFS{
		"README.md":           &fstest.MapFile{Data: []byte("# Root")},
		"docs/index.md":       &fstest.MapFile{Data: []byte("# Docs")},
		"docs/api/methods.md": &fstest.MapFile{Data: []byte("# API")},
		"internal/config.go":  &fstest.MapFile{Data: []byte("package config")},
	}

	tests := []struct {
		name      string
		pattern   string
		wantCount int
	}{
		{"all nested md files", "**/*.md", 2},
		{"docs deep subtree", "docs/**/*.md", 1},
		{"go files", "**/*.go", 1},
		{"no match", "**/*.yaml", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := glob.WalkFS(fsys, tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("WalkFS(%q) returned %d files; want %d: %v", tt.pattern, len(got), tt.wantCount, got)
			}
		})
	}
}
