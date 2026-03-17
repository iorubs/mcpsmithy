---
sidebar_position: 1
---

# Docs Assistant

## User story

As an **Engineer or documentation owner**, I want to expose my docs as
an MCP server with zero custom code so that users can connect their AI
agents and get domain-specific knowledge without reading the docs site
manually.

## Goals

- Docs are searchable and surfaced by relevance
- Docs are navigable by topic via a convention graph
- No custom server code — one YAML file, Docker, done
- Works for local Markdown and external/hosted docs sites

## Technical overview

**Deployment:** HTTP/SSE. The agent has no filesystem access to the
docs — every tool exposed by mcpsmithy is its only way to discover,
search, and read content.

**Data sources:**
- Local Markdown files (`project.sources.local`)
- Hosted or external docs sites (`project.sources.scrape`)

**Conventions as a graph:** Each doc section becomes a convention.
`relations` link sections together so the agent can navigate from
overview to features to API reference by following the graph, rather
than needing to know the structure upfront.

**Reading docs:** `read_doc` lets the agent fetch a specific file
directly once it knows what it's looking for — typically after
`get_convention` or `search` has pointed it at the right location.

**Tools needed:**
- `get_convention` — fetches a convention by ID; returns its content and related sections
- `search` — ranked search across all indexed sources; conventions are surfaced first — use this to build tools that surface the right docs automatically without the agent needing to know the structure upfront
- `read_doc` — reads a specific doc file

For full YAML examples of each source type, tool template, and convention pattern, see the [config reference](../../reference/config/README.md).

:::tip Generate this config with your agent
Run `mcpsmithy setup`, then share this story with your agent. Before
prompting, have ready:

- The local docs directory (or external site URLs) to serve
- A rough outline of your doc sections (e.g. overview, features, API, CLI)

Then use a prompt like:

> Set up mcpsmithy to serve this docs directory as an MCP server.
> Here's the structure: [list your sections]. Create a convention for
> each section with an ID, and add relations between them so agents
> can navigate the graph. Expose `get_convention`, `search`, and
> `read_doc` tools. The server will run in HTTP/SSE mode via Docker.

See [Assisted setup](../guided-setup.md) for the full workflow.
:::
