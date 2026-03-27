package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/search"
)

func TestConventionsForIncludesDescriptions(t *testing.T) {
	cfg := &config.Config{
		Conventions: map[string]config.Convention{
			"global":      {Scope: "*", Description: "Universal convention", Docs: []config.DocRef{{Source: "docs", Paths: []string{"docs/README.md"}}}},
			"controllers": {Scope: "internal/controller/**", Description: "Controller convention", Docs: []config.DocRef{{Source: "docs", Paths: []string{"docs/controllers.md"}}}},
			"api":         {Scope: "api/**", Description: "API convention", Docs: []config.DocRef{{Source: "docs", Paths: []string{"docs/api.md"}}}},
			"search-only": {Description: "Search only convention", Docs: []config.DocRef{{Source: "docs", Paths: []string{"docs/search.md"}}}},
		},
	}
	e := newTemplateEngine(cfg, "/project", cfg.Conventions, nil, nil, nil)
	fm := e.funcMap(nil)
	fn := fm["conventions_for"].(func(string) []config.Convention)
	result := fn("internal/controller/foo.go")
	hasDesc := func(desc string) bool {
		for _, c := range result {
			if c.Description == desc {
				return true
			}
		}
		return false
	}
	hasDocPath := func(path string) bool {
		for _, c := range result {
			for _, d := range c.Docs {
				if slices.Contains(d.Paths, path) {
					return true
				}
			}
		}
		return false
	}
	if !hasDesc("Universal convention") {
		t.Fatal("should include catch-all convention")
	}
	if !hasDesc("Controller convention") {
		t.Fatal("should include controller convention")
	}
	if !hasDocPath("docs/controllers.md") {
		t.Fatal("should include controller docs")
	}
	if hasDesc("API convention") {
		t.Fatal("should not include api convention")
	}
	if hasDesc("Search only convention") {
		t.Fatal("search-only convention (no scope) should not be returned")
	}
}

func TestConventionsForIncludesRelations(t *testing.T) {
	cfg := &config.Config{
		Conventions: map[string]config.Convention{
			"controllers": {
				Scope:       "internal/controller/**",
				Description: "Controller convention",
				Relations: []config.ConventionRelations{
					{Target: "crd-api", Description: "Controllers reconcile CRD types"},
				},
			},
		},
	}
	e := newTemplateEngine(cfg, "/project", cfg.Conventions, nil, nil, nil)
	fm := e.funcMap(nil)
	fn := fm["conventions_for"].(func(string) []config.Convention)
	result := fn("internal/controller/foo.go")
	if len(result) == 0 {
		t.Fatal("expected at least one convention")
	}
	var rel *config.ConventionRelations
	for _, c := range result {
		for i := range c.Relations {
			if c.Relations[i].Target == "crd-api" {
				rel = &c.Relations[i]
			}
		}
	}
	if rel == nil {
		t.Fatal("should include relation with target crd-api")
	}
	if rel.Description != "Controllers reconcile CRD types" {
		t.Fatalf("unexpected relation description: %q", rel.Description)
	}
}

func TestSourceMethod(t *testing.T) {
	cfgWithSources := &config.Config{
		Project: config.Project{
			Sources: &config.ProjectSources{
				Local:  map[string]config.LocalSource{"code": {Paths: []string{"cmd/**/*.go", "internal/**/*.go"}}},
				Git:    map[string]config.GitSource{"external": {Repo: "https://github.com/org/repo", Paths: []string{"docs/**/*.md"}}},
				Scrape: map[string]config.ScrapeSource{"api-ref": {URLs: []string{"https://api.example.com/docs"}}},
			},
		},
	}
	tests := []struct {
		name   string
		cfg    *config.Config
		source string
		want   string
	}{
		{"local source", cfgWithSources, "code", "cmd/**/*.go"},
		{"git source", cfgWithSources, "external", "github.com/org/repo"},
		{"scrape source", cfgWithSources, "api-ref", "api.example.com"},
		{"unknown source", cfgWithSources, "missing", "Unknown source"},
		{"no sources configured", &config.Config{}, "anything", "No sources configured"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTemplateEngine(tt.cfg, "/project", nil, nil, nil, nil)
			ctx := e.Context(nil)
			mctx := ctx[config.ReservedContextKey].(mcpsmithyContext)
			result := mctx.Source(tt.source)
			if !strings.Contains(result, tt.want) {
				t.Fatalf("expected %q in result, got: %s", tt.want, result)
			}
		})
	}
}

