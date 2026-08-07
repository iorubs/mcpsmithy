## General Config Guide

You are helping the user write or improve a `.mcpsmithy.yaml` file.
This guide gives you the structure, then you call `config_section` for
the details of each section you need.

### Core Principle: Don't Repeat What Tools Can Discover

An AI assistant with MCP tools can read files, search code, and call
the tools you define. Your config should provide **what the AI cannot
figure out on its own**; project conventions, the *why* behind
decisions, and pointers to the right docs. Don't duplicate what's
already in the filesystem.

### What the File Does

`.mcpsmithy.yaml` defines what an MCP server exposes to an AI agent.
It declares the project identity, coding conventions, tool definitions,
and content sources. `mcpsmithy serve` reads this file and starts an
MCP server; the agent connects and calls the tools you define.

### How to Approach a New Config

Understand before you write. Read the project; local files, README,
existing docs; before declaring sources or conventions. For remote
sources, pull them first and examine the fetched content before
referencing it. Call `config_section` for each section when you are
ready to write it.

### Authentication

HTTP sources and the HTTP template functions authenticate from a
credentials file, by default `~/.mcpsmithy/credentials`. Set
`project.credentials` to use a different path. Keep it outside the
project directory: `file_read` is sandboxed to the project root, so a
credentials file inside it could be read by the agent or indexed by a
local source glob. Restricting it to your own user (`chmod 600`) is
sensible on a shared machine, but is not enforced.

Entries are keyed by hostname. The fields you set determine the header
that gets sent:

```yaml
credentials:
  # Authorization: Bearer ghp_xxx
  api.github.com:
    token: ghp_xxx

  # Authorization: Token abc123
  # For vendor schemes that are not Bearer.
  api.pagerduty.com:
    scheme: Token
    token: abc123

  # PRIVATE-TOKEN: glpat-xxx
  # For APIs that use their own header instead of Authorization.
  gitlab.example.com:
    header: PRIVATE-TOKEN
    token: glpat-xxx

  # Authorization: Basic base64(user:pass)
  basic.example.com:
    username: <username>
    password: <password>
```

A scheme is a single word joined to the token by a space. For a vendor
format like `Authorization: Token token=abc`, set `scheme: Token` and
`token: token=abc`.

Hosts with no entry fall back to `~/.netrc`, where a login of `token`
(or no login) sends the password as a Bearer token and any other login
sends Basic Auth. netrc is deprecated: its keyword set is fixed and the
file is shared with curl and git, so it cannot express vendor schemes or
custom header names. Prefer the credentials file.

Git sources do not use either mechanism; see the project section.

### Decision Rules

- **Index or not?** Index docs and content the agent should search by
  content. Set `index: false` for source code and config files the
  agent can already read directly; they provide structure only.

- **Convention per package?** No. Consolidate around tasks an engineer
  performs. "How do I add a config field?" is a convention. "The config
  package" is not.

- **What goes in `description` vs. docs?** Descriptions should say
  what to do and what rules exist. If detailed rules live in doc files,
  point to them with `docs:`; don't restate them.

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
tool-specific instructions in that file; those are all discoverable
via the tools themselves. Don't hardcode tool names either; tool names
may change. State the principle instead: *tools are mandatory, call
them throughout the session*. The tool descriptions in
`.mcpsmithy.yaml` already explain when to use each one.

A better approach is to put the essential context in the `project`
section of `.mcpsmithy.yaml` and surface it on demand via a `project_info`
tool. The agent calls the tool at the start of a session and gets accurate,
live context; no always-on injection needed. See the tools section for
a complete `project_info` example.

This keeps the persistent instruction file minimal (just "use the MCP
tools") while the real project context lives where it belongs; in the
config; and is fetched on demand.

### Next Steps

Call `config_section` for each section you need to write:
- `config_section section=project`: project metadata, content sources, and remote source patterns
- `config_section section=conventions`: convention definitions
- `config_section section=tools`: tool definitions and built-in functions
