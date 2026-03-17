# Config

Auto-generated schema and authoring reference for `.mcpsmithy.yaml`.

## Schema Versions

- [Version 1](v1.md)

---

## General Config Guide

You are helping the user write or improve a `.mcpsmithy.yaml` file.
This guide gives you the structure, then you call `config_section` for
the details of each section you need.

### Core Principle: Don't Repeat What Tools Can Discover

An AI assistant with MCP tools can read files, search code, and call
the tools you define. Your config should provide **what the AI cannot
figure out on its own** — project conventions, the *why* behind
decisions, and pointers to the right docs. Don't duplicate what's
already in the filesystem.

### What the File Does

`.mcpsmithy.yaml` defines what an MCP server exposes to an AI agent.
It declares the project identity, coding conventions, tool definitions,
and content sources. `mcpsmithy serve` reads this file and starts an
MCP server — the agent connects and calls the tools you define.

### How to Approach a New Config

Understand before you write. Read the project — local files, README,
existing docs — before declaring sources or conventions. For remote
sources, pull them first and examine the fetched content before
referencing it. Call `config_section` for each section when you are
ready to write it.

### Decision Rules

- **Index or not?** Index docs and content the agent should search by
  content. Set `index: false` for source code and config files the
  agent can already read directly — they provide structure only.

- **Convention per package?** No. Consolidate around tasks an engineer
  performs. "How do I add a config field?" is a convention. "The config
  package" is not.

- **What goes in `description` vs. docs?** Descriptions should say
  what to do and what rules exist. If detailed rules live in doc files,
  point to them with `docs:` — don't restate them.

### Minimal Working Example

```yaml
version: "1"

project:
  name: "my-project"
  description: "Brief description of what this project does."
  sources:
    local:
      source:
        paths: ["src/**"]
        description: "Application source code"
        index: false
      docs:
        paths: ["docs/**", "README.md"]
        description: "Project documentation"

conventions:
  code-style:
    scope: "*"
    description: "General coding conventions. Read the docs."
    docs:
      - source: docs

tools:
  project_info:
    description: "Returns project overview and file structure. Call at the start of every session."
    template: |
      {{ .mcpsmithy.Project }}

      Conventions:
      {{ range $k, $v := .mcpsmithy.Conventions }}
      {{ $k }}: {{ $v.Description }}
      {{ end }}
```

### Project Instructions for AI

Many editors and AI coding tools support a persistent instruction file
that is injected into **every** AI interaction. Noise here drowns
signal. Its only job is to tell the AI that MCP tools exist and that
using them is mandatory.

Don't duplicate project structure, conventions, commands, or
tool-specific instructions in that file — those are all discoverable
via the tools themselves. Don't hardcode tool names either; tool names
may change. State the principle instead: *tools are mandatory, call
them throughout the session*. The tool descriptions in
`.mcpsmithy.yaml` already explain when to use each one.

A better approach is to put the essential context in the `project`
section of `.mcpsmithy.yaml` and surface it on demand via a `project_info`
tool. The agent calls the tool at the start of a session and gets accurate,
live context — no always-on injection needed. See the tools section for
a complete `project_info` example.

