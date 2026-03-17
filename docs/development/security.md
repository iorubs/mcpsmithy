# Security Model

## Threat Model

mcpsmithy reads files and renders templates on behalf of an AI
assistant. All tools are read-only by design — there is no
shell execution. The primary risks are:

1. **Path traversal** — AI requests a file outside the project
2. **Resource exhaustion** — Unbounded output from large file reads
3. **Information disclosure** — AI reads sensitive files within the project

## Mitigations

### Sandbox (Path Validation)

All file operations go through the sandbox. It:

- Resolves the project root to an absolute path (following symlinks)
- Rejects any resolved path that doesn't have the root as a prefix
- Applies to the `file_read` function and all file access paths


### Output Caps

- **Per-file:** The `file_read` function defaults to a 50 KB per-file cap (configurable via the optional `maxFileSize` argument)
- **Per-tool:** Each tool has a `maxOutputKB` field (default 1024 KB / 1 MB) that caps the final rendered output returned to the MCP client. Config authors can raise or lower this per tool.
- **HTTP body:** `http_get` caps the raw response body at 10 MB before template rendering, preventing OOM from large HTTP responses regardless of the tool output limit.
- Truncated output includes a `... (truncated at NKB)` marker

### Read-Only by Design

All tools are inherently read-only. There is no shell execution in tool
handlers — the AI can discover and read project content but never mutate it.

> **Note:** `exec.CommandContext` is used for git clone during source fetching. This is not AI-triggerable;
> fetching runs at server startup only. HTTP sources fetch over pure
> HTTP with no subprocess execution.

Shell support was intentionally removed to eliminate the largest
attack surface. See [improvements.md](improvements.md#shell-command-execution)
for the full rationale and the requirements that would need to be met
before reintroduction.

### Stderr-Only Logging

All logs go to stderr. Stdout is reserved exclusively for JSON-RPC
protocol traffic. This prevents log injection into protocol messages.

## What Is NOT Mitigated

- **Config trust** — The `.mcpsmithy.yaml` file is trusted. Anyone
  who can modify it controls which files are exposed to the AI and
  what template logic runs.
- **Sensitive file exposure** — File access is scoped by declared sources
  and the `project_file_path` parameter type, which restricts reads to
  paths within configured sources. This means the AI can only read files
  the config author explicitly exposed via source declarations. The config
  author's responsibility is to avoid declaring sources with overly broad
  glob patterns. The `.mcpsmithy.yaml` file is only readable if the author explicitly declared it in a source.
