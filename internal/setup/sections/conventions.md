## Conventions Section Guide

The `conventions` key is a map of named convention definitions. Each
convention tells the agent what rules apply to a part of the codebase.

### Think Tasks, Not Packages

Conventions should map to tasks an engineer performs, not mirror the
directory tree. Ask: "What would someone search for?"

```yaml
# Bad — mirrors the package tree
server:
  scope: "internal/server/**"
  description: "MCP protocol server implementation."
config:
  scope: "internal/config/**"
  description: "Config loading and validation."

# Good — task-oriented
config-schema:
  scope: "internal/config/**"
  description: |
    Versioned YAML config with strict parsing — unknown keys are errors.
    Adding a new config field:
      1. Add to the types in the latest config version
      2. Add a default if needed
      3. Add validation
tool-engine:
  scope: "internal/tools/**"
  description: |
    Tool execution, template rendering, and sandbox.
    Adding a new template function:
      1. Implement in templating.go
      2. Test in templating_test.go
      3. Document in config.md
```

### Describe Workflows, Not Inventories

The AI can read code to see what files exist. Conventions should
describe **what to do** — checklists for common tasks, the order of
operations, which files to touch and why.

```yaml
# Bad — the AI already knows this from reading the code
description: >-
  server.go: read-dispatch-write loop. types.go: JSON-RPC types.

# Good — tells the agent what to do
description: |
  Tool execution, template rendering, and sandbox.
  Adding a new template function:
    1. Implement the function and register it in templating.go
    2. Test in templating_test.go
    3. Document in config.md (built-in functions table)
    4. Update example YAMLs in docs/user/examples/ if user-facing
```

### Point to Docs, Don't Restate Them

If rules already live in a doc file, reference them — don't duplicate
them in the description. Duplication means maintaining rules in two
places; they'll drift.

```yaml
# Bad — restates what's in testing.md
description: |
  Tests use stdlib table-driven style with t.Run sub-tests.
  Use t.Helper() on shared assertion functions.

# Good — points to the source of truth
description: |
  This project has coding and testing conventions.
  Read the docs before writing or modifying code.
docs:
  - source: docs
    paths: ["docs/development/testing.md"]
```

**Exception:** Small projects without separate doc files can define
rules directly in conventions. Refactor into doc references when
the project grows.

### Avoid Brittle References

Descriptions that name specific types, leaf file paths, or internal
functions create maintenance traps. When those internals change, the
convention becomes a source of wrong advice.

Use intent-based language — describe *what* to find, not the exact
symbol name. The AI can locate the current implementation.

```yaml
# Brittle — renames or refactors break these references
description: |
  Adding a new config field:
    1. Add the field to config/v1/types.go
    2. Update applyDefaults() in config/v1/helpers.go
    3. Add validation in config/v1/validate.go

# Stable — says what to do without pinning to symbols
description: |
  Adding a new config field:
    1. Add to the types in the latest config version
    2. Add a default if needed
    3. Add validation
```

**Workflow steps vs. symbol references.** Naming files as steps in a
workflow is fine — "implement in templating.go, test in
templating_test.go". What drifts is pinning to internal symbols:
struct names, function signatures, internal constants. The AI can
find the current struct name; it can't recover from trusting a
renamed one.

### Name for Searchability

Convention names appear in search results and relation targets.
Choose names an engineer would actually search for.

```yaml
# Generic            # Searchable
tools:               tool-engine:
config:              config-schema:
sources:             sources-search:
```

### Fewer Conventions Beat More

Every convention competes for search ranking. If nobody would search
for a convention in isolation, it doesn't need to exist separately.
Consolidate related concerns.

### Scopes Must Match Real Directories

Convention scopes are matched against file paths. Scope globs that
point to nonexistent directories are dead weight. Audit against the
actual directory layout.

### Point to Specific Docs, Not Everything

Each convention should reference only the docs relevant to that
scope. If every convention points to `architecture.md`, it's noise —
drop the reference unless it contains information specific to that
convention's workflows.

```yaml
# Noisy — architecture.md on everything
server:
  docs: ["docs/development/architecture.md"]
config:
  docs: ["docs/development/config.md", "docs/development/architecture.md"]

# Targeted — only docs that help with this convention's tasks
config-schema:
  docs: ["docs/development/config.md"]
code-style:
  docs: ["docs/development/README.md", "docs/development/testing.md"]
```

### Keep Descriptions Accurate, Not Aspirational

Convention descriptions must reflect the code **as it is today**.
Don't describe planned features, removed code, or inaccurate flags.
An AI that trusts stale conventions will generate wrong code.

### Use Relations for Cross-References

When two conventions interact (e.g., config schema affects tool
definitions), link them with `relations`. The agent can then navigate
from one context to related contexts.

### Document Cross-System Relationships

When a config pulls in multiple sources spanning different systems,
conventions are the right place to capture how those systems relate.
No amount of code reading can produce operational topology — it
lives in the team's understanding.

```yaml
deployment-flow:
  scope: "*"
  description: |
    This config spans three systems that form a deployment chain:
      - infra (Terraform): provisions the EKS cluster
      - platform (ArgoCD + CI): builds and deploys
      - app: deployed via ArgoCD Application CRD

    When troubleshooting deployment failures, trace backwards:
    app manifest → ArgoCD sync → CI pipeline → cluster state.
```
