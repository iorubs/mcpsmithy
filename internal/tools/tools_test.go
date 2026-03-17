package tools

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/conventions"
	"github.com/operator-assistant/mcpsmithy/internal/project"
)

func testEngine(t *testing.T, cfg *config.Config) *Engine {
	t.Helper()
	dir := t.TempDir()
	eng, err := New(context.Background(), cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

// testEngineFS builds an Engine backed by an fs.FS (e.g. fstest.MapFS)
// instead of a real directory. Avoids disk I/O in tests.
func testEngineFS(t *testing.T, cfg *config.Config, fsys fs.FS) *Engine {
	t.Helper()
	sb := &sandbox{root: "/project", fsys: fsys}
	idx, _ := project.Build(context.Background(), cfg, sb.Root(), project.BuildOptions{})
	convIdx := conventions.BuildIndex(context.Background(), cfg.Conventions)
	tpl := newTemplateEngine(cfg, sb.Root(), cfg.Conventions, idx, convIdx, sb.fsys)

	e := &Engine{
		cfg:      cfg,
		sb:       sb,
		handlers: make(map[string]handler),
		tools:    cfg.Tools,
		tpl:      tpl,
	}

	for name, tool := range cfg.Tools {
		h, err := e.templateHandler(name, tool)
		if err != nil {
			t.Fatalf("tool %q: %v", name, err)
		}
		e.handlers[name] = h
	}
	return e
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		tools   map[string]config.Tool
		wantErr bool
		wantMsg string
		check   func(*testing.T, *Engine)
	}{
		{
			name: "registers tools",
			tools: map[string]config.Tool{
				"t1": {Description: "first", Template: "hello"},
				"t2": {Description: "second", Template: "world"},
			},
			check: func(t *testing.T, eng *Engine) {
				if len(eng.Tools()) != 2 {
					t.Fatalf("expected 2 tools, got %d", len(eng.Tools()))
				}
			},
		},
		{
			name:    "unknown function in template",
			tools:   map[string]config.Tool{"bad": {Description: "bad", Template: "{{ nonexistent }}"}},
			wantErr: true,
			wantMsg: "not defined",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Version: "1",
				Project: config.Project{Name: "test"},
				Tools:   tt.tools,
			}
			dir := t.TempDir()
			eng, err := New(context.Background(), cfg, dir)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
					t.Fatalf("expected %q in error, got: %v", tt.wantMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, eng)
			}
		})
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Project: config.Project{Name: "test"},
	}
	eng := testEngine(t, cfg)
	_, err := eng.Execute(context.Background(), "missing", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("expected 'unknown tool' in error, got: %v", err)
	}
}

func TestExecuteTemplate(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Project: config.Project{Name: "myproject"},
		Tools: map[string]config.Tool{
			"greet": {
				Description: "greets",
				Template:    "Hello from {{ .mcpsmithy.Project.Name }}",
			},
		},
	}
	eng := testEngine(t, cfg)
	out, err := eng.Execute(context.Background(), "greet", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hello from myproject") {
		t.Fatalf("expected project name in output, got: %s", out)
	}
}

func TestExecuteTemplateWithParams(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Project: config.Project{Name: "test"},
		Tools: map[string]config.Tool{
			"echo": {
				Description: "echoes",
				Template:    "msg={{ .msg }}",
				Params: []config.ToolParam{
					{Name: "msg", Type: config.ParamTypeString, Required: true},
				},
			},
		},
	}
	eng := testEngine(t, cfg)

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
		wantOut string
	}{
		{"missing required param", nil, true, ""},
		{"with param provided", map[string]any{"msg": "hi"}, false, "msg=hi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := eng.Execute(context.Background(), "echo", tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if out != tt.wantOut {
				t.Fatalf("expected %q, got %q", tt.wantOut, out)
			}
		})
	}
}

