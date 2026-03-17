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
