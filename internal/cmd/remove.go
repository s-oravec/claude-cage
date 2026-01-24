package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/ssh"
	"github.com/s-oravec/claude-cage/internal/virtiofs"
)

// NewRemoveCmd creates the remove command
func NewRemoveCmd() *cobra.Command {
	var force bool
	var all bool

	cmd := &cobra.Command{
		Use:   "remove [name]",
		Short: "Remove a cage and all its resources",
		Long: `Remove a cage VM and all its associated resources.

This stops the VM (if running) and removes:
- The VM definition
- Disk overlay
- SSH keys and known_hosts entries
- Network configuration
- Firewall rules

When run from a directory with .claude-cage.yml, the cage name is optional.

The cage's data is permanently deleted.`,
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return removeAllCages(cmd, force)
			}

			name, _, err := resolveCageName(args)
			if err != nil {
				return err
			}

			return removeCage(cmd, name, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force removal even if running")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Remove all cages")

	return cmd
}

func removeCage(cmd *cobra.Command, name string, force bool) error {
	// Check cage exists
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	// Load state to check status
	state, err := cage.LoadState(name)
	if err != nil {
		return err
	}

	// Check if running
	if state.Status == cage.StatusRunning && !force {
		return fmt.Errorf("cage '%s' is running, use --force or stop it first", name)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removing cage '%s'...\n", name)

	client := libvirt.NewClient()

	// Stop VM if running
	if state.Status == cage.StatusRunning {
		fmt.Fprintln(cmd.OutOrStdout(), "  Stopping VM...")
		if force {
			if err := client.DestroyDomain(name); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
			}
		} else {
			if err := client.StopDomain(name); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
			}
		}
	}

	// Undefine the domain
	fmt.Fprintln(cmd.OutOrStdout(), "  Removing VM definition...")
	if err := client.UndefineDomain(name); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
	}

	// Stop virtiofsd if running
	if state.VirtiofsPID > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  Stopping virtiofsd...")
		virtiofs.StopByPID(name, state.VirtiofsPID)
	} else {
		// Cleanup socket dir anyway
		virtiofs.CleanupSocket(name)
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

	// Delete SSH keys and known_hosts entry
	fmt.Fprintln(cmd.OutOrStdout(), "  Removing SSH keys...")
	ssh.DeleteKeys(name)

	// Remove known_hosts entry to avoid "host key changed" errors on recreate
	if state.SSHPort > 0 {
		ssh.RemoveKnownHost(fmt.Sprintf("[127.0.0.1]:%d", state.SSHPort))
	}
	if state.IP != "" {
		ssh.RemoveKnownHost(state.IP)
	}

	// Cleanup firewall and network (only for bridge mode)
	if state.NetworkMode == cage.NetworkBridge {
		fmt.Fprintln(cmd.OutOrStdout(), "  Cleaning up firewall...")
		cfg, _ := config.Load()
		dnsServer := "1.1.1.1"
		if cfg != nil && len(cfg.Network.DNS) > 0 {
			dnsServer = cfg.Network.DNS[0]
		}
		network.CleanupFirewall(name, dnsServer)

		fmt.Fprintln(cmd.OutOrStdout(), "  Destroying network...")
		network.DestroyNetwork(name)
	}

	// Delete cage state and files
	fmt.Fprintln(cmd.OutOrStdout(), "  Removing cage files...")
	if err := cage.DeleteState(name); err != nil {
		return fmt.Errorf("failed to cleanup: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Cage '%s' removed\n", name)
	return nil
}

func removeAllCages(cmd *cobra.Command, force bool) error {
	cages, err := cage.List()
	if err != nil {
		return err
	}

	if len(cages) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No cages to remove")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removing %d cage(s)...\n", len(cages))

	var errors []error
	for _, c := range cages {
		if err := removeCage(cmd, c.Name, force); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some cages failed to remove: %v", errors)
	}

	return nil
}
