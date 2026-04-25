// Package cmd implements the CLI subcommands.
package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LogLevel represents a supported log verbosity level.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

var LogEnum = strings.Join([]string{
	string(LogLevelDebug),
	string(LogLevelInfo),
	string(LogLevelWarn),
	string(LogLevelError),
}, ",")

// CLI is the root Kong CLI struct.
type CLI struct {
	LogLevel LogLevel `help:"Log level (one of: ${log_enum})." default:"${log_default}" enum:"${log_enum}" short:"l"`
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

// ProjectRoot resolves the project root from the config file location.
// The root is always the directory containing the config file.
func ProjectRoot(path string) (string, error) {
	if path != "" {
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("config path %q is a directory, not a file", path)
			}
			return filepath.Abs(filepath.Dir(path))
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
