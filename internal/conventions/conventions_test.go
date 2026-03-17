package conventions

import (
	"context"
	"testing"

	"github.com/operator-assistant/mcpsmithy/internal/config"
)

func TestScopeTags(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		want  []string
	}{
		{"path segments", "internal/controller/**", []string{"internal", "controller"}},
		{"wildcard only", "*", nil},
		{"single segment", "docs/**", []string{"docs"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scopeTags(tt.scope)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("tag[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildIndex(t *testing.T) {
	convs := map[string]config.Convention{
		"style":    {Scope: "*", Description: "Code style rules", Docs: []config.DocRef{{Source: "docs", Paths: []string{"docs/style.md"}}}},
		"no-scope": {Description: "Search only — no scope"},
		"api":      {Scope: "api/**", Description: "API conventions", Tags: []string{"api"}},
		"tooling":  {Scope: "internal/tools/**", Description: "Tool engine conventions"},
	}

	idx := BuildIndex(context.Background(), convs)
	if idx.Len() != 4 {
		t.Fatalf("expected 4 convention chunks, got %d", idx.Len())
	}

	results := idx.Search("api", 10)
	if len(results) == 0 {
		t.Fatal("expected results for 'api'")
	}
	if results[0].Chunk.ConventionID != "api" {
		t.Fatalf("expected 'api' convention first, got %q", results[0].Chunk.ConventionID)
	}
	for _, r := range results {
		if r.Chunk.ConventionID == "" {
			t.Fatal("all chunks in convention index should have ConventionID")
		}
	}
}

func TestForPath(t *testing.T) {
	convs := map[string]config.Convention{
		"global":      {Scope: "*", Description: "Universal"},
		"controllers": {Scope: "internal/controller/**", Description: "Controllers"},
		"search-only": {Description: "No scope — search only"},
		"api":         {Scope: "api/**", Description: "API types"},
	}

	hasDesc := func(cs []config.Convention, desc string) bool {
		for _, c := range cs {
			if c.Description == desc {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name     string
		path     string
		wantAll  []string
		wantNone []string
	}{
		{
			"controller path matches global and controllers, not api or search-only",
			"internal/controller/foo.go",
			[]string{"Universal", "Controllers"},
			[]string{"API types", "No scope — search only"},
		},
		{
			"api path matches global and api, not controllers",
			"api/types.go",
			[]string{"Universal", "API types"},
			[]string{"Controllers"},
		},
		{
			"unmatched path only matches global",
			"cmd/main.go",
			[]string{"Universal"},
			[]string{"Controllers", "API types"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ForPath(convs, tt.path)
			for _, want := range tt.wantAll {
				if !hasDesc(result, want) {
					t.Errorf("ForPath(%q): expected convention with description %q", tt.path, want)
				}
			}
			for _, notWant := range tt.wantNone {
				if hasDesc(result, notWant) {
					t.Errorf("ForPath(%q): unexpected convention with description %q", tt.path, notWant)
				}
			}
		})
	}
}
