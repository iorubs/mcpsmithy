---
sidebar_position: 2
---

# Support & Troubleshooting Assistant

## User story

As a **Support Engineer** working across large or distributed
codebases, I want my AI assistant to quickly orient itself in unfamiliar
repos and locate relevant code, configs, and docs — and when available,
pull CI logs directly — so that I can troubleshoot issues without
memorising every repo's layout or switching between tools.

## Goals

- Agent orients itself immediately in any repo
- Conventions surface the right runbook or doc for a given file path
- Agent can search across multiple indexed repos and external sources
- When CI logs are available, agent can fetch and parse them without leaving the chat

## Technical overview

**Deployment:** stdio (local). The agent already has workspace access
via the host editor. mcpsmithy adds orientation and cross-repo search
on top — it doesn't replace the agent's existing file tools.

**Orientation:** `project_info` gives the agent an immediate overview
of the repo layout and what each source contains — so it can orient
itself in an unfamiliar codebase without reading every directory.

**Data sources:**
- Local source code with `index: false` — provides structure, not search
- Remote runbooks and docs via `project.sources.git` — or `project.sources.http` for very large repos (pull a GitHub/GitLab archive instead of cloning)
- External wikis or status pages via `project.sources.scrape`

**Conventions:** Map service paths to their runbooks so the agent
knows what to read before investigating a given area.

**Live API reads (optional):** If your CI/CD system or any other
service exposes an API, a tool using `http_get` lets the agent fetch
data directly — CI logs for a given pipeline, Slack channel history
for a support thread, or a status page summary. Use `grep` in the
template to filter the output down to relevant lines before it reaches
the agent.

**Tools needed:**
- `project_info` — repo layout and source descriptions
- `find_convention` — returns the docs, convention rules, and workflows that apply to a file path
- `search` — ranked search across all indexed sources; conventions are surfaced first so the agent gets the right context before digging into code
- `ci_log` — fetches CI/CD logs for a given run or pipeline via `http_get`
- `slack_history` — fetches recent messages from a Slack channel via `http_get`; useful for reviewing support threads or incident chatter

For full YAML examples of each source type, tool template, and convention pattern, see the [config reference](../../reference/config/README.md).

:::tip Generate this config with your agent
Run `mcpsmithy setup`, then share this story with your agent. Before
prompting, have ready:

- A list of the repos to index (runbooks, service docs, or both)
- Which service paths map to which runbook or doc area
- If you have a CI API: the endpoint and any auth needed for log fetching

Then use a prompt like:

> Set up mcpsmithy for support troubleshooting across this platform.
> Here are the repos to index: [list them]. Map each service path to
> its runbook via conventions. Expose `project_info`,
> `find_convention`, `search`, and `ci_log` tools (ci_log is optional
> — only if I provide a CI API endpoint).

See [Assisted setup](../guided-setup.md) for the full workflow.
:::

:::note Adapting this for user-facing support
With minimal changes this config works for **end-user support** too —
even when you have no dedicated docs site. The main difference is which
files you index: narrow the source globs to docs, READMEs, and
changelogs (exclude internal implementation files), and add a `read_doc`
tool so the agent can surface specific files directly. The repo becomes
your support knowledge base with no extra tooling.
:::
