package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTMLToMarkdownElements(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:        "headings",
			html:        `<html><body><h1>Title</h1><h2>Section</h2><p>Content here.</p></body></html>`,
			wantPresent: []string{"# Title", "## Section", "Content here."},
		},
		{
			name:        "links",
			html:        `<p>Visit <a href="https://example.com">Example</a> for details.</p>`,
			wantPresent: []string{"Example"}, // href dropped, only link text kept
		},
		{
			name:        "unordered list",
			html:        `<ul><li>One</li><li>Two</li><li>Three</li></ul>`,
			wantPresent: []string{"- One"},
		},
		{
			name:        "ordered list",
			html:        `<ol><li>First</li><li>Second</li></ol>`,
			wantPresent: []string{"- First"}, // ordered/unordered both use dash; numbering has no LLM value
		},
		{
			name:        "code block",
			html:        `<pre><code class="language-go">func main() {}</code></pre>`,
			wantPresent: []string{"```go"},
		},
		{
			name:        "inline code",
			html:        `<p>Use <code>go build</code> to compile.</p>`,
			wantPresent: []string{"`go build`"},
		},
		{
			name:        "bold",
			html:        `<p>This is <strong>bold</strong>.</p>`,
			wantPresent: []string{"bold"}, // markers stripped
		},
		{
			name:        "italic",
			html:        `<p>This is <em>italic</em>.</p>`,
			wantPresent: []string{"italic"}, // markers stripped
		},
		{
			name:        "blockquote",
			html:        `<blockquote><p>A wise quote.</p></blockquote>`,
			wantPresent: []string{"A wise quote."}, // '>' prefix dropped
		},
		{
			name:        "entities",
			html:        `<p>A &amp; B &lt; C &gt; D</p>`,
			wantPresent: []string{"A & B < C > D"},
		},
		{
			name:        "table",
			html:        `<table><tr><th>Name</th><th>Value</th></tr><tr><td>Foo</td><td>Bar</td></tr></table>`,
			wantPresent: []string{"Name", "Foo"}, // rendered as tab-separated rows; no pipe syntax needed for LLM use
		},
		{
			name: "strip noise",
			html: `<html><head><script>var x=1;</script><style>body{}</style></head>` +
				`<body><nav>Menu</nav><h1>Hello</h1><footer>Copyright</footer></body></html>`,
			wantPresent: []string{"# Hello"},
			wantAbsent:  []string{"var x=1", "body{}", "Menu", "Copyright"},
		},
		{
			name: "full page",
			html: `<!DOCTYPE html><html><head><title>Test</title><script>var x=1;</script></head>` +
				`<body><nav><a href="/">Home</a></nav><main>` +
				`<h1>Welcome</h1><p>Hello <strong>world</strong> &amp; friends.</p>` +
				`<ul><li>Item 1</li><li>Item 2</li></ul>` +
				"<pre><code class=\"language-js\">console.log(\"hi\")</code></pre>" +
				`</main><footer><p>Copyright 2026</p></footer></body></html>`,
			wantPresent: []string{"# Welcome", "world", "& friends", "- Item 1", "```js"},
			wantAbsent:  []string{"var x=1", "Home", "Copyright"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := htmlToText(tt.html)
			for _, w := range tt.wantPresent {
				if !strings.Contains(md, w) {
					t.Fatalf("expected %q in output, got:\n%s", w, md)
				}
			}
			for _, w := range tt.wantAbsent {
				if strings.Contains(md, w) {
					t.Fatalf("unexpected %q in output, got:\n%s", w, md)
				}
			}
		})
	}
}

// --- Crawl tests using httptest ---

