// Package sources provides content acquisition from local files, git
// repositories, and web pages. Each source type returns []RawDoc;
// chunking and indexing are applied by the sources orchestrator.
package sources

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/glob"
)

// RawDoc is a fetched document before chunking/indexing.
// Sources return RawDocs; the orchestrator applies chunking later.
type RawDoc struct {
	// Source identifies the origin (relative path, URL, or repo:path).
	Source string
	// Content is the raw content of the document.
	Content string
}

// Source is the interface satisfied by every content source.
// Fetch acquires remote content to disk (no-op for local sources);
// Read returns matched documents from the source content.
type Source interface {
	Fetch(ctx context.Context) error
	Read(globs []string, prefix string) ([]RawDoc, error)
}

// skipFetch returns true when the resolved pull policy says the source
// should not be re-fetched.
func skipFetch(policy config.PullPolicy, destDir string) bool {
	switch policy {
	case config.PullPolicyNever:
		return true
	case config.PullPolicyAlways:
		return false
	default: // ifNotPresent (or empty, which defaults to ifNotPresent)
		_, err := os.Stat(destDir)
		return err == nil
	}
}

// resolvePolicy returns the per-source policy when set, otherwise the global fallback.
func resolvePolicy(perSource, global config.PullPolicy) config.PullPolicy {
	if perSource != "" {
		return perSource
	}
	return global
}

// SourceMeta carries per-source metadata used by the orchestrator
// to read and index content after Fetch.
type SourceMeta struct {
	NoIndex    bool     // when true, skip indexing for this source
	ReadGlobs  []string // glob patterns for content files within the source FS
	ReadPrefix string   // prefix prepended to file paths for attribution
}

// Factory creates a Source and its metadata from kind-specific configuration.
// name is the user-chosen label for the source entry.
// raw is the kind-specific config struct (e.g. config.LocalSource for "local").
// projectRoot is the workspace root; baseDir is the cache directory for
// fetched remote sources.
// global is the project-wide PullPolicy fallback.
type Factory func(name string, raw any, projectRoot, baseDir string, global config.PullPolicy) (Source, SourceMeta, error)

// Registry holds source factories keyed by kind (e.g. "local", "git").
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// Register adds (or replaces) the source factory for kind.
func (r *Registry) Register(kind string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories == nil {
		r.factories = make(map[string]Factory)
	}
	r.factories[kind] = f
}

// Lookup returns the factory registered for kind.
func (r *Registry) Lookup(kind string) (Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[kind]
	return f, ok
}

// DefaultRegistry is the package-level registry populated by init() functions
// in each source file (local.go, git.go, etc.).
var DefaultRegistry = &Registry{}

// readFS reads all files matching the given globs from fsys and returns
// their content as RawDocs. If prefix is non-empty it is prepended to each
// Source as "<prefix>:<rel-path>" for attribution.
func readFS(fsys fs.FS, globs []string, prefix string) ([]RawDoc, error) {
	files, err := matchFiles(fsys, globs)
	if err != nil {
		return nil, err
	}

	var docs []RawDoc
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			continue
		}
		src := f
		if prefix != "" {
			src = filepath.Join(prefix, f)
		}
		docs = append(docs, RawDoc{Source: src, Content: string(data)})
	}
	return docs, nil
}

// matchFiles returns relative paths of files matching the given globs within fsys.
func matchFiles(fsys fs.FS, globs []string) ([]string, error) {
	var files []string
	for _, g := range globs {
		matches, err := globOne(fsys, g)
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	return files, nil
}

// globOne resolves a single glob pattern against fsys, using glob.WalkFS for ** patterns.
func globOne(fsys fs.FS, g string) ([]string, error) {
	if strings.Contains(g, "**") {
		return glob.WalkFS(fsys, g)
	}
	return fs.Glob(fsys, g)
}
