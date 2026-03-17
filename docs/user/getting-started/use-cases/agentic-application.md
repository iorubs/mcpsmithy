---
sidebar_position: 3
---

# Agentic Application

## User story

As a **Backend engineer**, I want to use mcpsmithy as the knowledge
and context layer inside my application's agentic workload so that my
application can provide AI-powered features backed by any knowledge I
choose — without building or maintaining a custom tool server.

## Goals

- Agent has a persistent, searchable knowledge layer across any sources you choose
- Agent can pull live data from APIs on demand
- No custom server code to write or maintain
- Multiple agents or workloads can share the same instance
- Clean separation: mcpsmithy owns knowledge, the application owns everything else

## Technical overview

**Deployment:** HTTP/SSE. The agent connects to mcpsmithy over HTTP as
one of its tool providers. mcpsmithy serves the knowledge layer; the
application handles orchestration, actions, and user interaction.

**Data sources:**
- Internal docs (`project.sources.local`)
- Scraped external sites (`project.sources.scrape`)
- API or endpoint data (`project.sources.http`) — fetched and indexed;
  can sync periodically. Use when data is too large for a live fetch or
  when you need to search across it efficiently.

**Conventions:** Encode domain rules the agent must follow when
operating in specific areas — service boundaries, required reading
before making changes, etc.

**Live data:** For real-time data that must always be fresh, use
`http_get` in a tool template. Unlike sources, it's never indexed —
the agent gets the raw response on each call. Pair with `grep` in the
template to filter large responses down to what the agent actually needs.

**Tools needed:**
- `search` — ranked search across all indexed sources; conventions are surfaced first, letting you inject the right context into the agent's workflow automatically
- `find_convention` — returns the docs, convention rules, and workflows that apply to a file path
- `get_status` — fetches live data from an API on demand (always fresh, not indexed)

For full YAML examples of each source type, tool template, and convention pattern, see the [config reference](../../reference/config/README.md).

:::tip Generate this config with your agent
Run `mcpsmithy setup`, then share this story with your agent. Before
prompting, have ready:

- The internal docs or service directories you want indexed
- Any external sites to scrape
- API or service endpoints to index at startup (`http` source)
- API endpoints you want fetched live on demand (`http_get` tool)

Then use a prompt like:

> Set up mcpsmithy as a knowledge layer for this agentic application.
> I'll give you the sources: [list indexed sources] and [list any http
> sources for startup fetch]. For live data, create a tool using
> `http_get` that fetches on demand — use `grep` to filter the response.
> Add `search`, `find_convention`, and the live-fetch tool.
> The server will run in HTTP/SSE mode via Docker.

See [Assisted setup](../guided-setup.md) for the full workflow.
:::
