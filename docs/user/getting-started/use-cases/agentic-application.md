---
sidebar_position: 3
---

# Agentic Application

## User story

As a **Backend engineer**, I want to use mcpsmithy as the knowledge,
context, and API integration layer inside my application's agentic
workload so that my application can provide AI-powered features —
reading live data, creating resources, and triggering actions — without
building or maintaining a custom tool server.

## Goals

- Agent has a persistent, searchable knowledge layer across any sources you choose
- Agent can read live data, create resources, and trigger actions via HTTP APIs
- Sensitive credentials stay in `.netrc` — never passed as params the agent controls
- No custom server code to write or maintain
- Multiple agents or workloads can share the same instance
- Clean separation: mcpsmithy owns knowledge and API access, the application owns everything else

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

**API access:** Use `http_get`, `http_post`, and `http_put` in tool
templates to give the agent direct API access. Hardcode the base URL in
each template and expose only the variable parts as params; set the
`urlAllowList` tool option when the URL must come from a param.
Pair `http_get` with `grep` to filter large responses before they
reach the agent.

**Tools needed:**
- `search` — ranked search across all indexed sources; conventions are surfaced first, letting you inject the right context into the agent's workflow automatically
- `find_convention` — returns the docs, convention rules, and workflows that apply to a file path
- `api_read` — fetches live data from an API on demand (always fresh, not indexed); uses `http_get`
- `api_create` — creates a resource or triggers an action via `http_post`
- `api_update` — updates an existing resource via `http_put`

For full YAML examples of each source type, tool template, and convention pattern, see the [config reference](../../reference/config/README.md).

:::tip Generate this config with your agent
Run `mcpsmithy setup`, then share this story with your agent. Before
prompting, have ready:

- The internal docs or service directories you want indexed
- Any external sites to scrape
- API or service endpoints to index at startup (`http` source)
- API endpoints the agent should reach live — reads (`http_get`),
  creates (`http_post`), and updates (`http_put`)
- Which fields the agent should supply vs. which should be hardcoded

Then use a prompt like:

> Set up mcpsmithy as a knowledge and API layer for this agentic
> application. I'll give you the sources: [list indexed sources] and
> [list any http sources for startup fetch]. For live data, create
> tools using `http_get`, `http_post`, and `http_put` — hardcode the
> base URL in each template. Add `search` and `find_convention`.
> The server will run in HTTP/SSE mode via Docker.

See [Assisted setup](../guided-setup.md) for the full workflow.
:::