func TestSearchFor(t *testing.T) {
	sourceChunks := []search.Chunk{
		{Source: "b.md", Title: "Docker Setup", Body: "Install docker on your machine."},
	}
	convChunks := []search.Chunk{
		{Source: "internal/k8s/**", Title: "k8s-overview", Body: "Kubernetes pods and deployments.", Tags: []string{"k8s"}, ConventionID: "k8s-overview", Docs: []string{"docs/k8s.md"}},
	}
	noopCfg := &config.Config{}
	cfgWithTool := &config.Config{
		Tools: map[string]config.Tool{
			"get_convention": {Template: `{{ index .mcpsmithy.Conventions .name }}`},
		},
	}
	tests := []struct {
		name    string
		cfg     *config.Config
		idxMgr  search.Searcher
		convIdx search.Searcher
		query   string
		check   func(*testing.T, string)
	}{
		{
			name:    "finds conventions and shows generic tip",
			cfg:     noopCfg,
			idxMgr:  search.NewIndex(sourceChunks),
			convIdx: search.NewIndex(convChunks),
			query:   "kubernetes",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "Matching conventions") {
					t.Fatal("should contain conventions section header")
				}
				if !strings.Contains(result, "k8s-overview") {
					t.Fatal("should contain convention name")
				}
				if !strings.Contains(result, "Docs: docs/k8s.md") {
					t.Fatal("should contain convention docs")
				}
				if !strings.Contains(result, "Tip: Look up a convention by name") {
					t.Fatal("should contain generic convention tip")
				}
			},
		},
		{
			name:    "uses specific tool name in tip when get_convention exists",
			cfg:     cfgWithTool,
			idxMgr:  search.NewIndex(sourceChunks),
			convIdx: search.NewIndex(convChunks),
			query:   "kubernetes",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "Tip: Use get_convention with") {
					t.Fatal("should reference discovered tool name, got: " + result)
				}
			},
		},
		{
			name:    "no results",
			cfg:     noopCfg,
			idxMgr:  search.NewIndex([]search.Chunk{{Source: "a.md", Title: "Hello", Body: "World"}}),
			convIdx: nil,
			query:   "nonexistent",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "No results") {
					t.Fatal("should indicate no results")
				}
			},
		},
		{
			name:   "no index",
			cfg:    noopCfg,
			idxMgr: nil,
			query:  "anything",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "No search index") {
					t.Fatal("should indicate no index")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTemplateEngine(tt.cfg, "/project", nil, tt.idxMgr, tt.convIdx, nil)
			fn := e.funcMap(nil)["search_for"].(func(string, ...any) string)
			result := fn(tt.query, 10, 200)
			tt.check(t, result)
		})
	}
}

