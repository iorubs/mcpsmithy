package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iorubs/mcpsmithy/pkg/api"
)

// ValidateCmd validates the config file.
type ValidateCmd struct{}

// Run executes validate.
func (cmd *ValidateCmd) Run(ctx context.Context, cli *CLI) error {
	_, _, err := api.LoadConfig(cli.Config)
	if err != nil {
		return fmt.Errorf("config validation: %w", err)
	}
	slog.InfoContext(ctx, "config is valid")
	return nil
}
