# Server

## mcpsmithy

Project-agnostic MCP tool server. Reads .mcpsmithy.yaml and serves MCP tools over stdio.

```
mcpsmithy <command> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-h, --help` | `bool` | — | Show context-sensitive help. |
| `-c, --config` | `string` | `.mcpsmithy.yaml` | Path to config file. |
| `-l, --log-level` | `enum(debug,info,warn,error)` | `info` | Log level. |


### serve

Start the MCP server.

```
mcpsmithy serve [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--transport` | `enum(stdio,http)` | `stdio` | Transport to use. |
| `--addr` | `string` | `:8080` | Listen address (HTTP transport only). |
| `--watch` | `bool` | `false` | Watch config file and hot-reload on change. |


### validate

Validate config file.

```
mcpsmithy validate [flags]
```

### sources

Manage sources.

```
mcpsmithy sources [flags]
```

#### sources pull

Fetch external sources and write them to disk.

```
mcpsmithy sources pull [flags]
```

### setup

Start the config-authoring assistant (no config required).

```
mcpsmithy setup [flags]
```
