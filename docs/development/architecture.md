# Architecture

## Overview

mcpsmithy is a single Go binary that reads a declarative YAML
project descriptor (`.mcpsmithy.yaml`) and serves a fully functional
MCP tool server over stdio. It bridges any project's conventions,
structure, and common tasks into the Model Context Protocol so that
AI coding assistants can discover and invoke them.

## Design Principles

1. **Zero per-project code** — All behaviour is declared in YAML.
2. **Minimal dependencies** — stdlib-first; keep external modules
   to the bare minimum to reduce supply-chain risk and keep builds fast.
3. **Security by default** — Read-only by design, sandbox path
   validation, output caps. No shell execution — all tools are
   inherently read-only.
4. **Everything internal** — Protocol types, config, engine, server
   all live under `internal/`. There is no public Go API — this is a
   standalone binary.

## Package Dependencies

Dependencies flow in one direction — leaf packages know nothing about
the packages that import them.

## Data Flow

```
 stdin / HTTP POST (/message)
   │
   ▼
 server.Transport         stdio: reads newline-delimited JSON
 (stdio or http)          http:  receives POST body
   │
   ▼
 server.Server            Dispatches by method name
   │
   ├─ initialize          Returns capabilities + server info
   ├─ tools/list          Returns tool definitions from config
   ├─ tools/call  ───►  tools.Engine
   │                      │
   │                      └─ template handler  → templateEngine rendering
   ├─ ping                Returns empty result
   ├─ notifications/*     Silently dropped
   │
   ▼
 server.Transport         stdio: writes JSON-RPC to stdout
 (stdio or http)          http:  pushes SSE event to GET /sse stream
```

All logs go to **stderr** to keep stdout and the SSE stream clean.

## Config-Driven Design

The engine builds a `Handler` function for each tool at startup by
parsing its `template:` field from YAML. No tool logic is hard-coded —
the binary is the same regardless of project. The YAML config is the
only thing that changes between Go, TypeScript, Python, Rust, or any
other project.

Every tool is a Go `text/template` that can call built-in functions
and access the project context via `{{ .mcpsmithy }}`.

User-facing docs for template functions and the `mcpsmithyContext` fields are generated automatically from code.

## Source Lifecycle

Sources feed a BM25 search index that powers `search_for`. Conventions are indexed
separately in their own BM25 index so they are always surfaced
prominently regardless of term frequency in source docs.

`search_for` queries both indexes: **conventions first** (ranked
among themselves), then **source docs** (ranked among themselves).
This ensures conventions are always visible and the LLM can decide
whether they apply.

Both indexes use real term frequency, English stop-word removal,
and suffix stemming to improve search quality. The lifecycle has
two phases:

1. **Pull** — Fetch external content (HTTP archive download or
   `git clone`, HTTP scrape) and write the raw source files to
   `.mcpsmithy/`. Local files need no fetching.
   Conventions require no fetching — they are built from config.
2. **Serve** — Start the transport immediately; each source
   fetches, chunks, and merges into the source index
   concurrently. The convention index is built synchronously
   from config at startup. Local sources and conventions are
   available sub-10 ms; remote sources enrich the source
   index as they complete. Neither index is written to disk.

### Markdown chunking

`.md`/`.markdown`/`.mdx` files are split into chunks at ATX heading
boundaries (`# `, `## `, …). Two details worth knowing:

- **Frontmatter stripping** — YAML frontmatter (`---…---` at the start
  of a file) is stripped before splitting, so key-value metadata never
  pollutes BM25 term scores.
- **Heading breadcrumb** — Each chunk's `Section` field carries the
  full ancestor hierarchy, e.g. `"Feature X > Installation"`. This
  makes parent heading terms available to BM25 scoring on every
  descendant chunk without changing chunk boundaries.

All other file types are indexed as a single chunk (whole-file).
Scraped HTML is converted to Markdown before chunking, so it also
benefits from heading-based splitting.

Git sources shell out to the local `git` binary to clone repositories; credentials come from SSH keys, credential helpers, or `~/.netrc`. HTTP sources download tarballs or other files over HTTP(S) using `~/.netrc` or custom headers — use these when you want archive downloads without requiring the `git` binary. The `pullPolicy` field controls when fetching runs at startup; it governs source fetching only — the index is always rebuilt in-memory from local source files regardless of policy. See the config reference for field semantics.

The `sources pull` CLI command always re-fetches all sources regardless
of policy — it is the explicit "refresh now" action useful for Docker
build-time pre-population of `.mcpsmithy/`.

Local sources are always read from disk at index build time (they
have no remote endpoint to fetch from).

## Concurrency Model

The stdio transport is effectively sequential: one goroutine reads requests and
the dispatch loop handles them one at a time. `writeResponse` holds a mutex to
guard against any future concurrent writes.

The HTTP transport is concurrent by Go's `net/http` design — each incoming
request is handled in its own goroutine. Responses are serialized back to the
correct SSE stream via a per-session buffered channel, so each client's stream
is coherent even under concurrent load.

## Error Handling

- **Config errors** — Fatal at startup; the server won't start with
  a broken config.
- **Tool execution errors** — Returned as `ToolResult` with
  `isError: true`, not as JSON-RPC errors. This lets the AI see what
  went wrong.
- **Protocol errors** — Returned as JSON-RPC error responses with
  standard codes (-32700 parse error, -32601 method not found,
  -32602 invalid params).
- **Unknown methods** — Notifications are silently dropped; requests
  get a `-32601 Method Not Found` error.
