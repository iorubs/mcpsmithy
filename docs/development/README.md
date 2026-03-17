# Development Guide

## Overview

mcpsmithy is a config-driven MCP tool server written in Go. All
tool behaviour is declared in YAML — the binary contains no
project-specific logic. See the project [README](../../README.md) for
user-facing documentation.

## Development Docs

| Document | Scope |
|----------|-------|
| [Architecture](architecture.md) | Package layout, dependency graph, data flow, error handling |
| [Config](config.md) | `.mcpsmithy.yaml` format reference, versioning, and how to extend it |
| [CLI Design](cli.md) | Kong-based CLI, subcommands |
| [Security](security.md) | Sandbox, path validation, output limits |
| [Testing](testing.md) | Testing conventions and guidelines |
| [Improvements](improvements.md) | Roadmap and tracked enhancements |

## Dependencies

The project follows a **stdlib-first** approach.

**Go extended standard library (`golang.org/x`)** — maintained by the
Go team under the same review standards as the stdlib. Low risk; the
main consideration is binary size.

- `golang.org/x/net` — HTML tokenizer/parser used by the scrape
  source to convert fetched pages into Markdown.

**Third-party** — each addition should be justified by significant
value over a stdlib solution, and vetted for maintenance status,
transitive dependencies, and API stability.

- `github.com/alecthomas/kong` — CLI parsing. Provides declarative
  struct-tag-based argument definitions with no runtime dependencies.
  Revisit once the CLI surface stabilises — replacing with stdlib
  `flag` is straightforward but premature while commands are still
  evolving.
- `go.yaml.in/yaml/v4` — YAML config parsing. Maintained by the
  YAML spec maintainers with zero transitive dependencies.

Avoid adding dependencies unless they provide significant value over a
stdlib solution.

## Naming Conventions

Follow standard Go naming conventions throughout the codebase:

- **Packages** — Short, lowercase, single-word names. The package
  name should describe what it provides, not what it contains.
- **Exported identifiers** — Use `MixedCaps`. Exports should form the
  public API of the package; keep it small and intentional.
- **Unexported identifiers** — Use `mixedCaps`. Prefer short names
  for local variables with small scopes.
- **Interfaces** — Name after the behaviour they describe, not the
  implementation. Single-method interfaces use the method name plus
  `-er` suffix when it reads naturally.
- **Files** — Use `snake_case.go`. Test files use `_test.go` suffix.
  Keep one primary type or concept per file.
- **Acronyms** — Use consistent casing: `ID`, `URL`, `HTTP`, not
  `Id`, `Url`, `Http`.

## Comment Conventions

Only add comments when they provide meaningful value beyond the code
itself. Specifically:

- **Package comments** — Every package should have a `// Package ...`
  comment on the primary file, describing the package's purpose.
- **Exported symbols** — Document all exported types, functions, and
  methods with a `// Name ...` comment that explains what and why,
  not how.
- **Non-obvious logic** — Comment complex algorithms, non-trivial
  design decisions, or workarounds. If the next reader would need
  to stop and reason about the code, a comment helps.
- **Skip the obvious** — Do not comment trivial getters, setters,
  or straightforward control flow. The code is the documentation.

## Logs and Output

- **stdout** is the MCP protocol channel. Never write application
  output to stdout.
- **stderr** receives all log output via the `log/slog` package.
- Use the context-aware `slog` functions (`slog.InfoContext`,
  `slog.WarnContext`, etc.) for all log calls. Do not pass
  `*slog.Logger` as a function parameter or store it in structs.
  Instead, call `slog.SetDefault` once at startup and use the
  package-level `slog.*Context(ctx, ...)` functions everywhere.
  This keeps function signatures clean and lets handlers extract
  enrichment from `ctx` in the future (request IDs, trace spans,
  etc.).

### Log Levels

The CLI flag `--log-level` (`-l`) sets the minimum level. Default is
`info`. The principle: **info is what an operator sees in normal
production; debug is what a developer turns on to diagnose a
specific problem.**

| Level | Use for | Examples |
|-------|---------|----------|
| **Error** | Failures that stop or degrade the current operation. The server or command cannot continue normally. | Config load failure, engine build failure, hot-reload failure. |
| **Warn** | Unexpected conditions that are recoverable. The operation continues but the result may be incomplete. | Source fetch error (skipped, others continue), malformed JSON-RPC line, protocol version mismatch. |
| **Info** | Significant lifecycle events and operations involving network or I/O that an operator would want to see. One line per event, not per item. | Server starting, ready, fetching a remote source, index summary totals, hot-reload swap, refresh complete. |
| **Debug** | Per-item detail, internal decisions, protocol wire traffic. Useful for diagnosing but too noisy for normal operation. | Per-source index/skip/merge counts, JSON-RPC recv/send, tool call params, SSE connect/disconnect, refresh heartbeat. |

### Level Decision Rules

1. **Network or I/O action starting** → Info (the operator should know
   something external is happening).
2. **Skipping something / cache hit** → Debug (nothing happened, only
   interesting when diagnosing).
3. **Per-item progress within a batch** → Debug (the summary at the
   end covers info).
4. **Summary or total at end of a phase** → Info (one line, not N).
5. **Protocol-level wire messages** → Debug.
6. **Something failed but we continue** → Warn.
7. **Something failed and we stop** → Error.

## Error Messages

Error messages follow a consistent style:

- **Start with lowercase** — Go convention.
- **Lead with the operation, then the cause** — Use a colon separator when wrapping.
- **Be specific about what went wrong**, not prescriptive about how to fix it.
- **Include context for wrapped errors** — Include the struct/tool/function involved.
- **Wrap only when it adds value** — If the error already identifies the operation and location clearly, return it as-is. Redundant wrapping (e.g. `"reading file: %w"` when the underlying error already says which file) adds noise without aiding diagnosis.