func TestExecuteParamDefault(t *testing.T) {
	min, max := 0.0, 100.0
	tests := []struct {
		name    string
		tool    config.Tool
		wantOut string
	}{
		{
			name: "string default",
			tool: config.Tool{
				Template: "Hello {{ .name }}",
				Params:   []config.ToolParam{{Name: "name", Type: config.ParamTypeString, Default: "world"}},
			},
			wantOut: "Hello world",
		},
		{
			// A number param with a default and min/max should not fail validation
			// when the LLM omits the parameter.
			name: "number with constraints",
			tool: config.Tool{
				Template: "count={{ .n }}",
				Params: []config.ToolParam{{
					Name:        "n",
					Type:        config.ParamTypeNumber,
					Default:     10,
					Constraints: &config.ParamConstraints{Min: &min, Max: &max},
				}},
			},
			wantOut: "count=10",
		},
		{
			name: "bool default",
			tool: config.Tool{
				Template: "verbose={{ .verbose }}",
				Params:   []config.ToolParam{{Name: "verbose", Type: config.ParamTypeBool, Default: true}},
			},
			wantOut: "verbose=true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Version: "1",
				Project: config.Project{Name: "test"},
				Tools:   map[string]config.Tool{"tool": tt.tool},
			}
			eng := testEngine(t, cfg)
			out, err := eng.Execute(context.Background(), "tool", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tt.wantOut) {
				t.Fatalf("expected %q, got: %s", tt.wantOut, out)
			}
		})
	}
}

func TestZeroForType(t *testing.T) {
	tests := []struct {
		name string
		typ  config.ParamType
		want any
	}{
		{"string", config.ParamTypeString, ""},
		{"number", config.ParamTypeNumber, 0.0},
		{"bool", config.ParamTypeBool, false},
		{"project_file_path", config.ParamTypeProjectFilePath, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zeroForType(tt.typ)
			if got != tt.want {
				t.Errorf("zeroForType(%q) = %v (%T); want %v (%T)", tt.typ, got, got, tt.want, tt.want)
			}
		})
	}
}

// TestExecuteGrepWithFloat64Params verifies that number parameters supplied as
// float64 pass directly to grep without any coercion.
func TestExecuteGrepWithFloat64Params(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Project: config.Project{Name: "test"},
		Tools: map[string]config.Tool{
			"ci_log": {
				Description: "test",
				Template:    `{{ grep .pattern .before .after .input }}`,
				Params: []config.ToolParam{
					{Name: "pattern", Type: config.ParamTypeString, Required: true},
					{Name: "before", Type: config.ParamTypeNumber},
					{Name: "after", Type: config.ParamTypeNumber},
					{Name: "input", Type: config.ParamTypeString},
				},
			},
		},
	}
	eng := testEngine(t, cfg)
	// before/after supplied as float64 — exactly what JSON unmarshalling produces.
	out, err := eng.Execute(context.Background(), "ci_log", map[string]any{
		"pattern": "TARGET",
		"before":  float64(1),
		"after":   float64(1),
		"input":   "line1\nTARGET\nline3\n",
	})
	if err != nil {
		t.Fatalf("Execute with float64 number params failed: %v", err)
	}
	if !strings.Contains(out, "TARGET") {
		t.Errorf("expected output to contain 'TARGET', got: %q", out)
	}
}

// --- file_read function tests ---

func TestFileRead(t *testing.T) {
	tests := []struct {
		name  string
		fsys  fs.FS
		tool  config.Tool
		args  map[string]any
		check func(*testing.T, string)
	}{
		{
			name: "single file",
			fsys: fstest.MapFS{"readme.md": {Data: []byte("# Hello World\n")}},
			tool: config.Tool{
				Description: "read a file",
				Template:    "{{ file_read .path .maxFileSize }}",
				Params:      []config.ToolParam{{Name: "path", Type: config.ParamTypeString, Required: true}},
				Options:     map[string]any{"maxFileSize": 50},
			},
			args: map[string]any{"path": "readme.md"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "# Hello World") {
					t.Fatalf("expected file content, got: %s", out)
				}
			},
		},
		{
			name: "glob pattern",
			fsys: fstest.MapFS{
				"docs/a.md": {Data: []byte("doc A")},
				"docs/b.md": {Data: []byte("doc B")},
			},
			tool: config.Tool{
				Template: `{{ file_read "docs/*.md" }}`,
			},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "doc A") || !strings.Contains(out, "doc B") {
					t.Fatalf("expected both docs, got: %s", out)
				}
			},
		},
		{
			name: "no files found",
			fsys: fstest.MapFS{},
			tool: config.Tool{
				Description: "read missing",
				Template:    "{{ file_read .path .maxFileSize }}",
				Params:      []config.ToolParam{{Name: "path", Type: config.ParamTypeString, Required: true}},
				Options:     map[string]any{"maxFileSize": 50},
			},
			args: map[string]any{"path": "nonexistent/*.txt"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "No files found") {
					t.Fatalf("expected 'No files found', got: %s", out)
				}
			},
		},
		{
			name: "truncates oversized file",
			fsys: fstest.MapFS{"big.txt": {Data: []byte(strings.Repeat("x", 2048))}},
			tool: config.Tool{
				Description: "read big",
				Template:    "{{ file_read .path .maxFileSize }}",
				Params:      []config.ToolParam{{Name: "path", Type: config.ParamTypeString, Required: true}},
				Options:     map[string]any{"maxFileSize": 1},
			},
			args: map[string]any{"path": "big.txt"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "truncated at 1KB") {
					t.Fatalf("expected truncation marker, got: %s", out)
				}
				if len(out) > 1200 {
					t.Fatalf("output should be capped near 1KB, got %d bytes", len(out))
				}
			},
		},
		{
			name: "multi-file headers",
			fsys: fstest.MapFS{
				"docs/a.md": {Data: []byte("alpha")},
				"docs/b.md": {Data: []byte("beta")},
			},
			tool: config.Tool{
				Template: `{{ file_read "docs/*.md" }}`,
			},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "=== docs/a.md ===") || !strings.Contains(out, "=== docs/b.md ===") {
					t.Fatalf("expected file headers for multi-file output, got: %s", out)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Version: "1",
				Project: config.Project{Name: "test"},
				Tools:   map[string]config.Tool{"tool": tt.tool},
			}
			eng := testEngineFS(t, cfg, tt.fsys)
			out, err := eng.Execute(context.Background(), "tool", tt.args)
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, out)
		})
	}
}

