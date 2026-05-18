package cmd

import (
	"fmt"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/mode"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/ssh"
	"github.com/s-oravec/claude-cage/internal/virtiofs"
	"github.com/spf13/cobra"
)

// NewRemoveCmd creates the remove command
func NewRemoveCmd() *cobra.Command {
	var force bool
	var all bool

	cmd := &cobra.Command{
		Use:     "remove [name]",
		Aliases: []string{"rm"},
		Short:   "Remove a cage and all its resources",
		Long: `Remove a cage VM and all its associated resources.

This stops the VM (if running) and removes:
- The VM definition
- Disk overlay
- SSH keys and known_hosts entries
- Network configuration
- Firewall rules

When run from a directory with .cage.yml, the cage name is optional.

The cage's data is permanently deleted.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeCageNames(false),
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
	state, stateErr := cage.LoadState(name)
	haveState := stateErr == nil

	if !haveState {
		if !force {
			return fmt.Errorf("cage '%s' not found (use --force to clean up orphaned files/domains)", name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removing cage '%s' (no state — orphan cleanup)...\n", name)
	} else {
		if err := cage.RequireMode(name, mode.Current().String()); err != nil {
			if !force {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
		}
		if state.Status == cage.StatusRunning && !force {
			return fmt.Errorf("cage '%s' is running, use --force or stop it first", name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removing cage '%s'...\n", name)
	}

	// Stop and undefine the libvirt domain. With --force we try BOTH session
	// and system URIs since the cage might be defined in the other mode (e.g.
	// orphan from a prior run, or wrong-mode invocation).
	uris := []string{mode.Current().URI()}
	if force {
		other := mode.User.URI()
		if mode.Current() == mode.User {
			other = mode.Root.URI()
		}
		uris = append(uris, other)
	}
	for _, uri := range uris {
		c := libvirt.NewClientWithURI(uri)
		if haveState && state.Status == cage.StatusRunning {
			fmt.Fprintf(cmd.OutOrStdout(), "  Stopping VM in %s...\n", uri)
			if force {
				if err := c.DestroyDomain(name); err != nil && !force {
					fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
				}
			} else {
				if err := c.StopDomain(name); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
				}
			}
		}
		if err := c.UndefineDomain(name); err != nil {
			// Silent under --force: domain probably doesn't exist in this URI.
			if !force {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Undefined domain in %s\n", uri)
		}
	}

	// State-dependent cleanup (PIDs, ports, network mode).
	if haveState {
		if state.VirtiofsPID > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  Stopping virtiofsd...")
			virtiofs.StopByPID(name, state.VirtiofsPID)
		} else {
			virtiofs.CleanupSocket(name)
		}

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

		if state.SSHPort > 0 {
			ssh.RemoveKnownHost(fmt.Sprintf("[127.0.0.1]:%d", state.SSHPort))
		}
		if state.IP != "" {
			ssh.RemoveKnownHost(state.IP)
		}

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
	} else {
		// Best-effort socket dir cleanup without state.
		virtiofs.CleanupSocket(name)
	}

	// Always remove SSH keys + cage files (these are local rm -rf, idempotent).
	fmt.Fprintln(cmd.OutOrStdout(), "  Removing SSH keys...")
	ssh.DeleteKeys(name)

	fmt.Fprintln(cmd.OutOrStdout(), "  Removing cage files...")
	if err := cage.DeleteState(name); err != nil {
		if force {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
		} else {
			return fmt.Errorf("failed to cleanup: %w", err)
		}
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
