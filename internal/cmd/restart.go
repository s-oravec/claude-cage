package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
)

// NewRestartCmd creates the restart command
func NewRestartCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart a cage VM",
		Long: `Restart a cage VM by stopping and starting it.

The cage is restarted with the same configuration (image, profile, ports).
Use --force for immediate shutdown instead of graceful shutdown.`,
		Args: cobra.ExactArgs(1),
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

	// Load current state to preserve config
	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	if state.Status != cage.StatusRunning {
		return fmt.Errorf("cage '%s' is not running", name)
	}

	// Save restart config before stopping
	restartConfig := &cage.RestartConfig{
		Image:   state.Image,
		Profile: state.Profile,
		Ports:   state.Ports,
	}

	// Stop the cage
	fmt.Fprintf(cmd.OutOrStdout(), "Stopping cage '%s'...\n", name)

	// Create a new stop command and execute it
	stopCmd := NewStopCmd()
	stopCmd.SetOut(cmd.OutOrStdout())
	stopCmd.SetErr(cmd.ErrOrStderr())
	if force {
		stopCmd.SetArgs([]string{name, "--force"})
	} else {
		stopCmd.SetArgs([]string{name})
	}

	if err := stopCmd.Execute(); err != nil {
		return fmt.Errorf("failed to stop cage: %w", err)
	}

	// Save restart config for the start command
	if err := cage.SaveRestartConfig(name, restartConfig); err != nil {
		return fmt.Errorf("failed to save restart config: %w", err)
	}

	// Start the cage with the same config
	fmt.Fprintf(cmd.OutOrStdout(), "\nStarting cage '%s'...\n", name)

	startCmd := NewStartCmd()
	startCmd.SetOut(cmd.OutOrStdout())
	startCmd.SetErr(cmd.ErrOrStderr())

	// Build start args
	startArgs := []string{"--name", name, "--profile", restartConfig.Profile, "--image", restartConfig.Image}
	for _, p := range restartConfig.Ports {
		portSpec := fmt.Sprintf("%d:%d", p.Host, p.Guest)
		if p.Bind != "" && p.Bind != "127.0.0.1" {
			portSpec = fmt.Sprintf("%s:%s", p.Bind, portSpec)
		}
		if p.Protocol != "" && p.Protocol != "tcp" {
			portSpec = fmt.Sprintf("%s/%s", portSpec, p.Protocol)
		}
		startArgs = append(startArgs, "--port", portSpec)
	}

	startCmd.SetArgs(startArgs)

	if err := startCmd.Execute(); err != nil {
		// Clean up restart config on failure
		cage.DeleteRestartConfig(name)
		return fmt.Errorf("failed to start cage: %w", err)
	}

	// Clean up restart config
	cage.DeleteRestartConfig(name)

	fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Cage '%s' restarted\n", name)
	return nil
}