func TestConventionsContextTemplateTool(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Project: config.Project{Name: "test"},
		Conventions: map[string]config.Convention{
			"global": {Scope: "*", Description: "Global rules"},
		},
		Tools: map[string]config.Tool{
			"browse": {
				Description: "browse all conventions",
				Template:    `{{ range $k, $v := .mcpsmithy.Conventions }}{{ $k }}: {{ $v.Description }}{{ end }}`,
			},
		},
	}
	eng := testEngine(t, cfg)
	out, err := eng.Execute(context.Background(), "browse", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "global") {
		t.Fatalf("expected conventions output, got: %s", out)
	}
}

func TestProjectContextTemplateTool(t *testing.T) {
	cfg := &config.Config{
		Version: "1",
		Project: config.Project{
			Name:        "my-tool",
			Description: "A CLI tool",
			Sources: &config.ProjectSources{
				Local: map[string]config.LocalSource{
					"src": {Paths: []string{"cmd/**/*.go"}, Description: "Source code"},
				},
			},
		},
		Conventions: map[string]config.Convention{
			"style": {Scope: "*", Description: "Code style"},
		},
		Tools: map[string]config.Tool{
			"info": {
				Description: "project info",
				Template:    `{{ .mcpsmithy.Project }}`,
			},
		},
	}
	eng := testEngine(t, cfg)
	out, err := eng.Execute(context.Background(), "info", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "my-tool") {
		t.Fatalf("should contain project name, got: %s", out)
	}
}

// ----- validateConstraint -----

func TestValidateConstraint(t *testing.T) {
	min, max := 1.0, 100.0
	enumParam := config.ToolParam{
		Name: "env", Type: config.ParamTypeString,
		Constraints: &config.ParamConstraints{Enum: []any{"dev", "staging", "prod"}},
	}
	intEnumParam := config.ToolParam{
		Name: "limit", Type: config.ParamTypeNumber,
		Constraints: &config.ParamConstraints{Enum: []any{1, 50, 100}},
	}
	tests := []struct {
		name    string
		param   config.ToolParam
		value   any
		wantErr bool
	}{
		{"enum string valid", enumParam, "dev", false},
		{"enum string invalid", enumParam, "local", true},
		{
			"min-max valid",
			config.ToolParam{Name: "count", Type: config.ParamTypeNumber, Constraints: &config.ParamConstraints{Min: &min, Max: &max}},
			50, false,
		},
		{
			"below min",
			config.ToolParam{Name: "count", Type: config.ParamTypeNumber, Constraints: &config.ParamConstraints{Min: &min}},
			0, true,
		},
		{
			"above max",
			config.ToolParam{Name: "count", Type: config.ParamTypeNumber, Constraints: &config.ParamConstraints{Max: &max}},
			200, true,
		},
		{"nil constraints", config.ToolParam{Name: "x", Type: config.ParamTypeString}, "anything", false},
		{"enum int valid", intEnumParam, 50, false},
		{"enum int invalid", intEnumParam, 42, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConstraint(tt.param, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else if err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}
