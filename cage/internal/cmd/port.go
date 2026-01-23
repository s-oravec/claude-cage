package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/cage"
	"github.com/stiivo/cage/internal/config"
	"github.com/stiivo/cage/internal/network"
)

// NewPortCmd creates the port command with subcommands
func NewPortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port",
		Short: "Manage port forwarding for cages",
		Long: `Manage port forwarding rules for cages.

Port forwarding allows you to access services running inside a cage
from your host machine. By default, ports are bound to 127.0.0.1 (localhost only).`,
	}

	cmd.AddCommand(newPortListCmd())
	cmd.AddCommand(newPortAddCmd())
	cmd.AddCommand(newPortRemoveCmd())

	return cmd
}

func newPortListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <name>",
		Short: "List port forwards for a cage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return listPorts(cmd, args[0])
		},
	}
}

func newPortAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <port-spec>",
		Short: "Add a port forward to a running cage",
		Long: `Add a port forward to a running cage.

Port spec format:
  hostPort:guestPort        - Forward host port to guest port
  bind:hostPort:guestPort   - Bind to specific address
  hostPort:guestPort/proto  - Specify protocol (tcp or udp)

Examples:
  cage port add mycage 8080:80
  cage port add mycage 0.0.0.0:8080:80
  cage port add mycage 5353:53/udp`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return addPort(cmd, args[0], args[1])
		},
	}
}

func newPortRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name> <host-port>",
		Short: "Remove a port forward from a cage",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return removePort(cmd, args[0], args[1])
		},
	}
}

func listPorts(cmd *cobra.Command, name string) error {
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	if len(state.Ports) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No port forwards configured for cage '%s'\n", name)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Port forwards for cage '%s':\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %s\n", "HOST", "GUEST", "PROTOCOL")
	for _, p := range state.Ports {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%-14d %-10d %s\n",
			p.Bind, p.Host, p.Guest, p.Protocol)
	}

	return nil
}

func addPort(cmd *cobra.Command, name, spec string) error {
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	if state.Status != cage.StatusRunning {
		return fmt.Errorf("cage '%s' is not running", name)
	}

	if state.IP == "" {
		return fmt.Errorf("cage '%s' has no IP address", name)
	}

	// Get default bind from config
	cfg, _ := config.Load()
	defaultBind := "127.0.0.1"
	if cfg != nil && cfg.Network.PortBind != "" {
		defaultBind = cfg.Network.PortBind
	}

	// Parse port spec
	fwd, err := network.ParsePortSpec(spec, defaultBind)
	if err != nil {
		return err
	}

	// Check for duplicate
	for _, existing := range state.Ports {
		if existing.Host == fwd.HostPort {
			return fmt.Errorf("host port %d is already forwarded", fwd.HostPort)
		}
	}

	// Check for port conflict on host
	if network.PortConflict(fwd.HostPort, fwd.Bind) {
		return fmt.Errorf("host port %d is already in use", fwd.HostPort)
	}

	// Start port forwarding
	forwarder, err := network.AddSingleForward(name, state.IP, *fwd)
	if err != nil {
		return fmt.Errorf("failed to start port forwarding: %w", err)
	}

	// Update state with new port and forwarder PID
	state.Ports = append(state.Ports, cage.Port{
		Host:         fwd.HostPort,
		Guest:        fwd.GuestPort,
		Protocol:     fwd.Protocol,
		Bind:         fwd.Bind,
		ForwarderPID: forwarder.PID,
	})

	if err := cage.SaveState(state); err != nil {
		// Try to stop the forwarder we just started
		forwarder.Stop()
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added port forward: %s:%d -> %d/%s\n",
		fwd.Bind, fwd.HostPort, fwd.GuestPort, fwd.Protocol)
	return nil
}

func removePort(cmd *cobra.Command, name, hostPortStr string) error {
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	// Parse host port
	var hostPort int
	if _, err := fmt.Sscanf(hostPortStr, "%d", &hostPort); err != nil {
		return fmt.Errorf("invalid port number: %s", hostPortStr)
	}

	// Find and remove the port
	found := false
	var newPorts []cage.Port
	for _, p := range state.Ports {
		if p.Host == hostPort {
			found = true
			// Stop the forwarder if it has a PID
			if p.ForwarderPID > 0 {
				network.StopForwarderByPID(p.ForwarderPID)
			}
			continue
		}
		newPorts = append(newPorts, p)
	}

	if !found {
		return fmt.Errorf("port %d not found in cage '%s'", hostPort, name)
	}

	state.Ports = newPorts
	if err := cage.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed port forward: %d\n", hostPort)
	return nil
}