// newCrawlServer sets up a test HTTP server with linked pages.
// /           → links to /page1 and /page2
// /page1      → links to /page1/sub
// /page1/sub  → no outbound links
// /page2      → links to /external (different host, should be skipped)
func newCrawlServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<h1>Root</h1>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
		</body></html>`)
	})
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<h1>Page 1</h1>
			<a href="/page1/sub">Sub Page</a>
		</body></html>`)
	})
	mux.HandleFunc("/page1/sub", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h1>Sub Page</h1><p>Leaf node.</p></body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<h1>Page 2</h1>
			<a href="https://external.example.com/other">External</a>
		</body></html>`)
	})
	return httptest.NewServer(mux)
}

func TestScrape(t *testing.T) {
	tests := []struct {
		name        string
		maxDepth    int
		maxPages    int
		wantFiles   int
		wantMax     bool   // when true, assert len(files) <= wantFiles
		wantContent string // when non-empty, assert at least one file contains this string
	}{
		{name: "no crawl (depth 0)", maxDepth: 0, maxPages: 50, wantFiles: 1},
		{name: "depth 1", maxDepth: 1, maxPages: 50, wantFiles: 3, wantContent: "# Root"},
		{name: "depth 2", maxDepth: 2, maxPages: 50, wantFiles: 4},
		{name: "max pages cap", maxDepth: 2, maxPages: 2, wantFiles: 2, wantMax: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newCrawlServer()
			defer srv.Close()

			dir := t.TempDir()
			opts := ScrapeOptions{MaxPageSize: 1 << 20, MaxPages: tt.maxPages, MaxDepth: tt.maxDepth}
			src := &ScrapeSource{urls: []string{srv.URL + "/"}, opts: opts, destDir: dir, policy: "always"}
			if err := src.Fetch(context.Background()); err != nil {
				t.Fatal(err)
			}

			files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
			if tt.wantMax {
				if len(files) > tt.wantFiles {
					t.Fatalf("expected at most %d files, got %d", tt.wantFiles, len(files))
				}
			} else if len(files) != tt.wantFiles {
				t.Fatalf("expected %d files, got %d", tt.wantFiles, len(files))
			}

			if tt.wantContent != "" {
				found := false
				for _, f := range files {
					data, err := os.ReadFile(f)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(string(data), tt.wantContent) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected to find %q in crawled content", tt.wantContent)
				}
			}
		})
	}
}

func TestExtractLinks(t *testing.T) {
	html := `<html><body>
		<a href="/docs/page1">Page 1</a>
		<a href="https://example.com/other">Other</a>
		<a href="relative">Relative</a>
		<a href="#fragment">Fragment</a>
		<a href="mailto:x@y.com">Email</a>
	</body></html>`

	links := extractLinks(html, "https://example.com/docs/")
	if len(links) == 0 {
		t.Fatal("expected links")
	}

	// Should resolve /docs/page1 to absolute.
	found := false
	for _, l := range links {
		if l == "https://example.com/docs/page1" {
			found = true
		}
		// mailto should be filtered.
		if strings.HasPrefix(l, "mailto:") {
			t.Fatal("mailto links should be filtered")
		}
		// Fragments should be stripped.
		if strings.Contains(l, "#") {
			t.Fatalf("fragment should be stripped: %s", l)
		}
	}
	if !found {
		t.Fatalf("expected resolved /docs/page1, got: %v", links)
	}
}

func TestSeedPrefix(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"trailing slash kept", "https://example.com/docs/reference/", "https://example.com/docs/reference/"},
		{"no trailing slash", "https://example.com/docs/reference", "https://example.com/docs/"},
		{"root with slash", "https://example.com/", "https://example.com/"},
		{"root without slash", "https://example.com", "https://example.com/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := seedPrefix(tt.url)
			if got != tt.want {
				t.Errorf("seedPrefix(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"strips fragment", "https://example.com/page#section", "https://example.com/page"},
		{"strips trailing slash", "https://example.com/page/", "https://example.com/page"},
		{"no change needed", "https://example.com/page", "https://example.com/page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeURL(tt.url)
			if got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestMatchesSeedPrefix(t *testing.T) {
	prefixes := []string{"https://example.com/docs/"}
	tests := []struct {
		name   string
		url    string
		wantOK bool
	}{
		{"matching prefix", "https://example.com/docs/page1", true},
		{"different path", "https://example.com/blog/post", false},
		{"different host", "https://other.com/docs/page", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesSeedPrefix(tt.url, prefixes)
			if got != tt.wantOK {
				t.Errorf("matchesSeedPrefix(%q) = %v, want %v", tt.url, got, tt.wantOK)
			}
		})
	}
}

func TestURLToFilename(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"simple path", "https://example.com/docs/page", "example.com_docs_page.md"},
		{"trailing slash stripped", "https://example.com/docs/page/", "example.com_docs_page.md"},
		{"html extension stripped", "https://example.com/page.html", "example.com_page.md"},
		{"htm extension stripped", "https://example.com/page.htm", "example.com_page.md"},
		{"root", "https://example.com/", "example.com.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := urlToFilename(tt.url)
			if got != tt.want {
				t.Errorf("urlToFilename(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestURLToFilename_LongURL(t *testing.T) {
	// A URL producing a name >100 chars must be truncated; result must still end in .md.
	long := "https://example.com/" + strings.Repeat("x", 120)
	got := urlToFilename(long)
	if !strings.HasSuffix(got, ".md") {
		t.Errorf("expected .md suffix, got %q", got)
	}
	if len(got) > 110 {
		t.Errorf("expected truncated filename, got length %d: %q", len(got), got)
	}
}
