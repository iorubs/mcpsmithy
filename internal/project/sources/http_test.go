package sources

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarGz(t *testing.T) {
	// Build an in-memory tar.gz with a top-level wrapper directory.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	files := map[string]string{
		"repo-main/README.md":        "# Hello",
		"repo-main/docs/guide.md":    "## Guide",
		"repo-main/docs/advanced.md": "## Advanced",
	}

	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()

	destDir := t.TempDir()
	if err := extractTarGz(&buf, destDir); err != nil {
		t.Fatal(err)
	}

	// Verify all files were extracted with the wrapper directory stripped.
	for name, wantContent := range files {
		// Strip "repo-main/" prefix
		relPath := name[len("repo-main/"):]
		fullPath := filepath.Join(destDir, relPath)

		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("file %s not found: %v", relPath, err)
			continue
		}
		if string(data) != wantContent {
			t.Errorf("file %s: got %q, want %q", relPath, string(data), wantContent)
		}
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	// Attempt a path traversal — should be silently skipped.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{
		Name:     "repo-main/../../etc/passwd",
		Mode:     0o644,
		Size:     6,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte("hacked"))
	tw.Close()
	gz.Close()

	destDir := t.TempDir()
	if err := extractTarGz(&buf, destDir); err != nil {
		t.Fatal(err)
	}

	// The traversal file should NOT exist outside destDir.
	if _, err := os.Stat(filepath.Join(destDir, "..", "etc", "passwd")); err == nil {
		t.Error("path traversal file should not have been extracted")
	}
}

func TestDetectArchive(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		contentType string
		want        bool
	}{
		{"tar.gz suffix", "https://example.com/repo/archive/main.tar.gz", "", true},
		{"tgz suffix", "https://example.com/repo/archive/main.tgz", "", true},
		{"gzip content type", "https://example.com/api/download", "application/gzip", true},
		{"x-gzip content type", "https://example.com/api/download", "application/x-gzip", true},
		{"x-tar content type", "https://example.com/api/download", "application/x-tar", true},
		{"plain text", "https://example.com/readme.md", "text/plain", false},
		{"json api", "https://api.example.com/data", "application/json", false},
		{"no hints", "https://example.com/download", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectArchive(tt.url, tt.contentType)
			if got != tt.want {
				t.Errorf("detectArchive(%q, %q) = %v; want %v", tt.url, tt.contentType, got, tt.want)
			}
		})
	}
}

func TestSaveRawFile(t *testing.T) {
	destDir := t.TempDir()
	content := "hello world"

	err := saveRawFile(bytes.NewReader([]byte(content)), "https://example.com/readme.md?token=abc", destDir)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "readme.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

func TestSaveRawFile_FallbackFilename(t *testing.T) {
	// A rawURL whose filepath.Base resolves to "/" must fall back to "index".
	destDir := t.TempDir()
	if err := saveRawFile(bytes.NewReader([]byte("data")), "/", destDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "index")); err != nil {
		t.Errorf("expected fallback file \"index\", got error: %v", err)
	}
}

func TestHTTPSourceFetch(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		contentType string
		wantErr     bool
		wantFile    string // relative path under destDir that should exist
	}{
		{
			name: "raw text file",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprint(w, "# Hello")
			},
			wantFile: "file.txt",
		},
		{
			name: "non-200 status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			destDir := t.TempDir()
			src := &HTTPSource{
				rawURL:  srv.URL + "/file.txt",
				destDir: destDir,
				policy:  "always",
			}

			err := src.Fetch(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Fetch() error = %v; wantErr %v", err, tt.wantErr)
			}
			if tt.wantFile != "" {
				if _, statErr := os.Stat(filepath.Join(destDir, tt.wantFile)); statErr != nil {
					t.Errorf("expected file %q: %v", tt.wantFile, statErr)
				}
			}
		})
	}
}

func TestHTTPSourceFetch_SkipIfNotPresent(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		fmt.Fprint(w, "should not be fetched")
	}))
	defer srv.Close()

	destDir := t.TempDir()
	src := &HTTPSource{
		rawURL:  srv.URL + "/data.md",
		destDir: destDir,
		policy:  "ifNotPresent",
	}
	if err := src.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("expected Fetch to skip the HTTP request when destDir exists and policy is ifNotPresent")
	}
}