func TestGrepFunc(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		before  float64
		after   float64
		input   string
		check   func(*testing.T, string)
	}{
		{
			name:    "no match",
			pattern: "missing",
			input:   "hello\nworld\n",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "No lines matching") {
					t.Fatalf("expected no-match message, got: %s", result)
				}
			},
		},
		{
			name:    "single match no context",
			pattern: "bbb",
			input:   "aaa\nbbb\nccc\n",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "1 matches") {
					t.Fatalf("expected 1 match, got: %s", result)
				}
				if !strings.Contains(result, "bbb") {
					t.Fatalf("expected matching line, got: %s", result)
				}
				if strings.Contains(result, "aaa") || strings.Contains(result, "ccc") {
					t.Fatalf("should not include context lines, got: %s", result)
				}
			},
		},
		{
			name:    "match with context",
			pattern: "TARGET",
			before:  1,
			after:   1,
			input:   "line1\nline2\nTARGET\nline4\nline5\n",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "line2") || !strings.Contains(result, "line4") {
					t.Fatalf("expected context lines, got: %s", result)
				}
			},
		},
		{
			name:    "invalid regex falls back to literal",
			pattern: "a[b",
			input:   "a[b\nc[d\n",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "1 matches") {
					t.Fatalf("expected literal fallback match, got: %s", result)
				}
			},
		},
		{
			name:    "empty input",
			pattern: "x",
			input:   "",
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Fatalf("expected empty result, got: %s", result)
				}
			},
		},
		{
			name:    "multiple matches with separator",
			pattern: "TARGET",
			input:   "a\nTARGET1\nb\nc\nd\nTARGET2\ne\n",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "2 matches") {
					t.Fatalf("expected 2 matches, got: %s", result)
				}
				if !strings.Contains(result, "---") {
					t.Fatalf("expected separator between groups, got: %s", result)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := grepFunc(tt.pattern, tt.before, tt.after, tt.input)
			tt.check(t, result)
		})
	}
}

func TestHTTPGetFuncMap(t *testing.T) {
	// Verify http_get, http_post, and http_put are registered in the func map.
	cfg := &config.Config{}
	e := newTemplateEngine(cfg, "/project", nil, nil, nil, nil)
	fm := e.funcMap(nil)
	if _, ok := fm["http_get"]; !ok {
		t.Fatal("http_get should be in func map")
	}
	if _, ok := fm["http_post"]; !ok {
		t.Fatal("http_post should be in func map")
	}
	if _, ok := fm["http_put"]; !ok {
		t.Fatal("http_put should be in func map")
	}
}

func TestHTTPFetch(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantOut string
		wantErr string
	}{
		{
			name: "strips ANSI codes",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("hello \x1b[31mworld\x1b[0m"))
			},
			wantOut: "hello world",
		},
		{
			name: "404 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: "404",
		},
		{
			name: "redirect returns error with location",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://sso.example.com/login")
				w.WriteHeader(http.StatusFound)
			},
			wantErr: "redirected",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			body, err := httpFetch(context.Background(), ts.URL, 10*1024*1024, nil)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %q in error, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if body != tt.wantOut {
				t.Fatalf("expected %q, got: %q", tt.wantOut, body)
			}
		})
	}
}

func TestHTTPSend(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		handler     http.HandlerFunc
		body        string
		contentType string
		wantOut     string
		wantErr     string
	}{
		{
			name:   "POST echoes request body",
			method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				ct := r.Header.Get("Content-Type")
				if ct != "application/json" {
					t.Errorf("expected Content-Type application/json, got %q", ct)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ok":true}`))
			},
			body:        `{"key":"value"}`,
			contentType: "application/json",
			wantOut:     `{"ok":true}`,
		},
		{
			name:   "PUT returns 201 Created",
			method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte("created"))
			},
			body:        "data",
			contentType: "text/plain",
			wantOut:     "created",
		},
		{
			name:   "POST 404 error",
			method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			body:    "{}",
			wantErr: "404",
		},
		{
			name:   "POST redirect returns error",
			method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://sso.example.com/login")
				w.WriteHeader(http.StatusFound)
			},
			body:    "{}",
			wantErr: "redirected",
		},
		{
			name:   "POST strips ANSI codes",
			method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("ok \x1b[32mgreen\x1b[0m"))
			},
			body:    "{}",
			wantOut: "ok green",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			ct := tt.contentType
			if ct == "" {
				ct = "application/json"
			}
			body, err := httpSend(context.Background(), tt.method, ts.URL, tt.body, ct, 10*1024*1024, nil)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %q in error, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if body != tt.wantOut {
				t.Fatalf("expected %q, got: %q", tt.wantOut, body)
			}
		})
	}
}

