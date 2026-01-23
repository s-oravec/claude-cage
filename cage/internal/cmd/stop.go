package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/cage"
	"github.com/stiivo/cage/internal/libvirt"
)

// NewStopCmd creates the stop command
func NewStopCmd() *cobra.Command {
	var force bool
	var all bool

	cmd := &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a cage VM",
		Long: `Stop a cage VM and clean up its resources.

By default, performs a graceful shutdown. Use --force for immediate termination.
The cage's overlay disk is deleted - changes are lost.`,
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

	fmt.Fprintf(cmd.OutOrStdout(), "Stopping cage '%s'...\n", name)

	client := libvirt.NewClient()

	// Stop the domain
	if force {
		fmt.Fprintln(cmd.OutOrStdout(), "  Force stopping VM...")
		if err := client.DestroyDomain(name); err != nil {
			// Ignore error if domain not running
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  Shutting down VM...")
		if err := client.StopDomain(name); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
		}
	}

	// Undefine the domain
	fmt.Fprintln(cmd.OutOrStdout(), "  Removing VM definition...")
	if err := client.UndefineDomain(name); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
	}

	// Delete cage state and files
	fmt.Fprintln(cmd.OutOrStdout(), "  Cleaning up...")
	if err := cage.DeleteState(name); err != nil {
		return fmt.Errorf("failed to cleanup: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Cage '%s' stopped\n", name)
	return nil
}

func stopAllCages(cmd *cobra.Command, force bool) error {
	cages, err := cage.List()
	if err != nil {
		return err
	}

	if len(cages) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No cages running")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Stopping %d cage(s)...\n", len(cages))

	var errors []error
	for _, c := range cages {
		if err := stopCage(cmd, c.Name, force); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some cages failed to stop: %v", errors)
	}

	return nil
}
