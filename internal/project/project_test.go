package project_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/project"
	"github.com/operator-assistant/mcpsmithy/internal/project/sources"
)

// writeFixture writes files into dir for use as local source content.
func writeFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fakeSource is a Source that returns canned docs; Fetch is a no-op.
type fakeSource struct{ docs []sources.RawDoc }

func (s fakeSource) Fetch(context.Context) error { return nil }
func (s fakeSource) Read([]string, string) ([]sources.RawDoc, error) {
	return s.docs, nil
}

// errorSource is a Source whose Fetch always fails.
type errorSource struct{}

func (errorSource) Fetch(context.Context) error                     { return errors.New("boom") }
func (errorSource) Read([]string, string) ([]sources.RawDoc, error) { return nil, nil }

func TestBuild(t *testing.T) {
	t.Run("nil sources", func(t *testing.T) {
		cfg := &config.Config{}
		result, ready := project.Build(t.Context(), cfg, t.TempDir(), project.BuildOptions{})
		<-ready
		if result == nil {
			t.Fatal("expected non-nil Searcher")
		}
		if result.Len() != 0 {
			t.Fatalf("expected 0 chunks, got %d", result.Len())
		}
	})

	t.Run("empty sources", func(t *testing.T) {
		cfg := &config.Config{Project: config.Project{Sources: &config.ProjectSources{}}}
		result, ready := project.Build(t.Context(), cfg, t.TempDir(), project.BuildOptions{})
		<-ready
		if result.Len() != 0 {
			t.Fatalf("expected 0 chunks, got %d", result.Len())
		}
	})

	t.Run("local source indexes docs", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, map[string]string{"doc.md": "# Hello\nworld"})

		cfg := &config.Config{Project: config.Project{Sources: &config.ProjectSources{
			Local: map[string]config.LocalSource{"docs": {Paths: []string{"*.md"}}},
		}}}
		result, ready := project.Build(t.Context(), cfg, dir, project.BuildOptions{})
		<-ready
		if result.Len() != 1 {
			t.Fatalf("expected 1 chunk, got %d", result.Len())
		}
	})

	t.Run("remote source fetch then index", func(t *testing.T) {
		// Create a registry with a fake "git" factory backed by in-memory FS.
		reg := &sources.Registry{}
		reg.Register("git", func(name string, raw any, _, _ string, _ config.PullPolicy) (sources.Source, sources.SourceMeta, error) {
			src := raw.(config.GitSource)
			return fakeSource{docs: []sources.RawDoc{
					{Source: src.Repo + ":doc.md", Content: "# Hello\nworld"},
				}}, sources.SourceMeta{
					ReadGlobs:  []string{"*.md"},
					ReadPrefix: src.Repo,
				}, nil
		})

		cfg := &config.Config{Project: config.Project{Sources: &config.ProjectSources{
			Git: map[string]config.GitSource{"repo": {Repo: "https://github.com/example/repo"}},
		}}}
		result, ready := project.Build(t.Context(), cfg, t.TempDir(), project.BuildOptions{Registry: reg})
		<-ready
		if result.Len() != 1 {
			t.Fatalf("expected 1 chunk, got %d", result.Len())
		}
	})

	t.Run("fetch error produces no chunks", func(t *testing.T) {
		reg := &sources.Registry{}
		reg.Register("git", func(name string, raw any, _, _ string, _ config.PullPolicy) (sources.Source, sources.SourceMeta, error) {
			return errorSource{}, sources.SourceMeta{ReadGlobs: []string{"*.md"}}, nil
		})

		cfg := &config.Config{Project: config.Project{Sources: &config.ProjectSources{
			Git: map[string]config.GitSource{"repo": {Repo: "https://github.com/example/repo"}},
		}}}
		result, ready := project.Build(t.Context(), cfg, t.TempDir(), project.BuildOptions{Registry: reg})
		<-ready
		if result.Len() != 0 {
			t.Fatalf("expected 0 chunks, got %d", result.Len())
		}
	})

	t.Run("noIndex source produces no chunks", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, map[string]string{"doc.md": "# Hello\nworld"})

		falseVal := false
		cfg := &config.Config{Project: config.Project{Sources: &config.ProjectSources{
			Local: map[string]config.LocalSource{"docs": {Paths: []string{"*.md"}, Index: &falseVal}},
		}}}
		result, ready := project.Build(t.Context(), cfg, dir, project.BuildOptions{})
		<-ready
		if result.Len() != 0 {
			t.Fatalf("expected 0 chunks, got %d", result.Len())
		}
	})

	t.Run("skipIndex skips indexing", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, map[string]string{"doc.md": "# Hello\nworld"})

		cfg := &config.Config{Project: config.Project{Sources: &config.ProjectSources{
			Local: map[string]config.LocalSource{"docs": {Paths: []string{"*.md"}}},
		}}}
		result, ready := project.Build(t.Context(), cfg, dir, project.BuildOptions{SkipIndex: true})
		<-ready
		if result.Len() != 0 {
			t.Fatalf("expected 0 chunks, got %d", result.Len())
		}
	})

	t.Run("multiple sources all indexed", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, map[string]string{
			"a/doc.md": "# A\nworld",
			"b/doc.md": "# B\nworld",
		})

		cfg := &config.Config{Project: config.Project{Sources: &config.ProjectSources{
			Local: map[string]config.LocalSource{
				"a": {Paths: []string{"a/*.md"}},
				"b": {Paths: []string{"b/*.md"}},
			},
		}}}
		result, ready := project.Build(t.Context(), cfg, dir, project.BuildOptions{})
		<-ready
		if result.Len() != 2 {
			t.Fatalf("expected 2 chunks, got %d", result.Len())
		}
	})

	t.Run("sync interval with no sources", func(t *testing.T) {
		cfg := &config.Config{
			Project: config.Project{
				Sources: &config.ProjectSources{SyncInterval: 5, PullPolicy: config.PullPolicyNever},
			},
		}
		result, ready := project.Build(t.Context(), cfg, t.TempDir(), project.BuildOptions{})
		<-ready
		if result.Len() != 0 {
			t.Fatalf("expected 0 chunks, got %d", result.Len())
		}
	})
}