This keeps the persistent instruction file minimal (just "use the MCP
tools") while the real project context lives where it belongs — in the
config — and is fetched on demand.

### Next Steps

Call `config_section` for each section you need to write:
- `config_section section=project` — project metadata, content sources, and remote source patterns
- `config_section section=conventions` — convention definitions
- `config_section section=tools` — tool definitions and built-in functions


---

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
helpers, or HTTPS via `~/.netrc`. Both GitHub and GitLab accept
PAT-based Basic Auth over HTTPS:

```
machine github.com login <any> password <PAT>
machine gitlab.com login <any> password <PAT>
```

Use SSH repo URLs (`git@github.com:org/repo.git`) when SSH auth is
preferred.

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


#### Local sources

Structure and docs from the local workspace. Set `index: false` on
source code the agent can already read with its editor tools.

```yaml
project:
  name: "my-service"
  description: "A backend service with REST API"
  sources:
    local:
      source:
        paths: ["cmd/**", "internal/**", "pkg/**"]
        description: "Application source code"
        index: false
      docs:
        paths: ["docs/**/*.md", "README.md"]
        description: "Project documentation"
```

#### Git sources

Pull docs or config from another repository. Uses git clone under the
hood — credentials come from SSH keys, credential helpers, or `.netrc`.

```yaml
project:
  name: "my-service"
  description: "A backend service that follows platform conventions"
  sources:
    git:
      platform-docs:
        repo: "https://github.com/org/platform-docs.git"
        ref: "main"
        paths: ["docs/**/*.md"]
        description: "Platform engineering documentation"
      runbooks:
        repo: "git@github.com:org/runbooks.git"
        ref: "main"
        paths: ["services/my-service/**/*.md"]
        description: "Operational runbooks for this service"
        depth: 1
```

#### HTTP sources

Fetch archives or files from any authenticated HTTP endpoint — forge
archive URLs, artifact stores, private APIs. Auth via `.netrc` or
custom headers.

```yaml
project:
  name: "my-service"
  description: "Service with external API specs"
  sources:
    http:
      api-spec:
        url: "https://gitlab.com/api/v4/projects/org%2Fapi-spec/repository/archive.tar.gz?sha=main"
        paths: ["**/*.yaml"]
        description: "OpenAPI specs from the API spec repo"
      design-system:
        url: "https://github.com/org/design-system/archive/refs/heads/main.tar.gz"
        paths: ["docs/**/*.md"]
        description: "Design system documentation"
```

#### Scrape sources

Crawl external doc sites. Useful for vendor docs or internal wikis
that aren't in a git repo.

```yaml
project:
  name: "my-service"
  description: "Service that integrates with external APIs"
  sources:
    scrape:
      vendor-api:
        urls: ["https://docs.vendor.com/api/v2"]
        description: "Vendor API reference"
        maxDepth: 2
        maxPages: 50
      internal-wiki:
        urls: ["https://wiki.internal/platform/my-service"]
        description: "Internal wiki pages for this service"
        maxPages: 20
```

---

## Conventions Section Guide

The `conventions` key is a map of named convention definitions. Each
convention tells the agent what rules apply to a part of the codebase.

### Think Tasks, Not Packages

Conventions should map to tasks an engineer performs, not mirror the
directory tree. Ask: "What would someone search for?"

```yaml
# Bad — mirrors the package tree
server:
  scope: "internal/server/**"
  description: "MCP protocol server implementation."
config:
  scope: "internal/config/**"
  description: "Config loading and validation."

# Good — task-oriented
config-schema:
  scope: "internal/config/**"
  description: |
    Versioned YAML config with strict parsing — unknown keys are errors.
    Adding a new config field:
      1. Add to the types in the latest config version
      2. Add a default if needed
      3. Add validation
tool-engine:
  scope: "internal/tools/**"
  description: |
    Tool execution, template rendering, and sandbox.
    Adding a new template function:
      1. Implement in templating.go
      2. Test in templating_test.go
      3. Document in config.md
```

### Describe Workflows, Not Inventories

The AI can read code to see what files exist. Conventions should
describe **what to do** — checklists for common tasks, the order of
operations, which files to touch and why.

```yaml
# Bad — the AI already knows this from reading the code
description: >-
  server.go: read-dispatch-write loop. types.go: JSON-RPC types.

# Good — tells the agent what to do
description: |
  Tool execution, template rendering, and sandbox.
  Adding a new template function:
    1. Implement the function and register it in templating.go
    2. Test in templating_test.go
    3. Document in config.md (built-in functions table)
    4. Update example YAMLs in docs/user/examples/ if user-facing
```

### Point to Docs, Don't Restate Them

If rules already live in a doc file, reference them — don't duplicate
them in the description. Duplication means maintaining rules in two
places; they'll drift.

```yaml
# Bad — restates what's in testing.md
description: |
  Tests use stdlib table-driven style with t.Run sub-tests.
  Use t.Helper() on shared assertion functions.

# Good — points to the source of truth
description: |
  This project has coding and testing conventions.
  Read the docs before writing or modifying code.
docs:
  - source: docs
    paths: ["docs/development/testing.md"]
```

**Exception:** Small projects without separate doc files can define
rules directly in conventions. Refactor into doc references when
the project grows.

### Avoid Brittle References

Descriptions that name specific types, leaf file paths, or internal
functions create maintenance traps. When those internals change, the
convention becomes a source of wrong advice.

Use intent-based language — describe *what* to find, not the exact
symbol name. The AI can locate the current implementation.

```yaml
# Brittle — renames or refactors break these references
description: |
  Adding a new config field:
    1. Add the field to config/v1/types.go
    2. Update applyDefaults() in config/v1/helpers.go
    3. Add validation in config/v1/validate.go

# Stable — says what to do without pinning to symbols
description: |
  Adding a new config field:
    1. Add to the types in the latest config version
    2. Add a default if needed
    3. Add validation
```

**Workflow steps vs. symbol references.** Naming files as steps in a
workflow is fine — "implement in templating.go, test in
templating_test.go". What drifts is pinning to internal symbols:
struct names, function signatures, internal constants. The AI can
find the current struct name; it can't recover from trusting a
renamed one.

### Name for Searchability

Convention names appear in search results and relation targets.
Choose names an engineer would actually search for.

```yaml
# Generic            # Searchable
tools:               tool-engine:
config:              config-schema:
sources:             sources-search:
```

### Fewer Conventions Beat More

Every convention competes for search ranking. If nobody would search
for a convention in isolation, it doesn't need to exist separately.
Consolidate related concerns.

### Scopes Must Match Real Directories

Convention scopes are matched against file paths. Scope globs that
point to nonexistent directories are dead weight. Audit against the
actual directory layout.

### Point to Specific Docs, Not Everything

Each convention should reference only the docs relevant to that
scope. If every convention points to `architecture.md`, it's noise —
drop the reference unless it contains information specific to that
convention's workflows.

```yaml
# Noisy — architecture.md on everything
server:
  docs: ["docs/development/architecture.md"]
config:
  docs: ["docs/development/config.md", "docs/development/architecture.md"]

# Targeted — only docs that help with this convention's tasks
config-schema:
  docs: ["docs/development/config.md"]
code-style:
  docs: ["docs/development/README.md", "docs/development/testing.md"]
```

### Keep Descriptions Accurate, Not Aspirational

Convention descriptions must reflect the code **as it is today**.
Don't describe planned features, removed code, or inaccurate flags.
An AI that trusts stale conventions will generate wrong code.

### Use Relations for Cross-References

When two conventions interact (e.g., config schema affects tool
definitions), link them with `relations`. The agent can then navigate
from one context to related contexts.

### Document Cross-System Relationships

When a config pulls in multiple sources spanning different systems,
conventions are the right place to capture how those systems relate.
No amount of code reading can produce operational topology — it
lives in the team's understanding.

```yaml
deployment-flow:
  scope: "*"
  description: |
    This config spans three systems that form a deployment chain:
      - infra (Terraform): provisions the EKS cluster
      - platform (ArgoCD + CI): builds and deploys
      - app: deployed via ArgoCD Application CRD

    When troubleshooting deployment failures, trace backwards:
    app manifest → ArgoCD sync → CI pipeline → cluster state.
```


#### Task-oriented convention with docs

Scope maps a file path to rules and documentation. The description
says what to do — the linked docs provide the full detail.

```yaml
conventions:
  config-schema:
    scope: "internal/config/**"
    description: |
      Versioned YAML config with strict parsing — unknown keys are errors.
      Adding a new config field:
        1. Add the field to the latest version types
        2. Add a default if needed
        3. Add validation
        4. Run the tests
    docs:
      - source: docs
        paths: ["docs/development/config.md"]
    tags: ["config"]
    relations:
      - target: tool-engine
        description: "Config defines tool and param declarations consumed by the engine"
```

#### Convention graph for documentation

Conventions don't have to map to code paths. They can model a
knowledge graph over doc sections — each convention is a topic with
relations to other topics.

```yaml
conventions:
  overview:
    scope: "docs/overview/**"
    description: "Architecture and high-level concepts."
    tags: ["architecture"]
    relations:
      - target: "features"
        description: "Architecture explains the features"
      - target: "api-reference"
        description: "Architecture references the API surface"

  features:
    scope: "docs/features/**"
    description: "Feature guides by area — auth, deployment, networking."
    tags: ["how-to"]
    relations:
      - target: "api-reference"
        description: "Feature guides reference API specs"
```

---

## Tools Section Guide

The `tools` key is a map of named tool definitions. The field
reference below covers every field; this guide covers how to use
them well.

### Templates

Every tool requires a `template:` field — a Go `text/template`
string. Templates can call built-in functions (`conventions_for`,
`search_for`, `file_read`, `http_get`, `grep`) and access the
project context via `{{ .mcpsmithy }}`. Params and options are
accessible as `{{ .paramName }}`.

### Let Descriptions Teach

Tool descriptions appear in `tools/list` — the agent reads them
every session. Write descriptions that say **when** and **why** to
call each tool, not just what it returns. "Returns project info" is
weak. "Returns project overview and file structure. Call at the start
of every session." tells the agent when to use it.

### Use Param Descriptions to Guide Inference

The agent sees param names and descriptions before deciding what
value to pass. Use them to bridge the gap between what the user
provides and what the template actually needs.

For example, if a tool calls `http_get` against a CI API but the
user will paste a browser URL, the param name and description should
tell the agent how to transform it:

```yaml
params:
  - name: "traceUrl"
    type: "string"
    required: true
    description: >-
      Use the API trace format:
      https://<host>/api/v4/projects/<url-encoded-project-path>/jobs/<job_id>/trace.
      Extract the project path and job ID from the standard job page URL.
```

The name `traceUrl` already signals it's not a raw browser URL.
The description spells out the exact format. Without these hints,
cheaper models may pass the browser URL directly and blame
authentication when the request fails.

This applies broadly — whenever the template expects a value in a
different format than what the user naturally provides, encode that
in the param name and description.

### Encode Opinions, Not Syntax

The AI knows `go test` and `npm run build`. What it doesn't know is
your project's rules. A `project_commands` template tool should
encode **how the AI should operate** — commands plus behavioral
rules like "always run tests with -cover" or "do not use sed for
file edits."

### Don't List Aspirational Commands

If a command doesn't work yet, don't put it in the config. The AI
will try to run it, fail, and waste time debugging a tool that
doesn't exist. Keep the config honest about the current state.

### Combine Conventions and Search

If you index sources, make sure some tool queries them. A common
pattern combines `conventions_for` with `search_for` in a template
so one call gives both the rules and relevant content.

### `file_read` Is for Inaccessible Content

The AI can already read local files with its editor tools. Use
`file_read` in templates only for content the AI cannot access
directly — files outside the workspace or generated content.

### Tool Sets by Use Case

- **Docs Assistant** — `get_convention`, `search`, `read_doc`
- **Project Awareness** — `find_convention`, `search`
- **Support & Troubleshooting** — `project_info`, `find_convention`, `search`, `ci_log`
- **Agentic Application** — `search`, `find_convention`, `get_status`

Full YAML examples for each deployment mode are below.


Both `conventions` and `tools` are required. A valid config needs at
least one entry in each.

#### Local mode (stdio) — agent has workspace access

When the agent already has file and terminal tools (VS Code, Cursor,
etc.), mcpsmithy adds the project context layer. Use `file_read`
only for content outside the workspace (git/scrape sources).

```yaml
tools:
  project_info:
    description: |
      Platform overview, convention map, and source inventory.
      ALWAYS call this tool first — it establishes the context needed
      for all subsequent tool calls.
    template: |
      {{ .mcpsmithy.Project }}

      Tools:
      {{ .mcpsmithy.Tools }}

      Conventions:
      {{ .mcpsmithy.Conventions }}

  find_convention:
    description: |
      Match a file path to related conventions and indexed content.
      Use when unsure which rules apply to the code you're editing.
    template: |
      {{ conventions_for .path }}

      Related indexed content:
      {{ search_for .path .maxResults .maxResultSize }}
    params:
      - name: "path"
        type: "string"
        required: true
        description: "File path relative to project root (e.g. internal/api/handler.go)"
    options:
      maxResults: 5
      maxResultSize: 2048

  search:
    description: |
      Search across all indexed sources. Returns matching conventions
      and content from docs, runbooks, and any other indexed source.
    template: "{{ search_for .query .maxResults .maxResultSize }}"
    params:
      - name: "query"
        type: "string"
        required: true
        description: "Keywords, error messages, or file names to search for"
    options:
      maxResults: 10
      maxResultSize: 2048

  project_commands:
    description: "Build, test, and lint commands with rules."
    template: |
      Build:  go build ./...
      Test:   go test -cover ./...
      Lint:   golangci-lint run ./...

      Rules:
      - Always run tests with -cover.
      - Write tests alongside code, not after.

  ci_log:
    description: |
      Fetch and filter CI/CD job logs. CI logs often surface multiple
      errors; the root cause typically appears first — focus on the
      earliest match.
    template: "{{ http_get .traceUrl | grep .pattern .before .after }}"
    params:
      - name: "traceUrl"
        type: "string"
        required: true
        description: >-
          CI job trace API URL. For GitLab:
          https://<host>/api/v4/projects/<url-encoded-path>/jobs/<id>/trace.
          Extract project path and job ID from the browser URL.
      - name: "pattern"
        type: "string"
        required: true
        description: "Regex to filter log lines (e.g. 'error|fatal|FAIL')"
      - name: "before"
        type: "int"
        default: 10
        description: "Context lines before each match"
      - name: "after"
        type: "int"
        default: 10
        description: "Context lines after each match"
```

#### Remote mode (HTTP/SSE) — agent has no workspace access

When the agent connects over HTTP (docs server, agentic backend),
it has no filesystem access. Expose `read_doc` and `search` so the
agent can discover and read content. `file_read` is essential here
because the agent has no other way to access the files.

```yaml
tools:
  project_info:
    description: |
      Documentation map and available conventions.
      ALWAYS call this tool first — it establishes the context needed
      for all subsequent tool calls.
    template: |
      {{ .mcpsmithy.Project }}

      Conventions:
      {{ range $k, $v := .mcpsmithy.Conventions }}
      {{ $k }}:
        description: {{ $v.Description }}
        scope: {{ $v.Scope }}
      {{ end }}

  search:
    description: |
      Search all indexed sources by keyword. Returns ranked results
      with matching conventions, titles, and snippets.
    template: "{{ search_for .query .maxResults .maxResultSize }}"
    params:
      - name: "query"
        type: "string"
        required: true
        description: "Search terms (e.g. 'deploy application')"
    options:
      maxResults: 10
      maxResultSize: 400

  read_doc:
    description: |
      Read the full content of a file by path. Use after search to
      get the complete document.
    template: "{{ file_read .path .maxFileSize }}"
    params:
      - name: "path"
        type: "string"
        required: true
        description: "Path from search results (e.g. docs/features/deployment.md)"
    options:
      maxFileSize: 50

  find_convention:
    description: "Returns full details for a convention by name."
    template: "{{ index .mcpsmithy.Conventions .name }}"
    params:
      - name: "name"
        type: "string"
        required: true
        description: "Convention name from project_info"
```
