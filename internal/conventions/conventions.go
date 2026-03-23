// Package conventions matches file paths to project documentation
// and provides browsing of convention definitions.
package conventions

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/glob"
	"github.com/operator-assistant/mcpsmithy/internal/search"
)

// sourcesDir is the directory under the project root where fetched sources are stored.
const sourcesDir = ".mcpsmithy"

// BuildIndex creates a search index from the conventions map.
// Conventions are indexed independently so they are always surfaced
// prominently in search results, regardless of BM25 scores from source docs.
func BuildIndex(ctx context.Context, cfg *config.Config) search.Searcher {
	var srcs *config.ProjectSources
	if cfg != nil {
		srcs = cfg.Project.Sources
	}
	prefixes := sourcePrefixes(srcs)

	var conventions map[string]config.Convention
	if cfg != nil {
		conventions = cfg.Conventions
	}

	var chunks []search.Chunk
	for name, c := range conventions {
		var docStrings []string
		for _, d := range c.Docs {
			prefix := prefixes[d.Source]
			if len(d.Paths) > 0 {
				for _, p := range d.Paths {
					if prefix != "" {
						docStrings = append(docStrings, filepath.Join(prefix, p))
					} else {
						docStrings = append(docStrings, p)
					}
				}
			} else {
				if prefix != "" {
					docStrings = append(docStrings, prefix)
				} else {
					docStrings = append(docStrings, d.Source)
				}
			}
		}
		chunks = append(chunks, search.Chunk{
			Source:       c.Scope,
			Title:        name,
			Body:         c.Description,
			Tags:         append(c.Tags, scopeTags(c.Scope)...),
			ConventionID: name,
			Docs:         docStrings,
		})
	}
	idx := search.NewIndex(chunks)
	slog.InfoContext(ctx, "convention index built", "chunks", idx.Len())
	return idx
}

// sourcePrefixes builds a map of source name -> cache path prefix for
// non-local sources (git, http, scrape). Local sources have no prefix.
func sourcePrefixes(srcs *config.ProjectSources) map[string]string {
	m := make(map[string]string)
	if srcs == nil {
		return m
	}
	for name := range srcs.Git {
		m[name] = filepath.Join(sourcesDir, "git", name)
	}
	for name := range srcs.HTTP {
		m[name] = filepath.Join(sourcesDir, "http", name)
	}
	for name := range srcs.Scrape {
		m[name] = filepath.Join(sourcesDir, "scrape", name)
	}
	return m
}

// scopeTags derives search tags from a convention's scope path.
func scopeTags(scope string) []string {
	if scope == "*" {
		return nil
	}
	var tags []string
	parts := strings.FieldsFunc(scope, func(r rune) bool {
		return r == '/' || r == '*'
	})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != "." {
			tags = append(tags, p)
		}
	}
	return tags
}

// ForPath returns conventions from the given map that match path.
// Conventions with no scope are skipped — they are search-only.
// Scopes support glob patterns: ** matches any depth, * matches within a
// single path segment, and a bare * matches any path.
func ForPath(conventions map[string]config.Convention, path string) []config.Convention {
	var matched []config.Convention
	for _, c := range conventions {
		if c.Scope == "" {
			continue
		}
		if c.Scope == "*" {
			matched = append(matched, c)
			continue
		}
		if glob.ToRegexp(c.Scope).MatchString(path) {
			matched = append(matched, c)
		}
	}
	return matched
}
