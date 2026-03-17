package sources

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func init() {
	DefaultRegistry.Register("scrape", func(name string, raw any, _, baseDir string, global config.PullPolicy) (Source, SourceMeta, error) {
		src, ok := raw.(config.ScrapeSource)
		if !ok {
			return nil, SourceMeta{}, fmt.Errorf("scrape source %q: unexpected config type %T", name, raw)
		}
		destDir := filepath.Join(baseDir, "scrape", name)
		return &ScrapeSource{
				urls: src.URLs,
				opts: ScrapeOptions{
					MaxPages:    src.MaxPages,
					MaxPageSize: int64(src.MaxPageSize) * 1024,
					MaxDepth:    src.MaxDepth,
				},
				destDir: destDir,
				policy:  resolvePolicy(src.PullPolicy, global),
			}, SourceMeta{
				NoIndex:   src.Index != nil && !*src.Index,
				ReadGlobs: []string{"*.md"},
			}, nil
	})
}

// ScrapeSource crawls web pages and saves them as markdown to disk.
type ScrapeSource struct {
	urls    []string
	opts    ScrapeOptions
	destDir string
	policy  config.PullPolicy
}

// httpScrapeClient uses a shorter timeout than the HTTP source client
// since pages are small and a hung page must not stall the whole crawl.
var httpScrapeClient = &http.Client{
	Timeout: 30 * time.Second,
}

// ScrapeOptions configures limits for URL scraping.
type ScrapeOptions struct {
	MaxPageSize int64 // max bytes per page
	MaxPages    int   // max URLs to fetch (seeds + discovered)
	MaxDepth    int   // max link-following depth; 0 = seed URLs only
}

// Fetch scrapes the configured URLs, converts HTML to text, and saves each
// page as a .md file under destDir. When opts.MaxDepth > 0, discovered links
// that share the same origin and path prefix as their seed URL are followed.
// Existing destDir content is cleared first for a clean fetch.
func (s *ScrapeSource) Fetch(ctx context.Context) error {
	if skipFetch(s.policy, s.destDir) {
		return nil
	}

	urls := s.urls
	if len(urls) > s.opts.MaxPages {
		urls = urls[:s.opts.MaxPages]
	}

	if err := os.RemoveAll(s.destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(s.destDir, 0o755); err != nil {
		return err
	}

	seedPrefixes := make([]string, 0, len(urls))
	for _, u := range urls {
		if p := seedPrefix(u); p != "" {
			seedPrefixes = append(seedPrefixes, p)
		}
	}

	type entry struct {
		url   string
		depth int
	}
	queue := make([]entry, 0, len(urls))
	visited := make(map[string]bool, len(urls))
	for _, u := range urls {
		if norm := normalizeURL(u); !visited[norm] {
			visited[norm] = true
			queue = append(queue, entry{url: u, depth: 0})
		}
	}

	fetched := 0
	for len(queue) > 0 && fetched < s.opts.MaxPages {
		e := queue[0]
		queue = queue[1:]

		if err := ctx.Err(); err != nil {
			return err
		}

		content, contentType, err := fetchURL(ctx, e.url, s.opts.MaxPageSize)
		if err != nil {
			continue // skip fetch failures; partial results are fine
		}
		fetched++

		isHTML := strings.Contains(contentType, "text/html") ||
			(!strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "text/markdown"))

		if isHTML && s.opts.MaxDepth > 0 && e.depth < s.opts.MaxDepth {
			for _, link := range extractLinks(content, e.url) {
				if norm := normalizeURL(link); !visited[norm] && matchesSeedPrefix(link, seedPrefixes) {
					visited[norm] = true
					queue = append(queue, entry{url: link, depth: e.depth + 1})
				}
			}
		}

		var md string
		if isHTML {
			md = htmlToText(content)
		} else {
			md = strings.TrimSpace(content)
		}
		if md == "" {
			continue
		}

		if err := os.WriteFile(filepath.Join(s.destDir, urlToFilename(e.url)), []byte(md), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *ScrapeSource) Read(globs []string, prefix string) ([]RawDoc, error) {
	return readFS(os.DirFS(s.destDir), globs, prefix)
}

// extractLinks parses raw HTML and returns absolute HTTP(S) URLs from <a href>.
func extractLinks(rawHTML, baseURL string) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}

	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.A {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if ref, err := url.Parse(a.Val); err == nil {
						resolved := base.ResolveReference(ref)
						if resolved.Scheme == "http" || resolved.Scheme == "https" {
							resolved.Fragment = ""
							links = append(links, resolved.String())
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}

// seedPrefix returns the origin + path directory prefix for scope filtering.
func seedPrefix(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	p := u.Path
	if p == "" {
		p = "/"
	}
	if !strings.HasSuffix(p, "/") {
		p = p[:strings.LastIndex(p, "/")+1]
	}
	return u.Scheme + "://" + u.Host + p
}

// matchesSeedPrefix reports whether rawURL falls under any seed prefix.
func matchesSeedPrefix(rawURL string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(rawURL, prefix) {
			return true
		}
	}
	return false
}

// normalizeURL strips fragment and trailing slash for deduplication.
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Fragment = ""
	return strings.TrimSuffix(u.String(), "/")
}

// fetchURL fetches a single URL and returns its body and content type.
func fetchURL(ctx context.Context, rawURL string, maxPageSize int64) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "mcpsmithy/1.0")
	req.Header.Set("Accept", "text/html, text/plain, text/markdown")

	resp, err := httpScrapeClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetching %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("fetching %q: HTTP %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageSize))
	if err != nil {
		return "", "", fmt.Errorf("reading %q: %w", rawURL, err)
	}

	return string(body), resp.Header.Get("Content-Type"), nil
}

