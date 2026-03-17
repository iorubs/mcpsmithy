// Command mcpsmithy is a config-driven MCP tool server.
// It reads .mcpsmithy.yaml and serves MCP tools over stdio.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/operator-assistant/mcpsmithy/internal/commands"
)

func main() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "--help")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var cli commands.CLI
	kctx := kong.Parse(&cli,
		kong.Name("mcpsmithy"),
		kong.Description("Project-agnostic MCP tool server. Reads .mcpsmithy.yaml and serves MCP tools over stdio."),
		kong.UsageOnError(),
		kong.BindTo(ctx, (*context.Context)(nil)),
	)

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: commands.ParseLogLevel(cli.LogLevel),
	})))

	kctx.FatalIfErrorf(kctx.Run(&cli))
}
