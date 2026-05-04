# Server

## mcpsmithy

Project-agnostic MCP tool server. Reads .mcpsmithy.yaml and serves MCP tools over stdio.

```
mcpsmithy <command> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-h, --help` | `bool` | — | Show context-sensitive help. |
| `-l, --log-level` | `enum(debug,info,warn,error)` | `info` | Log level (one of: debug,info,warn,error). |


### serve

Run MCP server.

```
mcpsmithy serve [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-c, --config` | `string` | `.mcpsmithy.yaml` | Path to config. |
| `--transport` | `enum(stdio,http)` | `stdio` | Transport to use (one of: stdio,http). |
| `--addr` | `string` | `:8080` | Listen address (HTTP transport only). |
| `--watch` | `bool` | `false` | Watch config file and hot-reload on change. |


### validate

Validate config file.

```
mcpsmithy validate [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-c, --config` | `string` | `.mcpsmithy.yaml` | Path to config. |


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

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-c, --config` | `string` | `.mcpsmithy.yaml` | Path to config. |


### setup

Start config-authoring MCP server assistant.

```
mcpsmithy setup [flags]
```
