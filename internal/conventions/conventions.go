// Package conventions matches file paths to project documentation
// and provides browsing of convention definitions.
package conventions

import (
	"context"
	"log/slog"
	"strings"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/glob"
	"github.com/operator-assistant/mcpsmithy/internal/search"
)

// BuildIndex creates a search index from the conventions map.
// Conventions are indexed independently so they are always surfaced
// prominently in search results, regardless of BM25 scores from source docs.
func BuildIndex(ctx context.Context, conventions map[string]config.Convention) search.Searcher {
	var chunks []search.Chunk
	for name, c := range conventions {
		var docStrings []string
		for _, d := range c.Docs {
			if len(d.Paths) > 0 {
				docStrings = append(docStrings, d.Paths...)
			} else {
				docStrings = append(docStrings, d.Source)
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
