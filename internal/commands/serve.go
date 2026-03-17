package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/operator-assistant/mcpsmithy/internal/server"
	"github.com/operator-assistant/mcpsmithy/internal/tools"
)

// ServeCmd starts the MCP server.
type ServeCmd struct {
	Transport string `help:"Transport to use." default:"stdio" enum:"stdio,http"`
	Addr      string `help:"Listen address (HTTP transport only)." default:":8080"`
	Watch     bool   `help:"Watch config file and hot-reload on change." default:"false"`
}

// Run executes the serve command.
func (cmd *ServeCmd) Run(ctx context.Context, cli *CLI) error {
	cfg, root, err := cli.LoadConfig()
	if err != nil {
		return err
	}

	eng, err := tools.New(ctx, cfg, root)
	if err != nil {
		return fmt.Errorf("engine: %w", err)
	}

	var srv *server.Server
	switch cmd.Transport {
	case "http":
		srv = server.NewHTTP(eng, cmd.Addr)
	default:
		srv = server.New(eng, os.Stdin, os.Stdout)
	}

	if cmd.Watch {
		go watchConfig(ctx, cli, srv)
	}

	slog.InfoContext(ctx, "ready", "project", cfg.Project.Name, "root", root, "tools", len(cfg.Tools))
	return srv.Serve(ctx)
}

// watchConfig polls the config file for mtime changes and hot-reloads on change.
func watchConfig(ctx context.Context, cli *CLI, srv *server.Server) {
	const pollInterval = 2 * time.Second
	const debounceDelay = 500 * time.Millisecond

	info, err := os.Stat(cli.Config)
	if err != nil {
		slog.ErrorContext(ctx, "watch: cannot stat config", "err", err)
		return
	}
	lastMod := info.ModTime()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var debounce *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi, err := os.Stat(cli.Config)
			if err != nil || !fi.ModTime().After(lastMod) {
				continue
			}
			lastMod = fi.ModTime()
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(debounceDelay, func() {
				reload(ctx, cli, srv)
			})
		}
	}
}

func reload(ctx context.Context, cli *CLI, srv *server.Server) {
	cfg, root, err := cli.LoadConfig()
	if err != nil {
		slog.ErrorContext(ctx, "reload: config error, keeping previous engine", "err", err)
		return
	}
	eng, err := tools.New(ctx, cfg, root)
	if err != nil {
		slog.ErrorContext(ctx, "reload: engine build failed, keeping previous engine", "err", err)
		return
	}
	srv.SwapEngine(eng)
	slog.InfoContext(ctx, "reload: engine swapped", "tools", len(cfg.Tools))
}
