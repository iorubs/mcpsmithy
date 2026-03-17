# MCPSmithy

> Drop a `.mcpsmithy.yaml` in any repo. Run the binary. Every
> MCP-compatible AI assistant gets project-aware tools — no custom
> code required. Build MCP tools on the fly for any data source,
> docs, or codebase.

![mcpsmithy forge](docs/images/forge.png)

**MCPSmithy** is a single Go binary that reads a declarative YAML
config file and serves a fully functional
[MCP](https://modelcontextprotocol.io) tool server. It works for
any software project — no language or framework assumptions baked in.

## Quick Start

```bash
# Build
docker build -t mcpsmithy:latest .

# Or from source (Go 1.26+)
go build -o bin/mcpsmithy ./cmd/mcpsmithy
```

See the [Install guide](docs/user/getting-started/install.md) for
editor integration and detailed setup.

## Documentation

### For Users

| | |
|---|---|
| [Docs site](https://iorubs.github.io/mcpsmithy/) | Documentation overview |

### For Contributors

| | |
|---|---|
| [.mcpsmithy.yaml](.mcpsmithy.yaml) | Project sources, conventions, tools, and commands for AI assistants |
| [Development Guide](docs/development/README.md) | Architecture, CLI, config schema, testing, and security |
