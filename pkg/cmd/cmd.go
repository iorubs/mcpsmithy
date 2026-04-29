// Package cmd implements the CLI subcommands.
package cmd

import (
	"log/slog"
)

// LogLevel represents a supported log verbosity level.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// CLI is the root Kong CLI struct.
type CLI struct {
	LogLevel LogLevel `help:"Log level (one of: ${enum})." default:"info" enum:"debug,info,warn,error" short:"l"`
	Commands
}

// Commands holds the subcommands and config, safe to embed.
type Commands struct {
	Serve    ServeCmd    `cmd:"" help:"Run MCP server."`
	Validate ValidateCmd `cmd:"" help:"Validate config file."`
	Sources  SourcesCmd  `cmd:"" help:"Manage sources."`
	Setup    SetupCmd    `cmd:"" help:"Start config-authoring MCP server assistant."`
}

type ConfigFlag struct {
	Config string `help:"Path to config." default:".mcpsmithy.yaml" type:"path" short:"c"`
}

// ParseLogLevel maps the CLI log-level flag to slog.Level.
func ParseLogLevel(l LogLevel) slog.Level {
	switch l {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
