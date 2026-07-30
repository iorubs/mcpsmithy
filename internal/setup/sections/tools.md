## Tools Section Guide

The `tools` key is a map of named tool definitions. The field
reference below covers every field; this guide covers how to use
them well.

### Templates

Every tool requires a `template:` field; a Go `text/template`
string. Templates can call built-in functions and access the project context via `{{ .mcpsmithy }}`. Params and options are accessible as `{{ .paramName }}`.


### Let Descriptions Teach

Tool descriptions appear in `tools/list`; the agent reads them
every session. Write descriptions that say **when** and **why** to
call each tool, not just what it returns. "Returns project info" is
weak. "Returns project overview and file structure. Call at the start
of every session." tells the agent when to use it.

### Use Param Descriptions to Guide Inference

The agent sees param names and descriptions before deciding what
value to pass. Use them to bridge the gap between what the user
provides and what the template actually needs.

For example, if a tool calls `http_get` against a CI API but the
user will paste a browser URL, the param name and description should
tell the agent how to transform it:

```yaml
params:
  - name: "traceUrl"
    type: "string"
    required: true
    description: >-
      Use the API trace format:
      https://<host>/api/v4/projects/<url-encoded-project-path>/jobs/<job_id>/trace.
      Extract the project path and job ID from the standard job page URL.
```

The name `traceUrl` already signals it's not a raw browser URL.
The description spells out the exact format. Without these hints,
cheaper models may pass the browser URL directly and blame
authentication when the request fails.

This applies broadly; whenever the template expects a value in a
different format than what the user naturally provides, encode that
in the param name and description.

Where possible, keep the full URL out of params entirely —
hardcode the base URL in the template and let the agent supply only
the variable parts (an ID, a project path, etc.). When the URL
must come from a param, set the `urlAllowList` tool option to
restrict which hosts the HTTP functions can reach.

### Encode Opinions, Not Syntax

The AI knows `go test` and `npm run build`. What it doesn't know is
your project's rules. A `project_commands` template tool should
encode **how the AI should operate**; commands plus behavioral
rules like "always run tests with -cover" or "do not use sed for
file edits."

### Don't List Aspirational Commands

If a command doesn't work yet, don't put it in the config. The AI
will try to run it, fail, and waste time debugging a tool that
doesn't exist. Keep the config honest about the current state.

### Combine Conventions and Search

If you index sources, make sure some tool queries them. A common
pattern combines `conventions_for` with `search_for` in a template
so one call gives both the rules and relevant content.

### `file_read` Is for Inaccessible Content

The AI can already read local files with its editor tools. Use
`file_read` in templates only for content the AI cannot access
directly; files outside the workspace or generated content.

### Parse JSON Responses Instead of Grepping Them

When an HTTP function returns JSON, pipe it through `from_json`
rather than matching it with `grep`. The result is a value the
template can index and range over, so the tool returns only the
fields that matter instead of a wall of raw JSON:

```yaml
template: |
  {{ $r := from_json (http_get .url) }}
  Status: {{ $r.status }}
  {{ range $r.items }}- {{ .id }}: {{ .title }}
  {{ end }}
```

Reach for `grep` when the response is plain text (CI logs, for
example); reach for `from_json` when it is an API payload.

Two things to know. `from_json` fails the tool call when the body is
not valid JSON, so an endpoint that returns HTML on error surfaces as
a clear failure rather than silently matching nothing. And because a
response shape is unknown until the call runs, templates calling
`from_json` skip the config-load dry-run: misspelled param names and
wrong argument counts in those templates surface on the first tool
call rather than at startup. Syntax errors and unknown function names
are still caught at load.

### HTTP Authentication

`http_get`, `http_post`, and `http_put` authenticate automatically from
the credentials file (`~/.mcpsmithy/credentials` by default), matched on
the request hostname. It supports Bearer, Basic, vendor schemes such as
`Authorization: Token <value>`, and APIs that use their own header
instead of `Authorization`. Hosts with no entry fall back to the
deprecated `~/.netrc`. See the guide section for the file format.

Credentials are never exposed to templates or params, so the agent
cannot read or redirect them. When a URL comes from a param, set the
`urlAllowList` tool option so credentials cannot be sent to a host the
agent chooses.

### Tool Sets by Use Case

- **Docs Assistant.** `get_convention`, `search`, `read_doc`
- **Project Awareness.** `find_convention`, `search`
- **Support & Troubleshooting.** `project_info`, `find_convention`, `search`, `ci_log`
- **Agentic Application.** `search`, `find_convention`, `api_read`, `api_create`, `api_update`

Full YAML examples for each deployment mode are below.
