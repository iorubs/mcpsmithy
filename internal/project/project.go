// Package project orchestrates content acquisition and search
// indexing for the mcpsmithy sources system.
package project

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/operator-assistant/mcpsmithy/internal/config"
	"github.com/operator-assistant/mcpsmithy/internal/project/sources"
	"github.com/operator-assistant/mcpsmithy/internal/search"
)

// sourcesDir is the directory under root where fetched sources are stored.
const sourcesDir = ".mcpsmithy"

// indexManager manages the source pipeline: fetching, indexing, and
// periodic refresh. Use Get() to read the current search.
type indexManager struct {
	idx *search.LiveIndex

	// Retained for refresh loop.
	cfg         *config.Config
	projectRoot string
	reg         *sources.Registry
}

// BuildOptions configures the behaviour of Build.
type BuildOptions struct {
	// SkipIndex skips building the search index; only fetches remote sources
	// to disk. Used by the "sources pull" CLI command.
	SkipIndex bool

	// Registry overrides the source factory registry. When nil the
	// default registry (populated by init) is used. Tests can pass
	// a cloned registry to inject fakes without mutating globals.
	Registry *sources.Registry
}

// sourceEntry represents a single source in the unified pipeline.
type sourceEntry struct {
	name       string
	kind       string
	source     sources.Source
	noIndex    bool
	readGlobs  []string
	readPrefix string
}

// Build processes all sources according to cfg and opts, returning a Searcher
// and a channel that closes when initial processing completes.
// Each source is fetched (if remote and policy permits) then indexed (unless
// opts.SkipIndex or the source has index: false).
//
// Modes:
//   - SkipIndex=true:  fetch only, no indexing, blocks until complete (used by "sources pull")
//   - otherwise:       indexing runs in background; if syncInterval>0 a refresh loop re-runs every N minutes
//
// Pull policies (applied per-source, falling back to global):
//   - always:          re-fetch on every startup and refresh
//   - ifNotPresent:    skip fetch if source directory already exists (default)
//   - never:           never fetch, only use sources already on disk
func Build(ctx context.Context, cfg *config.Config, projectRoot string, opts BuildOptions) (search.Searcher, <-chan struct{}) {
	reg := sources.DefaultRegistry
	if opts.Registry != nil {
		reg = opts.Registry
	}
	mgr := &indexManager{
		idx:         search.NewLiveIndex(),
		cfg:         cfg,
		projectRoot: projectRoot,
		reg:         reg,
	}

	base := filepath.Join(projectRoot, sourcesDir)
	entries := sourceEntries(cfg, projectRoot, base, reg)

	done := make(chan struct{})

	if len(entries) == 0 {
		slog.DebugContext(ctx, "no sources configured, skipping indexing")
		close(done)
		return mgr.idx, done
	}

	if opts.SkipIndex {
		mgr.processSources(ctx, entries, true)
		close(done)
		return mgr.idx, done
	}

	go func() {
		mgr.processSources(ctx, entries, false)
		close(done)
	}()
	if cfg.Project.Sources != nil && cfg.Project.Sources.SyncInterval > 0 {
		go mgr.refreshLoop(ctx, time.Duration(cfg.Project.Sources.SyncInterval)*time.Minute)
	}

	return mgr.idx, done
}

// processSources processes entries in parallel, blocking until all complete.
func (m *indexManager) processSources(ctx context.Context, entries []sourceEntry, skipIndex bool) {
	if len(entries) == 0 {
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	for _, e := range entries {
		wg.Add(1)
		go func(e sourceEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m.processEntry(ctx, e, skipIndex)
		}(e)
	}

	wg.Wait()
	if !skipIndex {
		slog.InfoContext(ctx, "sources indexed", "chunks", m.idx.Len())
	}
}

// processEntry handles a single source entry: optional fetch then optional
// index merge.
func (m *indexManager) processEntry(ctx context.Context, e sourceEntry, skipIndex bool) {
	if e.kind != "local" {
		slog.InfoContext(ctx, "fetching source", "name", e.name, "kind", e.kind)
	}
	if err := e.source.Fetch(ctx); err != nil {
		slog.WarnContext(ctx, "fetch error", "name", e.name, "kind", e.kind, "err", err)
		return
	}

	if skipIndex || e.noIndex {
		return
	}
	docs, err := e.source.Read(e.readGlobs, e.readPrefix)
	if err != nil {
		slog.WarnContext(ctx, "source read error", "name", e.name, "kind", e.kind, "err", err)
		return
	}

	var chunks []search.Chunk
	for _, d := range docs {
		chunks = append(chunks, search.ChunkContent(d.Source, d.Content, search.ResolveStrategy("", d.Source))...)
	}

	if n := m.idx.Get().Merge(chunks); n > 0 {
		slog.DebugContext(ctx, "indexed source", "name", e.name, "kind", e.kind, "chunks", n)
	}
}

// refreshLoop runs the periodic re-search. It blocks until ctx is cancelled.
func (m *indexManager) refreshLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.DebugContext(ctx, "starting scheduled refresh")

			fresh := &indexManager{idx: search.NewLiveIndex(), cfg: m.cfg, projectRoot: m.projectRoot, reg: m.reg}
			base := filepath.Join(m.projectRoot, sourcesDir)
			entries := sourceEntries(m.cfg, m.projectRoot, base, m.reg)
			fresh.processSources(ctx, entries, false)
			m.idx.Swap(fresh.idx.Get())
			slog.InfoContext(ctx, "refresh complete", "chunks", m.idx.Len())
		}
	}
}

// sourceEntries builds the unified source list from all sections of the config.
// base is the root directory where fetched remote sources are stored.
func sourceEntries(cfg *config.Config, projectRoot, base string, reg *sources.Registry) []sourceEntry {
	if cfg.Project.Sources == nil {
		return nil
	}
	global := cfg.Project.Sources.PullPolicy

	var entries []sourceEntry
	add := func(kind, name string, raw any) {
		factory, ok := reg.Lookup(kind)
		if !ok {
			return
		}
		src, meta, err := factory(name, raw, projectRoot, base, global)
		if err != nil {
			return
		}
		entries = append(entries, sourceEntry{
			name:       name,
			kind:       kind,
			source:     src,
			noIndex:    meta.NoIndex,
			readGlobs:  meta.ReadGlobs,
			readPrefix: meta.ReadPrefix,
		})
	}

	for name, src := range cfg.Project.Sources.Local {
		add("local", name, src)
	}
	for name, src := range cfg.Project.Sources.Git {
		add("git", name, src)
	}
	for name, src := range cfg.Project.Sources.Scrape {
		add("scrape", name, src)
	}
	for name, src := range cfg.Project.Sources.HTTP {
		add("http", name, src)
	}
	return entries
}
