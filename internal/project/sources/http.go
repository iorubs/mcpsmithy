package sources

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/operator-assistant/mcpsmithy/internal/auth"
	"github.com/operator-assistant/mcpsmithy/internal/config"
)

const sourceKindHTTP = "http"

func init() {
	DefaultRegistry.Register(sourceKindHTTP, func(name string, raw any, _, baseDir string, global config.PullPolicy) (Source, SourceMeta, error) {
		src, ok := raw.(config.HTTPSource)
		if !ok {
			return nil, SourceMeta{}, fmt.Errorf("http source %q: unexpected config type %T", name, raw)
		}
		destDir := filepath.Join(baseDir, sourceKindHTTP, name)
		return &HTTPSource{
				rawURL:  src.URL,
				headers: src.Headers,
				extract: src.Extract,
				destDir: destDir,
				policy:  resolvePolicy(src.PullPolicy, global),
			}, SourceMeta{
				NoIndex:    src.Index != nil && !*src.Index,
				ReadGlobs:  src.Paths,
				ReadPrefix: destDir,
			}, nil
	})
}

// HTTPSource downloads a URL archive (or single file) to disk.
type HTTPSource struct {
	rawURL  string
	headers map[string]string
	extract *bool
	destDir string
	policy  config.PullPolicy
}

// httpSourceClient is the shared HTTP client for HTTP source downloads.
var httpSourceClient = &http.Client{
	Timeout: 2 * time.Minute,
}

// Fetch downloads the URL to disk. When extract is nil, archive
// detection is automatic (Content-Type or .tar.gz/.tgz URL suffix).
// When extract is non-nil its value is used directly.
// Optional headers are added to the request alongside .netrc credentials.
func (s *HTTPSource) Fetch(ctx context.Context) error {
	if skipFetch(s.policy, s.destDir) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.destDir), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(s.destDir); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rawURL, nil)
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}

	auth.ApplyNetrcAuth(req)

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := httpSourceClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", s.rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: %s", s.rawURL, resp.Status)
	}

	shouldExtract := false
	if s.extract != nil {
		shouldExtract = *s.extract
	} else {
		shouldExtract = detectArchive(s.rawURL, resp.Header.Get("Content-Type"))
	}

	if shouldExtract {
		return extractTarGz(resp.Body, s.destDir)
	}
	return saveRawFile(resp.Body, s.rawURL, s.destDir)
}

func (s *HTTPSource) Read(globs []string, prefix string) ([]RawDoc, error) {
	return readFS(os.DirFS(s.destDir), globs, prefix)
}

// detectArchive returns true when the URL or Content-Type indicates a
// gzipped tar archive.
func detectArchive(rawURL, contentType string) bool {
	lower := strings.ToLower(rawURL)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return true
	}
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "application/gzip") ||
		strings.Contains(ct, "application/x-gzip") ||
		strings.Contains(ct, "application/x-tar")
}

// saveRawFile writes the response body as a single file under destDir.
// The filename is derived from the last URL path segment.
func saveRawFile(body io.Reader, rawURL, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	name := filepath.Base(rawURL)
	if name == "" || name == "/" || name == "." {
		name = "index"
	}
	// Strip query parameters from filename.
	if idx := strings.IndexByte(name, '?'); idx >= 0 {
		name = name[:idx]
	}

	target := filepath.Join(destDir, name)
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, body); err != nil {
		return fmt.Errorf("writing file %s: %w", target, err)
	}
	return nil
}

// extractTarGz reads a gzipped tar stream and extracts files to destDir,
// stripping the top-level directory that forges commonly wrap archives in.
func extractTarGz(r io.Reader, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	topDir := "" // detected from first entry

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		name := hdr.Name

		// Detect and strip the top-level wrapper directory.
		if topDir == "" {
			if idx := strings.IndexByte(name, '/'); idx >= 0 {
				topDir = name[:idx+1]
			}
		}
		name = strings.TrimPrefix(name, topDir)
		if name == "" || name == "." {
			continue
		}

		target := filepath.Join(destDir, filepath.FromSlash(name))

		// Security: ensure the path stays within destDir.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("creating parent dir for %s: %w", target, err)
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("writing file %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}
