// Package cmd implements the CLI subcommands.
package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	LogLevel LogLevel `help:"Log level." default:"info" enum:"debug,info,warn,error" short:"l"`
	Commands
}

// Commands holds the subcommands and config, safe to embed.
type Commands struct {
	Config   string      `help:"Path to config file." default:".mcpsmithy.yaml" type:"path" short:"c"`
	Serve    ServeCmd    `cmd:"" help:"Start the MCP server."`
	Validate ValidateCmd `cmd:"" help:"Validate config file."`
	Sources  SourcesCmd  `cmd:"" help:"Manage sources."`
	Setup    SetupCmd    `cmd:"" help:"Start the config-authoring assistant (no config required)."`
}

// ProjectRoot resolves the project root from the config file location.
// The root is always the directory containing the config file.
func (cli *CLI) ProjectRoot() (string, error) {
	if cli.Config != "" {
		info, err := os.Stat(cli.Config)
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("config path %q is a directory, not a file", cli.Config)
			}
			return filepath.Abs(filepath.Dir(cli.Config))
		}
	}
	return filepath.Abs(".")
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
