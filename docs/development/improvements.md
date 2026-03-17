# Improvements & Future Initiatives Roadmap

Tracked feature ideas, technical debt, and enhancements for
MCPSmithy.

## Improvements

### OpenTelemetry observability

**Problem:** Tool call log lines are isolated events. There is no way to correlate them back to the upstream request that triggered them — whether that's a user request hitting a backend API, a job in an agentic workload, or a pipeline step. Each `tool/call done` line has latency and outcome, but no link to the broader execution context it was part of.

**Value:** If MCPSmithy is part of an application stack, e.g. a backend delegates to an agent, the agent calls mcpsmithy tools over HTTP — OTel `traceparent` propagation would let tool calls appear as spans within the originating request's trace. You'd get end-to-end visibility across services: user request → backend → agent → tool call, all in one trace. That's meaningfully different from what log aggregation alone can provide.

**Why parked:** `slog` JSON output is sufficient for the current use cases — local/stdio for individual engineers, and shared HTTP for doc server deployments. The application stack pattern (mcpsmithy embedded inside a product's agentic workload) is a real use case but no one is running it yet. OTel is only worth building once that adoption exists — the SDK adds meaningful binary weight and the integration requires operator infra (Jaeger, Tempo, etc.) to be in place anyway. Revisit when a concrete deployment requires cross-service trace correlation.

---

### Plugin / module system

**Problem:** Sources are limited to three built-in types (`local`, `git`, `scrape`) and tool template functions are a fixed set. There is no way to add external data sources or custom tool logic without modifying core code. Both extension points share the same shape, but there is no shared interface or registration mechanism that would allow logic to live outside the main binary.

**Value:** The core stays short and security-auditable — config parsing, validation, sandbox, MCP protocol, template engine, BM25 indexing, transport. Specialized logic like `http_get` (HTTP + .netrc auth), `scrape` (HTML parsing), and `git` (clone via local git binary) could migrate to officially maintained plugins. Community authors could add integrations without forking the project.

**Why parked:** The current three source types and built-in functions cover all concrete use cases today. A plugin system is an architecture decision with long-lived API commitments — designing the contract, versioning story, and security boundary before there is a forcing use case risks over-engineering.

---

### Action-capable tool type

**Problem:** mcpsmithy only supports read-only operations. Teams that want AI agents to trigger actions — creating tickets, updating configuration, calling internal APIs — must build separate tooling or rely on the agent's own capabilities, which may lack access to internal systems or safe credential management.

**Value:** Would make mcpsmithy a more complete operations platform for a project — not just context retrieval but also audited action execution. Reduces the need for separate agent skill tooling.

**Why parked:** The boundary between what the MCP server should do vs what the agent should handle natively is genuinely unclear — agents already have HTTP and function-calling capabilities, so adding action execution here risks duplicating that layer without clear benefit. It also meaningfully expands the trust surface of the server, which currently has a clean read-only threat model. Needs a concrete use case and a security/capability design pass before it belongs on the active roadmap.

---

### Shell Command Execution

**Problem:** Some use cases require running project-specific commands to gather context — running tests, querying build artifacts, generating dynamic output — that can't be served by file reading or templates alone.

**Value:** Would unlock a wide range of dynamic context use cases that are currently impossible without a separate tool server. Power users operating trusted, single-user setups would benefit most.

**Why parked:** Shell execution is the largest security surface in this class of tools — it allows arbitrary code execution on the host. Doing this safely requires a meaningful set of mitigations (sandboxing, output limits, allowlists, credential isolation, replay safety) all designed and implemented together as a coherent security model. The current read-only tools cover the primary use cases without that risk. Parked until a concrete use case emerges that genuinely can't be solved without it.

---

### Chunking improvements

**Problem:** The current chunking strategies have two gaps. First, there is no upper bound on chunk size — a very large file becomes a single chunk that dilutes search scores. Second, the `section` strategy only works correctly for Markdown; code files have natural boundaries too, but there is no way to split them at the right granularity today.

**Value:** Bounded chunk sizes would prevent large files from dominating search results. Per-language chunking would improve search precision for code sources, returning the relevant function or declaration rather than the whole file.

**Why parked:** For the token-bound problem, splitting at fixed token boundaries destroys semantic meaning — a coherent section or whole file is always a better unit for the current search approach. This tradeoff only shifts once embedding-based search is added, so token chunking should be implemented alongside that, not before. For per-language chunking, auto-detection is correct and safe at current scale — whole-file results with preview snippets are sufficient for navigation. Revisit when code search quality becomes a demonstrated pain point.

---

### Per-tool sandbox scoping and multi-root workspaces

**Problem:** All tools share the same sandbox root. There is no way to restrict an individual tool to a sub-directory — e.g. a `read_frontend` tool limited to `frontend/` and a `read_backend` tool restricted to `backend/`. Monorepo setups have a related question: whether to place a single config at the root (using glob patterns targeting sub-projects) or manage separate configs per sub-project.

**Value:** Would allow a single server instance to serve monorepo setups with stricter per-tool access boundaries, simplifying configuration for teams that prefer one server over many.

**Why parked:** Both concerns are already addressed by running separate MCP server instances — each with its config at the relevant sub-project root. This gives per-sub-project sandboxing today with no additional implementation. True per-tool scoping within a single server adds meaningful complexity and is premature before a concrete use case emerges that genuinely can't be served by separate instances.

---

### Environment variable substitution in config

**Problem:** There is no way to inject dynamic values — environment-specific paths or tokens — into the config without modifying the file directly. This makes it harder to share configs across environments or keep sensitive values out of version control.

**Value:** Would allow configs to be portable across environments without modification and keep sensitive values out of the config file itself.

**Why parked:** Path portability is already handled by relative paths and Docker mounts. The credential injection use case belongs to the action tools design — expansion at parse time is the wrong shape for secrets, since the config file is readable by the agent and any injected variable names would be visible to it. Any future design here must address that exposure at the same time.

---

### MCP Prompts support

**Problem:** Conventions and design docs are currently discovered through the tool-based workflow — agents search with `search_for` or call a convention listing tool. This works but requires multiple steps and offers limited UX for clients.

**Value:** The MCP spec defines a prompts capability that would allow conventions and docs to be exposed as native, browsable prompts (`prompts/list` and `prompts/get`) — no search needed. Conventions become discoverable as menu items, and non-agent users (humans in VS Code) can browse project context directly.

**Why parked:** Client support for MCP prompts is sparse — most MCP clients don't support the prompts capability yet, limiting the practical benefit to users today.

---

### Composite Tool Type

**Problem:** Some queries naturally span multiple tools — for example, looking up a convention and then reading the related doc. Agents handle this through sequential tool calls, but each round-trip adds latency and requires the agent to manage the sequencing explicitly.

**Value:** Reduced latency for known multi-step patterns; simpler agent prompts for fixed, deterministic workflows.

**Why parked:** In practice, LLM clients are better orchestrators than a fixed sequence — they can react to intermediate results, retry failed steps, and adapt based on what each tool returns. A composite tool hides that intermediate state and removes the agent's ability to course-correct.

---

### Progress notifications

**Problem:** Long-running operations provide no feedback to the client, making them appear frozen.

**Value:** Users see real-time feedback on long operations, improving perceived performance and preventing timeout assumptions.

**Why parked:** Today's tool surface (search, file reads, templates) completes in milliseconds with no external dependencies. Progress reporting only becomes valuable when tools genuinely require background work — async fetches, external API calls, compute-heavy operations.

---

### `logging/setLevel` capability

**Problem:** In remote/HTTP deployments, there is no way to change server log verbosity without restarting the container. Ambient server-side events — index build failures, source fetch errors — are invisible to the client, appearing only on the server's stderr, which is inaccessible when the server runs remotely.

**Value:** Runtime verbosity control without restart would help operators debug remote deployments. `notifications/message` forwarding would surface server health events directly in the client's interface.

**Why parked:** `--log-level` at startup covers the verbosity use case. The more useful half of this feature — forwarding server log events to the client via `notifications/message` — has no meaningful client support today. Revisit if a client we're targeting starts consuming those notifications.

---

### MCP Resources support

**Problem:** Browsing project files requires calling the `file_read` tool, which isn't intuitive for clients that support native resource browsing.

**Value:** Enhanced UX for clients — files appear in a navigable resource tree rather than requiring tool invocations. Real-time file change notifications. Better parity with traditional client interfaces.

**Why parked:** The functionality already works via `file_read`. Resources are a UX layer that depends on client support. Revisit if client adoption of Resources becomes widespread.

---

### Embedding-based semantic search and large corpus support

**Problem:** The current search fails when the user doesn't know the right vocabulary — a query like "how do I ship my app?" scores zero against a doc titled "Release Procedure" because the terms don't overlap. Embedding-based search doesn't have this limitation, but its storage and compute costs grow with corpus size in a way keyword search doesn't.

**Value:** Embedding-based search matches by meaning rather than exact terms, so vocabulary mismatch stops being a blocker. At project scale the infrastructure cost is modest — a dedicated vector index only becomes necessary at much larger corpus sizes than current use cases require.

**Why parked:** A capable agent running against a well-configured mcpsmithy instance can already bridge much of the vocabulary gap by rewriting queries before calling `search_for`. The infrastructure cost (embedding model dependency, vector storage, token-bounded chunking) is high relative to the marginal search quality improvement at current corpus sizes.

---

## Future Initiatives Roadmap

High-level strategic initiatives that evolve mcpsmithy from a read-only context server into a full AI platform. These are larger efforts that may become sibling projects; they live here until those repos exist.

---

### Agent Support

mcpsmithy today is a **read-only context provider**. The natural next step is an **agent execution layer** — so autonomous agents can act on a codebase while staying grounded in its conventions, structure, and docs.

##### Vision

```
Agent Runner (new)          ← plan → act → evaluate loop
  ↓ calls
MCP Client Interface        ← consumes mcpsmithy tools for context
  ↓ calls
mcpsmithy (existing)      ← conventions, sources, file scan, etc.
```

An agent runner would use mcpsmithy as its project-awareness backbone, adding autonomous planning and action on top of the existing context layer. Key capabilities this requires:

- **Write actions** — modify files, run commands, and call APIs
- **Agent loop** — goal-driven plan-act-evaluate cycle across steps
- **Guardrails** — human-in-the-loop approval for destructive operations
- **Memory** — state across steps to track progress and avoid redundant work
- **Toolsets per context** — different tool surfaces for different agent roles
- **Remote transport** — network-accessible for agents running outside the local machine

##### Open Questions

- Build a custom agent loop or integrate an existing agent framework as an MCP client?
- How to scope write permissions?
- Should agent configs live in `.mcpsmithy.yaml` or a separate manifest?

---

### Specialized Model Routing

Most of what makes frontier models expensive is compensating for lack of context. With the right conventions, code patterns, and docs injected at the right time, smaller models can perform nearly as well on domain-specific tasks. mcpsmithy is already building that context layer — model routing is the efficiency payoff.

#### Vision

Route tasks to the right model based on complexity: routine work goes to fast, cheap models; architecture and complex refactoring go to frontier models. Routing rules would be config-driven and project-specific. This could live inside the agent layer or as a standalone sibling service — the right answer depends on how the agent layer evolves.

#### Open Questions

- Sibling service or integrated into the agent layer?
- How to define task complexity thresholds for routing decisions?
- How to measure whether a cheaper model's output is "good enough" for a given task?

---

### Platform Operator (Kubernetes CRDs)

As the stack grows from context server → agents → model routing, deployment complexity grows with it. A Kubernetes operator is the natural unification layer — the config is already YAML and already CRD-shaped (`apiVersion`/`kind` patterns).

#### Vision

Each layer in the stack maps to a managed resource, with operational concerns like health checks, RBAC, and GitOps handled by the cluster rather than custom code. The operator is its own repo and binary; it consumes mcpsmithy as the underlying runtime. This only makes sense once the previous layers have traction and teams need multiple instances with different configs, agents, and model routing.

#### Open Questions

- How much of the operator logic lives in the operator vs. in mcpsmithy itself?
- Should the operator manage model endpoints, or delegate to existing inference infrastructure?

---

### Template Registry

mcpsmithy today requires whoever sets up a project to understand the platform — sources, conventions structure, tool surfaces. A **template registry** externalizes that expertise: curated templates that anyone can pull to bootstrap a project or an agentic workload.

#### Vision

Templates serve two purposes: **project setup** (structure, sources, and conventions for a domain) and **agentic workloads** (tool surfaces, guardrails, and behavioral boundaries for agents). The same template that scaffolds a project defines how agents operate against it. A registry lowers the floor beyond engineers — PMs, designers, and non-technical practitioners can start from something production-ready without understanding the underlying mechanics. Evolution: curated examples in docs today → community templates in-repo → hosted versioned registry → base configs for operator CRDs.

#### Open Questions

- Who qualifies as a maintainer, and how are templates reviewed and versioned?
- Should templates be composable (base + overrides) or standalone?
- How does template drift get managed as frameworks and tooling evolve?
