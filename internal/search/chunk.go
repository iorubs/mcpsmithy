// Package search provides chunk types, file-to-chunk splitting, and
// in-memory full-text search with BM25 scoring.
package search

import (
	"bufio"
	"path/filepath"
	"strings"
)

// Chunk is the atomic unit of searchable content.
type Chunk struct {
	// Source is the file path relative to the project root.
	Source string
	// Section is the heading hierarchy (e.g. "## Ownership > ### Cross-namespace").
	Section string
	// Title is the heading text or key name.
	Title string
	// Body is the full text content of the section.
	Body string
	// Tags are metadata labels (from config or auto-derived).
	Tags []string
	// ConventionID links this chunk to a convention (empty if from file source).
	ConventionID string
	// Docs lists documentation paths associated with a convention chunk.
	Docs []string
}

// ChunkContent splits content into chunks using the given strategy.
// strategy must be "section", "file", or "none".
func ChunkContent(source, content, strategy string) []Chunk {
	switch strategy {
	case "section":
		return chunkMarkdown(source, content)
	case "none":
		return nil
	default: // "file" or fallback
		return []Chunk{{Source: source, Title: source, Body: content}}
	}
}

// ResolveStrategy picks a strategy when the user provides "" (auto).
func ResolveStrategy(strategy, filename string) string {
	if strategy != "" {
		return strategy
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown", ".mdx":
		return "section"
	default:
		return "file"
	}
}

// chunkMarkdown splits Markdown content by ATX headings.
// It strips YAML frontmatter before splitting, requires a space after
// the # characters (e.g. "# Title", not "#tag"), and builds a full
// ancestor breadcrumb in the Section field (e.g. "Feature X > Installation").
func chunkMarkdown(source, content string) []Chunk {
	content = stripFrontmatter(content)

	var chunks []Chunk
	var (
		currentTitle string
		body         strings.Builder
		headings     [6]string // headings[0]=H1 … headings[5]=H6
	)

	buildSection := func() string {
		var parts []string
		for _, h := range headings {
			if h != "" {
				parts = append(parts, h)
			}
		}
		return strings.Join(parts, " > ")
	}

	flush := func() {
		text := strings.TrimSpace(body.String())
		if text == "" && currentTitle == "" {
			return
		}
		title := currentTitle
		if title == "" {
			title = source
		}
		chunks = append(chunks, Chunk{
			Source:  source,
			Section: buildSection(),
			Title:   title,
			Body:    text,
		})
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if level, title := markdownHeadingLevel(line); level > 0 {
			flush()
			body.Reset()

			currentTitle = title
			headings[level-1] = title
			// Clear all deeper heading levels.
			for i := level; i < 6; i++ {
				headings[i] = ""
			}
		} else {
			if body.Len() > 0 {
				body.WriteByte('\n')
			}
			body.WriteString(line)
		}
	}
	flush() // last section
	return chunks
}

// markdownHeadingLevel returns the ATX heading depth (1–6) and trimmed
// title text if line is a valid Markdown heading, or (0, "") otherwise.
// Requires a space after the # characters — bare "#tag" is not a heading.
func markdownHeadingLevel(line string) (int, string) {
	if len(line) == 0 || line[0] != '#' {
		return 0, ""
	}
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level > 6 || level >= len(line) || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level+1:])
}

// stripFrontmatter removes YAML frontmatter (---…---) from the start of
// Markdown content. If no valid frontmatter block is found, content is
// returned unchanged.
func stripFrontmatter(content string) string {
	const sep = "---"
	if !strings.HasPrefix(content, sep+"\n") && !strings.HasPrefix(content, sep+"\r\n") {
		return content
	}
	// Skip past the opening "---\n".
	rest := content[len(sep):]
	if len(rest) > 0 && rest[0] == '\r' {
		rest = rest[1:]
	}
	rest = rest[1:] // skip '\n'

	// Find the closing "---" on its own line. Also handle empty frontmatter
	// where the closing delimiter immediately follows the opening one.
	for _, prefix := range []string{"---\r\n", "---\n", "---"} {
		if strings.HasPrefix(rest, prefix) {
			after := rest[len(prefix):]
			after = strings.TrimPrefix(after, "\r\n")
			after = strings.TrimPrefix(after, "\n")
			return after
		}
	}
	for _, nl := range []string{"\n---\r\n", "\n---\n", "\n---"} {
		if _, after, ok := strings.Cut(rest, nl); ok {
			after := after
			// Strip a leading blank line left after the closing delimiter.
			after = strings.TrimPrefix(after, "\r\n")
			after = strings.TrimPrefix(after, "\n")
			return after
		}
	}
	return content // no closing delimiter — leave untouched
}