var urlReplacer = strings.NewReplacer("/", "_", ":", "_")

// urlToFilename creates a filesystem-safe .md filename from a URL.
func urlToFilename(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return hashString(rawURL) + ".md"
	}
	name := urlReplacer.Replace(strings.TrimSuffix(u.Host+u.Path, "/"))
	if len(name) > 100 {
		name = name[:80] + "_" + hashString(rawURL)[:8]
	}
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".html"), ".htm")
	return name + ".md"
}

// hashString returns a short hex SHA256 hash.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

// --------------- HTML → plain text converter ---------------
// Produces clean text for LLM consumption. Inline formatting (bold, italic,
// links, images) is stripped; structure (headings, lists, code) is kept as
// it carries semantic signal. Tables are rendered as tab-separated rows.

var (
	reExcessNewlines = regexp.MustCompile(`\n{3,}`)
	reStripTags      = regexp.MustCompile(`<[^>]+>`)

	noiseElements = map[atom.Atom]bool{
		atom.Script: true, atom.Style: true, atom.Noscript: true,
		atom.Nav: true, atom.Footer: true, atom.Header: true,
		atom.Svg: true, atom.Iframe: true,
	}
	headingMarker = map[atom.Atom]string{
		atom.H1: "# ", atom.H2: "## ", atom.H3: "### ",
		atom.H4: "#### ", atom.H5: "##### ", atom.H6: "###### ",
	}
)

func htmlToText(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return strings.TrimSpace(reStripTags.ReplaceAllString(rawHTML, " "))
	}
	var b strings.Builder
	walkNode(&b, doc, 0)
	return strings.TrimSpace(reExcessNewlines.ReplaceAllString(b.String(), "\n\n"))
}

func walkNode(b *strings.Builder, n *html.Node, depth int) {
	if n.Type == html.TextNode {
		b.WriteString(collapseWhitespace(n.Data))
		return
	}
	if n.Type != html.ElementNode {
		walkChildren(b, n, depth)
		return
	}
	if noiseElements[n.DataAtom] {
		return
	}
	if marker, ok := headingMarker[n.DataAtom]; ok {
		b.WriteString("\n\n" + marker)
		walkChildren(b, n, depth)
		b.WriteString("\n\n")
		return
	}
	switch n.DataAtom {
	case atom.P:
		b.WriteString("\n\n")
		walkChildren(b, n, depth)
		b.WriteString("\n\n")
	case atom.Br:
		b.WriteString("\n")
	case atom.Hr:
		b.WriteString("\n\n---\n\n")
	case atom.Strong, atom.B, atom.Em, atom.I, atom.Span, atom.A:
		walkChildren(b, n, depth)
	case atom.Img:
		// no value for LLM text; skip
	case atom.Code:
		if n.Parent != nil && n.Parent.DataAtom == atom.Pre {
			b.WriteString("```" + extractLang(getAttr(n, "class")) + "\n")
			writeRawText(b, n)
			b.WriteString("\n```")
		} else {
			b.WriteString("`")
			walkChildren(b, n, depth)
			b.WriteString("`")
		}
	case atom.Pre:
		b.WriteString("\n\n")
		if c := firstElementChild(n); c != nil && c.DataAtom == atom.Code {
			walkChildren(b, n, depth)
		} else {
			b.WriteString("```\n")
			writeRawText(b, n)
			b.WriteString("\n```")
		}
		b.WriteString("\n\n")
	case atom.Ul, atom.Ol:
		b.WriteString("\n")
		walkChildren(b, n, depth+1)
		b.WriteString("\n")
	case atom.Li:
		b.WriteString(strings.Repeat("  ", depth-1) + "- ")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	case atom.Tr:
		walkChildren(b, n, depth)
		b.WriteString("\n")
	case atom.Td, atom.Th:
		walkChildren(b, n, depth)
		b.WriteString("\t")
	case atom.Dt:
		b.WriteString("\n")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	case atom.Dd:
		b.WriteString("  ")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	default:
		b.WriteString("\n")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	}
}

func walkChildren(b *strings.Builder, n *html.Node, depth int) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkNode(b, c, depth)
	}
}

func writeRawText(b *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeRawText(b, c)
	}
}

// writeTable converts an HTML <table> into a Markdown pipe-table.
func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func extractLang(class string) string {
	for part := range strings.FieldsSeq(class) {
		if after, ok := strings.CutPrefix(part, "language-"); ok {
			return after
		}
		if after, ok := strings.CutPrefix(part, "lang-"); ok {
			return after
		}
	}
	return ""
}

func firstElementChild(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	return nil
}

// collapseWhitespace reduces runs of whitespace to a single space,
// trimming leading whitespace before any content has been written.
func collapseWhitespace(s string) string {
	var b strings.Builder
	lastWasSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !lastWasSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			lastWasSpace = true
			continue
		}
		b.WriteRune(r)
		lastWasSpace = false
	}
	return b.String()
}
