package cmd

import (
	"fmt"

	"github.com/s-oravec/cage/internal/cage"
	"github.com/s-oravec/cage/internal/config"
	"github.com/spf13/cobra"
)

// NewRestartCmd creates the restart command
func NewRestartCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart a cage VM",
		Long: `Restart a cage VM by stopping and starting it.

The cage is restarted with the same configuration.
Use --force for immediate shutdown instead of graceful shutdown.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeCageNames(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			return restartCage(cmd, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force immediate shutdown")

	return cmd
}

func restartCage(cmd *cobra.Command, name string, force bool) error {
	// Check cage exists
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	// Load current state
	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	if state.Status != cage.StatusRunning {
		return fmt.Errorf("cage '%s' is not running", name)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Restarting cage '%s'...\n", name)

	// Stop the cage
	if err := stopCage(cmd, name, force); err != nil {
		return fmt.Errorf("failed to stop cage: %w", err)
	}

	// Load config for start
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Start the cage again
	fmt.Fprintln(cmd.OutOrStdout())
	if err := startCage(cmd, name, nil, cfg); err != nil {
		return fmt.Errorf("failed to start cage: %w", err)
	}

	return nil
}
