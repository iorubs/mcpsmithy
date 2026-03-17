---
sidebar_position: 4
---

# Project Awareness

## User story

As an **Engineer**, I want my AI assistant to automatically know my
project's conventions and structure so that generated code follows the
right patterns — even with cheaper models.

## Goals

- The agent reads the right docs before touching a given file path
- Conventions and their relationships are discoverable by the agent
- No prompt engineering or manual context pasting per session

## Technical overview

**Deployment:** stdio (local). The agent already has workspace access
and its own file/search tools. mcpsmithy adds the project-specific
context layer the agent is otherwise missing.

**Data sources:**
- Local source code with `index: false` — provides structure only
- Local docs indexed for ranked search

**Conventions:** Map file-path globs to docs and rules. Use `relations`
to link conventions together so cross-cutting concerns (e.g. changing
an API also requires an integration test) are surfaced automatically.

**Tools needed:**
- `find_convention` — returns the docs, convention rules, and workflows that apply to a file path
- `search` — ranked search across all indexed sources; conventions are surfaced first so the agent gets relevant context it might not know to ask for

For full YAML examples of each source type, tool template, and convention pattern, see the [config reference](../../reference/config/README.md).

:::tip Generate this config with your agent
Run `mcpsmithy setup` in your project root, then share this story with
your agent. Before prompting, have ready:

- The key areas of your codebase and where their docs live
- Any cross-cutting rules (e.g. every API needs an integration test)

Then use a prompt like:

> Set up mcpsmithy for this project to enforce coding conventions.
> Explore the project structure, identify the main areas and their docs,
> and write a config with sources, conventions with relations, and a
> `find_convention` tool.

See [Assisted setup](../guided-setup.md) for the full workflow.
:::
