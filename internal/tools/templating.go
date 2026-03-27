package tools

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/operator-assistant/mcpsmithy/internal/auth"
	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/conventions"
	"github.com/operator-assistant/mcpsmithy/internal/search"
)

// defaultMaxReadKB is the default HTTP response body cap (10 MB) used by
// http_get, http_post, and http_put when the caller does not supply a limit.
const defaultMaxReadKB = 10 * 1024

// ansiRe matches ANSI escape sequences (CSI sequences and OSC sequences).
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x1b]*\x1b\\`)

// noRedirectClient is an HTTP client that never follows redirects.
// Redirects are treated as errors so callers get an explicit failure with
// the redirect destination rather than silently receiving content from a
// different URL (e.g. an SSO login page).
// If redirect-following becomes necessary in the future, this can be
// extended (e.g. opt-in via a third argument to http_get).
var noRedirectClient = &http.Client{
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// templateEngine provides template rendering with project-aware functions.
type templateEngine struct {
	cfg         *config.Config
	root        string
	conventions map[string]config.Convention
	idx         search.Searcher // source docs index (async-safe via LiveIndex)
	convIdx     search.Searcher // conventions-only index
	fsys        fs.FS           // sandbox filesystem for file_read
}

func newTemplateEngine(cfg *config.Config, root string, convs map[string]config.Convention, idx search.Searcher, convIdx search.Searcher, fsys fs.FS) *templateEngine {
	return &templateEngine{cfg: cfg, root: root, conventions: convs, idx: idx, convIdx: convIdx, fsys: fsys}
}

// mcpsmithyContext is the struct injected as {{ .mcpsmithy }} in
// every tool template. It gives templates access to the full config.
type mcpsmithyContext struct {
	Project     projectContext
	Conventions map[string]config.Convention
	Tools       map[string]config.Tool
	sources     *config.ProjectSources // unexported; accessed via Source() method
}

// Source returns a human-readable description of a named source,
// falling through Local → Git → Scrape. This replaces the former
// sources_for built-in function.
func (c mcpsmithyContext) Source(name string) string {
	if c.sources == nil {
		return "No sources configured."
	}
	if src, ok := c.sources.Local[name]; ok {
		return strings.Join(src.Paths, ", ")
	}
	if src, ok := c.sources.Git[name]; ok {
		line := src.Repo
		if len(src.Paths) > 0 {
			line += " — " + strings.Join(src.Paths, ", ")
		}
		return line
	}
	if src, ok := c.sources.Scrape[name]; ok {
		return strings.Join(src.URLs, ", ")
	}
	if src, ok := c.sources.HTTP[name]; ok {
		return src.URL
	}
	return fmt.Sprintf("Unknown source: %q", name)
}

// projectContext wraps Project fields with the sandbox root path.
type projectContext struct {
	Name        string
	Description string
	Root        string
	Sources     *config.ProjectSources
}

// String renders a human-readable project summary so that
// {{ .mcpsmithy.Project }} produces useful output in templates.
func (p projectContext) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Project: %s\n", p.Name)
	fmt.Fprintf(&sb, "Description: %s\n", p.Description)
	fmt.Fprintf(&sb, "Root: %s", p.Root)
	return sb.String()
}

// Context creates a template data map with the mcpsmithy namespace and user params.
func (e *templateEngine) Context(params map[string]any) map[string]any {
	ctx := map[string]any{
		config.ReservedContextKey: mcpsmithyContext{
			Project: projectContext{
				Name:        e.cfg.Project.Name,
				Description: e.cfg.Project.Description,
				Root:        e.root,
				Sources:     e.cfg.Project.Sources,
			},
			Conventions: e.cfg.Conventions,
			Tools:       e.cfg.Tools,
			sources:     e.cfg.Project.Sources,
		},
	}
	for k, v := range params {
		if k == config.ReservedContextKey {
			continue // prevent callers from overwriting engine-owned key
		}
		ctx[k] = v
	}
	return ctx
}

// parseURLAllowList extracts the urlAllowList option from tool options
// and returns a set of allowed "scheme://host" strings. Returns nil
// when the option is absent (meaning all URLs are allowed).
func parseURLAllowList(opts map[string]any) map[string]bool {
	raw, ok := opts["urlAllowList"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	allowed := make(map[string]bool, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		u, err := url.Parse(s)
		if err != nil || u.Host == "" {
			continue
		}
		allowed[strings.ToLower(u.Scheme)+"://"+strings.ToLower(u.Host)] = true
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

// urlAllowedByList checks whether rawURL's scheme+host is present in the
// allowlist. Returns true when the list is nil (no restrictions).
func urlAllowedByList(rawURL string, allowed map[string]bool) error {
	if allowed == nil {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	key := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	if !allowed[key] {
		return fmt.Errorf("URL %q is not in the allowed list", rawURL)
	}
	return nil
}

// funcMap returns the template function map wired to conventions and search.
// opts are the tool-level options from which urlAllowList is extracted.
func (e *templateEngine) funcMap(opts map[string]any) template.FuncMap {
	allowedURLs := parseURLAllowList(opts)
	return template.FuncMap{
		string(config.BuiltinFuncConventionsFor): func(path string) []config.Convention {
			return conventions.ForPath(e.conventions, path)
		},
		string(config.BuiltinFuncSearchFor): func(query string, opts ...any) string {
			maxResults := 10
			maxResultSize := 200
			if len(opts) > 0 {
				if n, ok := opts[0].(int); ok {
					maxResults = n
				} else if f, ok := opts[0].(float64); ok {
					maxResults = int(f)
				}
			}
			if len(opts) > 1 {
				if n, ok := opts[1].(int); ok {
					maxResultSize = n
				} else if f, ok := opts[1].(float64); ok {
					maxResultSize = int(f)
				}
			}
			return e.searchFor(query, maxResults, maxResultSize)
		},
		string(config.BuiltinFuncFileRead): func(pathGlob string, maxFileSize ...any) string {
			limit := 50
			if len(maxFileSize) > 0 {
				if n, ok := maxFileSize[0].(int); ok {
					limit = n
				} else if f, ok := maxFileSize[0].(float64); ok {
					limit = int(f)
				}
			}
			return e.fileRead(pathGlob, limit)
		},
		string(config.BuiltinFuncHTTPGet): func(rawURL string, args ...any) (string, error) {
			limit := defaultMaxReadKB
			if len(args) > 0 {
				if n, ok := args[0].(int); ok {
					limit = n
				}
			}
			return httpFetch(context.Background(), rawURL, int64(limit)*1024, allowedURLs)
		},
		string(config.BuiltinFuncHTTPPost): func(rawURL string, body string, args ...any) (string, error) {
			contentType := "application/json"
			limit := defaultMaxReadKB
			if len(args) > 0 {
				if s, ok := args[0].(string); ok && s != "" {
					contentType = s
				}
			}
			if len(args) > 1 {
				if n, ok := args[1].(int); ok {
					limit = n
				}
			}
			return httpSend(context.Background(), http.MethodPost, rawURL, body, contentType, int64(limit)*1024, allowedURLs)
		},
		string(config.BuiltinFuncHTTPPut): func(rawURL string, body string, args ...any) (string, error) {
			contentType := "application/json"
			limit := defaultMaxReadKB
			if len(args) > 0 {
				if s, ok := args[0].(string); ok && s != "" {
					contentType = s
				}
			}
			if len(args) > 1 {
				if n, ok := args[1].(int); ok {
					limit = n
				}
			}
			return httpSend(context.Background(), http.MethodPut, rawURL, body, contentType, int64(limit)*1024, allowedURLs)
		},
		string(config.BuiltinFuncGrep): func(pattern string, before, after float64, input string) string {
			return grepFunc(pattern, before, after, input)
		},
	}
}

// searchFor implements the search_for template function.
// It queries two separate indexes — conventions first, then source
// docs — so that conventions are always surfaced prominently.
//
// maxResults controls how many results are returned per search.
// maxResultSize controls the preview character limit for source doc results.
// Both are set via tool options by the config author.
func (e *templateEngine) searchFor(query string, maxResults, maxResultSize int) string {
	hasConvIdx := e.convIdx != nil && e.convIdx.Len() > 0
	hasSourceIdx := e.idx != nil && e.idx.Len() > 0

	if !hasConvIdx && !hasSourceIdx {
		return "No search index available."
	}

	n := maxResults
	previewSize := maxResultSize

	var sb strings.Builder
	wrote := false

	// --- Conventions section (separate index, always shown first) ---
	if hasConvIdx {
		convResults := e.convIdx.Search(query, n)
		if len(convResults) > 0 {
			sb.WriteString("Matching conventions (review these first):\n")
			for i, r := range convResults {
				sb.WriteString(fmt.Sprintf("%d. %s  [%s]\n", i+1, r.Chunk.Title, r.Chunk.Source))
				if len(r.Chunk.Tags) > 0 {
					sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(r.Chunk.Tags, ", ")))
				}
				// Conventions are short by design — always show the full body.
				if r.Chunk.Body != "" {
					sb.WriteString(fmt.Sprintf("   %s\n", r.Chunk.Body))
				}
				if len(r.Chunk.Docs) > 0 {
					sb.WriteString(fmt.Sprintf("   Docs: %s\n", strings.Join(r.Chunk.Docs, ", ")))
				}
				sb.WriteByte('\n')
			}

			// Hint to the agent about getting full convention details.
			convTool := ""
			for name, t := range e.cfg.Tools {
				tmpl := string(t.Template)
				if strings.Contains(tmpl, "index .mcpsmithy.Conventions") {
					convTool = name
					break
				}
			}
			if convTool != "" {
				sb.WriteString(fmt.Sprintf("Tip: Use %s with a convention name above to get full details including docs and relations.\n", convTool))
			} else {
				sb.WriteString("Tip: Look up a convention by name using {{ index .mcpsmithy.Conventions \"<name>\" }} in a tool template to get full details.\n")
			}
			sb.WriteByte('\n')
			wrote = true
		}
	}

	// --- Source docs section ---
	if hasSourceIdx {
		results := e.idx.Search(query, n)
		if len(results) > 0 {
			if wrote {
				sb.WriteString("Source results:\n")
			}
			for i, r := range results {
				sb.WriteString(fmt.Sprintf("%d. %s", i+1, r.Chunk.Title))
				if r.Chunk.Source != "" && r.Chunk.Source != r.Chunk.Title {
					sb.WriteString(fmt.Sprintf("  [%s]", r.Chunk.Source))
				}
				sb.WriteByte('\n')
				if len(r.Chunk.Tags) > 0 {
					sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(r.Chunk.Tags, ", ")))
				}
				body := r.Chunk.Body
				if previewSize > 0 && len(body) > previewSize {
					trunc := body[:previewSize]
					for !utf8.ValidString(trunc) && len(trunc) > 0 {
						trunc = trunc[:len(trunc)-1]
					}
					body = trunc + "..."
				}
				if body != "" {
					sb.WriteString(fmt.Sprintf("   %s\n", body))
				}
				sb.WriteByte('\n')
			}
			if len(results) >= n {
				sb.WriteString("Tip: Results may be truncated. Consider narrowing your query.\n")
			}
			wrote = true
		}
	}

	if !wrote {
		return fmt.Sprintf("No results for %q.", query)
	}

	return sb.String()
}

// fileRead implements the file_read template function.
// It resolves a glob pattern against the sandbox filesystem, reads
// matching files (up to maxFileSize KB each), and returns their
// concatenated contents with headers.
//
// maxFileSizeArg is set via tool options by the config author (KB).
func (e *templateEngine) fileRead(pathGlob string, maxKB int) string {
	if e.fsys == nil {
		return "file_read: no filesystem available"
	}
	maxSize := int64(maxKB) * 1024

	var all []string
	if ms, err := globFS(e.fsys, pathGlob); err == nil && len(ms) > 0 {
		all = append(all, ms...)
	} else if _, err := fs.Stat(e.fsys, pathGlob); err == nil {
		all = append(all, pathGlob)
	}

	if len(all) == 0 {
		return "No files found."
	}

	var b strings.Builder
	for i, f := range all {
		data, err := fs.ReadFile(e.fsys, f)
		if err != nil {
			fmt.Fprintf(&b, "=== %s ===\n(warning: %v)\n\n", f, err)
			continue
		}
		content := string(data)
		trunc := false
		if int64(len(content)) > maxSize {
			content = content[:maxSize]
			trunc = true
		}
		if len(all) > 1 {
			fmt.Fprintf(&b, "=== %s ===\n", f)
		}
		b.WriteString(content)
		if trunc {
			fmt.Fprintf(&b, "\n... (truncated at %dKB)", maxKB)
		}
		if i < len(all)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// httpFetch performs an authenticated HTTP GET, strips ANSI codes, and
// returns the body as a string. Applies .netrc credentials automatically.
// Redirects are never followed — a 3xx response is returned as an error
// that includes the redirect destination from the Location header.
// When allowedHosts is non-nil the request URL must match one of the
// entries; otherwise the call is rejected before any network I/O.
func httpFetch(ctx context.Context, rawURL string, maxRead int64, allowedHosts map[string]bool) (string, error) {
	if err := urlAllowedByList(rawURL, allowedHosts); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	auth.ApplyNetrcAuth(req)
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			return "", fmt.Errorf("%s redirected to %s (%s) — update the URL", rawURL, location, resp.Status)
		}
		return "", fmt.Errorf("%s returned %s", rawURL, resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned %s", rawURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRead))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	return ansiRe.ReplaceAllString(string(data), ""), nil
}

// httpSend performs an authenticated HTTP POST or PUT, strips ANSI codes,
// and returns the body as a string. Applies .netrc credentials automatically.
// Any 2xx response is treated as success. Redirects are never followed.
// When allowedHosts is non-nil the request URL must match one of the
// entries; otherwise the call is rejected before any network I/O.
func httpSend(ctx context.Context, method, rawURL, body, contentType string, maxRead int64, allowedHosts map[string]bool) (string, error) {
	if err := urlAllowedByList(rawURL, allowedHosts); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	auth.ApplyNetrcAuth(req)
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", method, rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			return "", fmt.Errorf("%s redirected to %s (%s) — update the URL", rawURL, location, resp.Status)
		}
		return "", fmt.Errorf("%s returned %s", rawURL, resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s returned %s", rawURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRead))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	return ansiRe.ReplaceAllString(string(data), ""), nil
}

// grepFunc implements the grep template function.
// It performs a regex match with context lines on the input string.
// Falls back to literal match when the pattern is not valid regex.
func grepFunc(pattern string, before, after float64, input string) string {
	if input == "" {
		return ""
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		re = regexp.MustCompile(regexp.QuoteMeta(pattern))
	}
	b, a := int(before), int(after)
	if b < 0 {
		b = 0
	}
	if a < 0 {
		a = 0
	}

	lines := strings.Split(input, "\n")

	var matches []int
	for i, line := range lines {
		if re.MatchString(line) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return fmt.Sprintf("No lines matching %q found (%d lines searched).", pattern, len(lines))
	}

	type span struct{ start, end int }
	spans := make([]span, 0, len(matches))
	for _, m := range matches {
		start := max(m-b, 0)
		end := min(m+a+1, len(lines))
		spans = append(spans, span{start, end})
	}

	merged := []span{spans[0]}
	for _, sp := range spans[1:] {
		last := &merged[len(merged)-1]
		if sp.start <= last.end {
			if sp.end > last.end {
				last.end = sp.end
			}
		} else {
			merged = append(merged, sp)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d matches for %q\n\n", len(matches), pattern)
	for i, sp := range merged {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		for j := sp.start; j < sp.end; j++ {
			sb.WriteString(lines[j])
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
