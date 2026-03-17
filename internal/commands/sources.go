package commands

import (
	"context"
	"log/slog"

	"github.com/operator-assistant/mcpsmithy/internal/project"
)

// SourcesCmd is the command group for source management.
type SourcesCmd struct {
	Pull SourcesPullCmd `cmd:"" help:"Fetch external sources and write them to disk."`
}

// SourcesPullCmd fetches all sources and writes them to disk.
type SourcesPullCmd struct{}

// Run executes sources pull.
func (cmd *SourcesPullCmd) Run(ctx context.Context, cli *CLI) error {
	cfg, root, err := cli.LoadConfig()
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "pull starting", "config", cli.Config, "root", root)

	project.Build(ctx, cfg, root, project.BuildOptions{SkipIndex: true})

	slog.InfoContext(ctx, "pull complete")
	return nil
}
