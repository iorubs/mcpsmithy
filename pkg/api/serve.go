package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/iorubs/mcpsmithy/internal/config"
	"github.com/iorubs/mcpsmithy/internal/server"
	"github.com/iorubs/mcpsmithy/internal/tools"
)

// ServeOptions controls server behaviour.
type ServeOptions struct {
	// Root is the directory the server is rooted in.
	Root string
	// Transport enum(stdio|http) controlls the server transport mode, defaults to stdio.
	Transport string
	// Addr is the listening address for the HTTP transport.
	Addr string
}

// Serve builds the tool engine from cfg and opts.Root and runs the MCP server
// on the requested transport. It blocks until ctx is cancelled.
func Serve(ctx context.Context, cfg *config.Config, opts ServeOptions) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	eng, err := tools.New(ctx, cfg, opts.Root)
	if err != nil {
		return fmt.Errorf("engine: %w", err)
	}

	transport := opts.Transport
	if transport == "" {
		transport = "stdio"
	}

	var srv *server.Server
	if transport == "http" {
		srv = server.New(eng, server.WithHTTP(opts.Addr))
	} else {
		srv = server.New(eng)
	}
	slog.InfoContext(ctx, "ready", "project", cfg.Project.Name, "root", opts.Root, "tools", len(cfg.Tools))

	return srv.Serve(ctx)
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
	cfg, root, err := LoadConfig(path)
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
