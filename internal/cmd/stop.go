package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/virtiofs"
)

// NewStopCmd creates the stop command
func NewStopCmd() *cobra.Command {
	var force bool
	var all bool

	cmd := &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a cage VM",
		Long: `Stop a running cage VM.

By default, performs a graceful shutdown. Use --force for immediate termination.
The cage's resources (disk, network, keys) are preserved and can be restarted.

To remove a cage and all its resources, use 'cage remove'.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return stopAllCages(cmd, force)
			}

			if len(args) == 0 {
				return fmt.Errorf("cage name required (or use --all)")
			}

			return stopCage(cmd, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force immediate shutdown")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Stop all running cages")

	return cmd
}

func stopCage(cmd *cobra.Command, name string, force bool) error {
	// Check cage exists
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	// Load state
	state, err := cage.LoadState(name)
	if err != nil {
		return err
	}

	if state.Status != cage.StatusRunning {
		return fmt.Errorf("cage '%s' is not running", name)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Stopping cage '%s'...\n", name)

	client := libvirt.NewClient()

	// Stop the domain
	if force {
		fmt.Fprintln(cmd.OutOrStdout(), "  Force stopping VM...")
		if err := client.DestroyDomain(name); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  Shutting down VM...")
		if err := client.StopDomain(name); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
		}
	}

	// Stop virtiofsd if running
	if state.VirtiofsPID > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Stopping virtiofsd...")
		virtiofs.StopByPID(name, state.VirtiofsPID)
	}

	// Stop port forwarders
	if len(state.Ports) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Stopping port forwarders...")
		seenPIDs := make(map[int]bool)
		for _, p := range state.Ports {
			if p.ForwarderPID > 0 && !seenPIDs[p.ForwarderPID] {
				seenPIDs[p.ForwarderPID] = true
				network.StopForwarderByPID(p.ForwarderPID)
			}
		}
	}

	// Update state to stopped
	state.Status = cage.StatusStopped
	state.VirtiofsPID = 0
	state.Ports = nil // Clear port forwarders
	if err := cage.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Cage '%s' stopped\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage start %s' to restart\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage remove %s' to delete\n", name)
	return nil
}

func stopAllCages(cmd *cobra.Command, force bool) error {
	cages, err := cage.List()
	if err != nil {
		return err
	}

	// Filter to only running cages
	var running []*cage.State
	for _, c := range cages {
		if c.Status == cage.StatusRunning {
			running = append(running, c)
		}
	}

	if len(running) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No running cages")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Stopping %d cage(s)...\n", len(running))

	var errors []error
	for _, c := range running {
		if err := stopCage(cmd, c.Name, force); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some cages failed to stop: %v", errors)
	}

	return nil
}
