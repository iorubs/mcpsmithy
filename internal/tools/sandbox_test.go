package tools

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	sb, err := newSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"inside", "foo/bar.txt", false},
		{"outside", "../../etc/passwd", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sb.Resolve(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(sb.Root(), tt.path)
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTrim bool
	}{
		{"short", "hello", false},
		{"at limit", strings.Repeat("a", 1024*1024), false},
		{"over limit", strings.Repeat("a", 1024*1024+100), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateOutput(tt.input, 1024*1024)
			if tt.wantTrim {
				if len(got) > 1024*1024+50 {
					t.Errorf("output not truncated: len=%d", len(got))
				}
				if !strings.Contains(got, "truncated") {
					t.Error("expected truncation marker")
				}
			} else {
				if got != tt.input {
					t.Errorf("expected unchanged output")
				}
			}
		})
	}
}

func TestRoot(t *testing.T) {
	dir := t.TempDir()
	sb, err := newSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sb.Root() == "" {
		t.Error("root should not be empty")
	}
}

func TestSandboxFSReadsFileInsideRoot(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "hello.txt"), []byte("sandbox content"), 0o644); err != nil {
		t.Fatal(err)
	}

	sb, err := newSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Read through the sandbox's FS.
	data, err := fs.ReadFile(sb.fsys, "sub/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "sandbox content" {
		t.Errorf("got %q, want %q", data, "sandbox content")
	}
}

func TestSandboxValidateFilePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	sb, err := newSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"existing file", "exists.txt", false},
		{"missing file", "missing.txt", true},
		{"path outside sandbox", "../../etc/passwd", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sb.ValidateFilePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGlobFS(t *testing.T) {
	stdFS := fstest.MapFS{
		"docs/a.md":   {Data: []byte("a")},
		"docs/b.md":   {Data: []byte("b")},
		"docs/c.txt":  {Data: []byte("c")},
		"src/main.go": {Data: []byte("go")},
	}
	deepFS := fstest.MapFS{
		"docs/guide/intro.md": {Data: []byte("intro")},
		"docs/guide/setup.md": {Data: []byte("setup")},
	}
	multiSegFS := fstest.MapFS{
		"config/v1/types.go":   {Data: []byte("go")},
		"config/v1/parse.go":   {Data: []byte("go")},
		"config/v2/types.go":   {Data: []byte("go")},
		"config/v1/testdata/x": {Data: []byte("x")},
		"other/root.go":        {Data: []byte("go")},
	}
	tests := []struct {
		name    string
		fsys    fstest.MapFS
		pattern string
		want    int
	}{
		{"standard glob", stdFS, "docs/*.md", 2},
		{"double-star glob", stdFS, "**/*.go", 1},
		{"deep double-star", deepFS, "docs/**", 2},
		{"multi-segment **/v1/*.go", multiSegFS, "**/v1/*.go", 2},
		{"prefix config/**/v1/*.go", multiSegFS, "config/**/v1/*.go", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms, err := globFS(tt.fsys, tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if len(ms) != tt.want {
				t.Fatalf("expected %d matches for %q, got %d: %v", tt.want, tt.pattern, len(ms), ms)
			}
		})
	}
}

func TestNewSandboxFS(t *testing.T) {
	fsys := fstest.MapFS{
		"src/main.go": {Data: []byte("package main")},
	}
	sb := &sandbox{root: "/project", fsys: fsys}
	if sb.Root() != "/project" {
		t.Errorf("expected root /project, got %q", sb.Root())
	}
	// Read through the sandbox's FS.
	data, err := fs.ReadFile(sb.fsys, "src/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package main" {
		t.Errorf("got %q, want %q", data, "package main")
	}
}

func TestSandboxFSValidateFilePath(t *testing.T) {
	fsys := fstest.MapFS{
		"exists.txt": {Data: []byte("ok")},
	}
	sb := &sandbox{root: "/project", fsys: fsys}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"existing file", "exists.txt", false},
		{"missing file", "missing.txt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sb.ValidateFilePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
