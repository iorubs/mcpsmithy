package cmd

import (
	"context"
	"log/slog"

	"github.com/iorubs/mcpsmithy/internal/server"
	"github.com/iorubs/mcpsmithy/internal/setup"
)

// SetupCmd starts an MCP server for config-authoring sessions.
// It does not require an existing .mcpsmithy.yaml.
type SetupCmd struct{}

// Run starts the setup server on stdio.
func (s *SetupCmd) Run(ctx context.Context, cli *CLI) error {
	slog.InfoContext(ctx, "setup server running on stdio — connect your agent to write .mcpsmithy.yaml")
	slog.InfoContext(ctx, "when done: mcpsmithy validate; then: mcpsmithy serve")
	eng := setup.New()
	srv := server.New(eng)
	return srv.Serve(ctx)
}
