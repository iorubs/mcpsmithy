## Project Section Guide

The `project` key identifies the project and declares its content
sources. The field reference below covers every field; this guide
covers strategy.

### Setup Workflow

1. **Examine local content** — read the directory tree, README, and
   any existing docs you can access directly. Understand the project
   structure before declaring sources.

2. **Declare sources** — set `name` and `description`. Declare local
   sources for source code, docs, tests, and config files. For remote
   sources (git, http, scrape), add a minimal entry with just the
   required fields — enough to pull.

3. **Pull remote sources** — run `mcpsmithy sources pull`. This
   fetches remote content into `.mcpsmithy/` so you can examine it.

4. **Examine remote content** — read the fetched files under
   `.mcpsmithy/`. Understand what each remote source actually
   contains — its structure, doc topics, naming patterns — before
   writing conventions or tools that reference them.

### Keep the Description Relevant

The `description` field appears in `project_info` output. The agent can use this to orient itself at the start of every session when combined with a tool.

### Don't Over-Segment Sources

Multiple sources with fine-grained descriptions can usually be
consolidated into fewer entries if the distinction doesn't affect
conventions. Split only when conventions need to reference them
separately.

### Git Authentication

Git sources use your existing git credentials — SSH keys, credential
helpers, or HTTPS via `~/.netrc`.

### HTTP Source Authentication

HTTP sources (for forge archive downloads, artifact stores, private
APIs) read `~/.netrc` automatically. Custom headers can be added for
bearer tokens or API keys. Use an HTTP source instead of git when
you want a tarball download without requiring the `git` binary.

### Pre-Built Images Pattern

For CI/CD where credentials are available at build time but not
runtime, use a multi-stage Docker build: fetch sources with creds in
the build stage (`mcpsmithy sources pull`), then copy the cache
directory (`.mcpsmithy`) into the runtime image. Set
`pullPolicy: never` in the runtime config so the server never
attempts to fetch.