func TestParseURLAllowList(t *testing.T) {
	tests := []struct {
		name string
		opts map[string]any
		want map[string]bool
	}{
		{
			name: "nil when option absent",
			opts: nil,
			want: nil,
		},
		{
			name: "nil when option is wrong type",
			opts: map[string]any{"urlAllowList": "not-a-list"},
			want: nil,
		},
		{
			name: "parses scheme and host",
			opts: map[string]any{"urlAllowList": []any{
				"https://api.example.com",
				"http://internal.corp:8080",
			}},
			want: map[string]bool{
				"https://api.example.com":   true,
				"http://internal.corp:8080": true,
			},
		},
		{
			name: "lowercases scheme and host",
			opts: map[string]any{"urlAllowList": []any{"HTTPS://API.Example.Com/ignored/path"}},
			want: map[string]bool{"https://api.example.com": true},
		},
		{
			name: "skips empty and invalid entries",
			opts: map[string]any{"urlAllowList": []any{"", "not-a-url", 42, "https://valid.example.com"}},
			want: map[string]bool{"https://valid.example.com": true},
		},
		{
			name: "nil when all entries invalid",
			opts: map[string]any{"urlAllowList": []any{"", "no-host"}},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseURLAllowList(tt.opts)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for k := range tt.want {
				if !got[k] {
					t.Fatalf("missing key %q in %v", k, got)
				}
			}
		})
	}
}

func TestURLAllowedByList(t *testing.T) {
	allowed := map[string]bool{
		"https://api.example.com":   true,
		"http://internal.corp:8080": true,
	}
	tests := []struct {
		name    string
		url     string
		allowed map[string]bool
		wantErr bool
	}{
		{name: "nil list allows everything", url: "https://anywhere.com/foo", allowed: nil},
		{name: "allowed host passes", url: "https://api.example.com/v1/data", allowed: allowed},
		{name: "allowed host with port passes", url: "http://internal.corp:8080/path", allowed: allowed},
		{name: "blocked host rejected", url: "https://evil.com/steal", allowed: allowed, wantErr: true},
		{name: "wrong scheme rejected", url: "http://api.example.com/v1/data", allowed: allowed, wantErr: true},
		{name: "case insensitive match", url: "HTTPS://API.EXAMPLE.COM/v1/data", allowed: allowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := urlAllowedByList(tt.url, tt.allowed)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHTTPFetchAllowList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	// Allowed host — should succeed.
	allowed := map[string]bool{ts.URL: true}
	body, err := httpFetch(context.Background(), ts.URL+"/path", 1024, allowed)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if body != "ok" {
		t.Fatalf("expected %q, got %q", "ok", body)
	}

	// Blocked host — should fail before any network I/O.
	blocked := map[string]bool{"https://other.example.com": true}
	_, err = httpFetch(context.Background(), ts.URL+"/path", 1024, blocked)
	if err == nil {
		t.Fatal("expected error for blocked host")
	}
	if !strings.Contains(err.Error(), "not in the allowed list") {
		t.Fatalf("expected allowlist error, got: %v", err)
	}
}

func TestHTTPSendAllowList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	// Allowed host — should succeed.
	allowed := map[string]bool{ts.URL: true}
	body, err := httpSend(context.Background(), http.MethodPost, ts.URL, "{}", "application/json", 1024, allowed)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if body != "ok" {
		t.Fatalf("expected %q, got %q", "ok", body)
	}

	// Blocked host — should fail before any network I/O.
	blocked := map[string]bool{"https://other.example.com": true}
	_, err = httpSend(context.Background(), http.MethodPost, ts.URL, "{}", "application/json", 1024, blocked)
	if err == nil {
		t.Fatal("expected error for blocked host")
	}
	if !strings.Contains(err.Error(), "not in the allowed list") {
		t.Fatalf("expected allowlist error, got: %v", err)
	}
}
