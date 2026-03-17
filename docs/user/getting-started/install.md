---
sidebar_position: 1
---

# Install

## Binary

### Download from GitHub Releases

Download `mcpsmithy-<os>-<arch>` from the
[latest release](https://github.com/iorubs/mcpsmithy/releases/latest).

### go install

```sh
go install github.com/iorubs/mcpsmithy/cmd/mcpsmithy@latest
```

### Connect your agent

Optionally, move the binary to a directory in your `PATH`:

**VS Code** — add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "mcpsmithy": {
      "command": "mcpsmithy",
      "args": ["serve"]
    }
  }
}
```

<!-- TODO: add binary connection examples for Claude Desktop, Cursor, GitHub Copilot CLI -->

## Docker

```sh
# TODO: confirm final image name/registry. For now you can build from source.
docker build mcpsmithy:latest
```

### Connect your agent

**VS Code** — add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "mcpsmithy": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "${workspaceFolder}:/project:ro",
        "-w", "/project",
        "mcpsmithy:latest",
        "serve"
      ]
    }
  }
}
```

<!-- TODO: add Docker connection examples for Claude Desktop, Cursor, GitHub Copilot CLI -->

## Next steps

Once connected, you'll need a `.mcpsmithy.yaml` config. See the
[Use Cases](./use-cases/docs-assistant) section to find a scenario
that fits your needs, then follow the tip at the bottom to generate
your config with your agent.
