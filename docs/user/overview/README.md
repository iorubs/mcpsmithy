---
slug: /
---

# Overview

![mcpsmithy forge](/img/body.png)
> **MCPSmithy** is a single Go binary that reads a declarative YAML
config file (`.mcpsmithy.yaml`) and serves a fully functional
[MCP](https://modelcontextprotocol.io) tool server.

## Why does it exist?

There are many ways to give an agent domain-specific context. What's
harder is knowing *which* context matters for a given task, without
building custom servers or deciding for yourself what's relevant every time.

- **Turn any knowledge source into MCP tools** — local files, git repos,
  scraped sites, docs directories, all declared in YAML with no server
  code. Use it to give agents a map of complex or distributed systems,
  serve your docs as an MCP server, or define the conventions and
  structure of any project.
- **Zero per-project code** — All behaviour comes from YAML config. Works for any stack — Go, Python, TypeScript, Rust, or anything else.
- **Works with any model** — Frontier models can sometimes infer conventions from context; cheaper ones can't. By making context explicit and tool-accessible, mcpsmithy closes that gap.
- **Captures expert context** — Conventions encode the knowledge that
  lives in engineers' heads: which docs govern which paths, what rules
  apply, and how parts of the system relate.

## How it works

1. You write a `.mcpsmithy.yaml` describing your project.
2. You run `mcpsmithy serve` (or via Docker).
3. Your MCP-compatible AI assistant discovers and calls the tools.

## Config Overview

| Section        | Purpose                                                                                               |
|----------------|-------------------------------------------------------------------------------------------------------|
| `project`      | Name, description, and content sources                                                                |
| `conventions`  | Encode expert knowledge: which docs and rules apply to which paths, and how parts of the system relate |
| `tools`        | Tools defined as Go templates — call built-in functions or compose them into pipelines, all in config |

The server speaks MCP over JSON-RPC 2.0. It uses stdio by default
(the MCP client spawns the binary directly) or HTTP/SSE for Docker
and remote deployments.

### Reference docs:
- [Config Reference](../reference/config/)
- [Server Reference](../reference/server/)
