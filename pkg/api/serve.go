package api

import (
	"context"
	"fmt"
	"log/slog"

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
