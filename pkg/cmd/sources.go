package cmd

import (
	"context"
	"log/slog"

	"github.com/iorubs/mcpsmithy/internal/project"
	"github.com/iorubs/mcpsmithy/pkg/api"
)

// SourcesCmd is the command group for source management.
type SourcesCmd struct {
	Pull SourcesPullCmd `cmd:"" help:"Fetch external sources and write them to disk."`
}

// SourcesPullCmd fetches all sources and writes them to disk.
type SourcesPullCmd struct {
	ConfigFlag
}

// Run executes sources pull.
func (cmd *SourcesPullCmd) Run(ctx context.Context) error {
	cfg, root, err := api.LoadConfig(cmd.Config)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "pull starting", "config", cmd.Config, "root", root)

	project.Build(ctx, cfg, root, project.BuildOptions{SkipIndex: true})

	slog.InfoContext(ctx, "pull complete")
	return nil
}
