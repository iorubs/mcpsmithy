package search

import (
	"os"
	"path/filepath"
	"testing"
)

// chunkFiles is a test helper that reads files matching globs under root and chunks them.
func chunkFiles(root string, globs []string, strategy string) ([]Chunk, error) {
	var chunks []Chunk
	for _, g := range globs {
		matches, err := filepath.Glob(filepath.Join(root, g))
		if err != nil {
			return nil, err
		}
		for _, f := range matches {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			rel, _ := filepath.Rel(root, f)
			chunks = append(chunks, ChunkContent(rel, string(data), ResolveStrategy(strategy, rel))...)
		}
	}
	return chunks, nil
}

func TestChunkMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		md    string
		check func(*testing.T, []Chunk)
	}{
		{
			name: "headings",
			file: "test.md",
			md:   "# Title\n\nIntro text.\n\n## Section One\n\nContent one.\n\n## Section Two\n\nContent two.\n",
			check: func(t *testing.T, chunks []Chunk) {
				if len(chunks) != 3 {
					t.Fatalf("expected 3 chunks, got %d", len(chunks))
				}
				if chunks[0].Title != "Title" {
					t.Fatalf("expected title 'Title', got %q", chunks[0].Title)
				}
				if chunks[1].Title != "Section One" {
					t.Fatalf("expected 'Section One', got %q", chunks[1].Title)
				}
				if chunks[2].Title != "Section Two" {
					t.Fatalf("expected 'Section Two', got %q", chunks[2].Title)
				}
			},
		},
		{
			name: "no headings",
			file: "plain.md",
			md:   "Just plain text.\nMore lines.\n",
			check: func(t *testing.T, chunks []Chunk) {
				if len(chunks) != 1 {
					t.Fatalf("expected 1 chunk, got %d", len(chunks))
				}
				if chunks[0].Title != "plain.md" {
					t.Fatalf("expected source as title, got %q", chunks[0].Title)
				}
			},
		},
		{
			name: "breadcrumb",
			file: "test.md",
			md:   "# A\n\nIntro text.\n\n## B\n\nSub content.\n",
			check: func(t *testing.T, chunks []Chunk) {
				if len(chunks) != 2 {
					t.Fatalf("expected 2 chunks, got %d", len(chunks))
				}
				if chunks[0].Section != "A" {
					t.Errorf("chunk 0 Section = %q, want %q", chunks[0].Section, "A")
				}
				if chunks[1].Section != "A > B" {
					t.Errorf("chunk 1 Section = %q, want %q", chunks[1].Section, "A > B")
				}
			},
		},
		{
			name: "frontmatter",
			file: "test.md",
			md:   "---\ntitle: Ignored\n---\n\n# Title\n\nBody text.\n",
			check: func(t *testing.T, chunks []Chunk) {
				if len(chunks) != 1 {
					t.Fatalf("expected 1 chunk, got %d", len(chunks))
				}
				if chunks[0].Title != "Title" {
					t.Errorf("chunk title = %q, want %q", chunks[0].Title, "Title")
				}
				body := chunks[0].Body
				if len(body) >= 3 && body[0:3] == "---" {
					t.Errorf("frontmatter leaked into chunk body: %q", body)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, chunkMarkdown(tt.file, tt.md))
		})
	}
}

func TestChunkFiles(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		glob      string
		strategy  string
		wantLen   int
		wantTitle string
	}{
		{
			name:      "markdown section strategy",
			files:     map[string]string{"a.md": "# Hello\n\nWorld.\n"},
			glob:      "*.md",
			strategy:  "section",
			wantLen:   1,
			wantTitle: "Hello",
		},
		{
			name:      "text file strategy",
			files:     map[string]string{"notes.txt": "some notes"},
			glob:      "*.txt",
			strategy:  "file",
			wantLen:   1,
			wantTitle: "notes.txt",
		},
		{
			name:      "deep double-star",
			files:     map[string]string{"docs/guide/x.md": "# Deep\n\nNested doc."},
			glob:      "docs/**/*.md",
			strategy:  "section",
			wantLen:   1,
			wantTitle: "Deep",
		},
		{
			name: "multi-segment v1 only",
			files: map[string]string{
				"config/v1/types.go": "package v1",
				"config/v1/parse.go": "package v1",
				"config/v2/types.go": "package v2",
			},
			glob:     "**/v1/*.go",
			strategy: "file",
			wantLen:  2,
		},
		{
			name:      "auto strategy md",
			files:     map[string]string{"doc.md": "# Title\n\nContent.\n"},
			glob:      "*.md",
			strategy:  "",
			wantLen:   1,
			wantTitle: "Title",
		},
		{
			name:      "auto strategy txt",
			files:     map[string]string{"notes.txt": "# Not a heading\n\nJust text.\n"},
			glob:      "*.txt",
			strategy:  "",
			wantLen:   1,
			wantTitle: "notes.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for relPath, content := range tt.files {
				full := filepath.Join(dir, filepath.FromSlash(relPath))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			chunks, err := chunkFiles(dir, []string{tt.glob}, tt.strategy)
			if err != nil {
				t.Fatal(err)
			}
			if len(chunks) != tt.wantLen {
				t.Fatalf("expected %d chunk(s), got %d: %v", tt.wantLen, len(chunks), chunks)
			}
			if tt.wantTitle != "" && chunks[0].Title != tt.wantTitle {
				t.Fatalf("expected title %q, got %q", tt.wantTitle, chunks[0].Title)
			}
		})
	}
}

func TestChunkContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		strategy string
		check    func(*testing.T, []Chunk)
	}{
		{
			name:     "section",
			content:  "# Intro\n\nHello world.\n\n## Details\n\nMore info.",
			strategy: "section",
			check: func(t *testing.T, chunks []Chunk) {
				if len(chunks) != 2 {
					t.Fatalf("expected 2 chunks (section), got %d", len(chunks))
				}
				if chunks[0].Title != "Intro" {
					t.Fatalf("expected 'Intro', got %q", chunks[0].Title)
				}
			},
		},
		{
			name:     "file",
			content:  "# Heading\n\nSome content.",
			strategy: "file",
			check: func(t *testing.T, chunks []Chunk) {
				if len(chunks) != 1 {
					t.Fatalf("expected 1 chunk (file), got %d", len(chunks))
				}
				if chunks[0].Title != "test.md" {
					t.Fatalf("expected source as title, got %q", chunks[0].Title)
				}
			},
		},
		{
			name:     "none",
			content:  "anything",
			strategy: "none",
			check: func(t *testing.T, chunks []Chunk) {
				if len(chunks) != 0 {
					t.Fatalf("expected 0 chunks (none), got %d", len(chunks))
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, ChunkContent("test.md", tt.content, tt.strategy))
		})
	}
}

func TestResolveStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		file     string
		want     string
	}{
		{"md auto section", "", "docs/guide.md", "section"},
		{"markdown auto section", "", "README.markdown", "section"},
		{"mdx auto section", "", "notes.mdx", "section"},
		{"go auto file", "", "main.go", "file"},
		{"yaml auto file", "", "config.yaml", "file"},
		{"json auto file", "", "data.json", "file"},
		{"explicit overrides auto", "file", "docs/guide.md", "file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStrategy(tt.strategy, tt.file)
			if got != tt.want {
				t.Errorf("ResolveStrategy(%q, %q) = %q, want %q", tt.strategy, tt.file, got, tt.want)
			}
		})
	}
}

func TestMarkdownHeadingLevel(t *testing.T) {
	tests := []struct {
		line      string
		wantLevel int
		wantTitle string
	}{
		{"# Title", 1, "Title"},
		{"## Sub heading", 2, "Sub heading"},
		{"### Deep", 3, "Deep"},
		{"###### H6", 6, "H6"},
		{"### ", 3, ""},
		{"#tag", 0, ""},
		{"#no-space", 0, ""},
		{"# ", 1, ""},
		{"####### too deep", 0, ""},
		{"not a heading", 0, ""},
		{"", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			gotLevel, gotTitle := markdownHeadingLevel(tt.line)
			if gotLevel != tt.wantLevel || gotTitle != tt.wantTitle {
				t.Errorf("markdownHeadingLevel(%q) = (%d, %q), want (%d, %q)",
					tt.line, gotLevel, gotTitle, tt.wantLevel, tt.wantTitle)
			}
		})
	}
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "with frontmatter",
			input: "---\ntitle: X\nauthor: Y\n---\n\n# Title\n\nBody",
			want:  "# Title\n\nBody",
		},
		{
			name:  "no frontmatter",
			input: "# Title\n\nBody",
			want:  "# Title\n\nBody",
		},
		{
			name:  "unclosed frontmatter",
			input: "---\ntitle: X\n\n# Title\n\nBody",
			want:  "---\ntitle: X\n\n# Title\n\nBody",
		},
		{
			name:  "empty frontmatter",
			input: "---\n---\n\n# Title",
			want:  "# Title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFrontmatter(tt.input)
			if got != tt.want {
				t.Errorf("stripFrontmatter: got %q, want %q", got, tt.want)
			}
		})
	}
}
