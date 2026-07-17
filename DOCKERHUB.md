# MCPSmithy

Drop a `.mcpsmithy.yaml` in any repo and get a project-aware [MCP](https://modelcontextprotocol.io) tool server, no custom code required. MCPSmithy reads a declarative YAML config and serves tools tailored to your codebase, docs, or data sources.

## Usage

Run it directly against your project, mounted read-only:

```bash
docker run --rm -i \
  -v "$(pwd)":/project:ro \
  -w /project \
  smithylabs/mcpsmithy:latest \
  serve
```

### VS Code

Add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "mcpsmithy": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "${workspaceFolder}:/project:ro",
        "-w", "/project",
        "smithylabs/mcpsmithy:latest",
        "serve"
      ]
    }
  }
}
```

## Tags

- `latest` — most recent stable release
- `X`, `X.Y`, `X.Y.Z` — semver-pinned releases

## Links

- [GitHub](https://github.com/iorubs/mcpsmithy)
- [Docs](https://iorubs.github.io/mcpsmithy/)
