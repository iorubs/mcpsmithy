package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/iorubs/mcpsmithy/internal/server"
	"github.com/iorubs/mcpsmithy/internal/tools"
	"github.com/iorubs/mcpsmithy/pkg/api"
)

// ServeCmd starts the MCP server.
type ServeCmd struct {
	Transport string `help:"Transport to use." default:"stdio" enum:"stdio,http"`
	Addr      string `help:"Listen address (HTTP transport only)." default:":8080"`
	Watch     bool   `help:"Watch config file and hot-reload on change." default:"false"`
}

// Run executes the serve command.
func (cmd *ServeCmd) Run(ctx context.Context, cli *CLI) error {
	cfg, root, err := api.LoadConfig(cli.Config)
	if err != nil {
		return err
	}

	opts := api.ServeOptions{Root: root, Transport: cmd.Transport, Addr: cmd.Addr}
	if cmd.Watch {
		eng, err := tools.New(ctx, cfg, root)
		if err != nil {
			return fmt.Errorf("engine: %w", err)
		}

		transport := cmd.Transport
		if transport == "" {
			transport = "stdio"
		}

		var srv *server.Server
		if transport == "http" {
			srv = server.New(eng, server.WithHTTP(cmd.Addr))
		} else {
			srv = server.New(eng)
		}

		go watchConfig(ctx, cli.Config, srv)
		slog.InfoContext(ctx, "ready", "project", cfg.Project.Name, "root", root, "tools", len(cfg.Tools))

		return srv.Serve(ctx)
	}

	return api.Serve(ctx, cfg, opts)
}

// watchConfig polls the config file for mtime changes and hot-reloads on change.
func watchConfig(ctx context.Context, path string, srv *server.Server) {
	const pollInterval = 2 * time.Second
	const debounceDelay = 500 * time.Millisecond

	info, err := os.Stat(path)
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
			fi, err := os.Stat(path)
			if err != nil || !fi.ModTime().After(lastMod) {
				continue
			}
			lastMod = fi.ModTime()
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(debounceDelay, func() {
				reload(ctx, path, srv)
			})
		}
	}
}

func reload(ctx context.Context, path string, srv *server.Server) {
	cfg, root, err := api.LoadConfig(path)
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
