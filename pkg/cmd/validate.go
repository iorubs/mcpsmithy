package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iorubs/mcpsmithy/pkg/api"
)

// ValidateCmd validates the config file.
type ValidateCmd struct {
	ConfigFlag
}

// Run executes validate.
func (cmd *ValidateCmd) Run(ctx context.Context) error {
	_, _, err := api.LoadConfig(cmd.Config)
	if err != nil {
		return fmt.Errorf("config validation: %w", err)
	}
	slog.InfoContext(ctx, "config is valid")
	return nil
}
