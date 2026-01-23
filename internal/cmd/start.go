package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/ssh"
	"github.com/s-oravec/claude-cage/internal/virtiofs"
)

// NewStartCmd creates the start command
func NewStartCmd() *cobra.Command {
	var ports []string

	cmd := &cobra.Command{
		Use:   "start [name]",
		Short: "Start an existing cage VM",
		Long: `Start a cage VM that was previously created.

Use 'cage create' to create a new cage first.
Use 'cage start' to start or restart a stopped cage.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return startCage(cmd, args[0], ports)
		},
	}

	cmd.Flags().StringArrayVar(&ports, "port", nil, "Port forwarding (e.g., 8080:80)")

	return cmd
}

func startCage(cmd *cobra.Command, name string, ports []string) error {
	// Check cage exists
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found, use 'cage create' first", name)
	}

	// Load state
	state, err := cage.LoadState(name)
	if err != nil {
		return err
	}

	if state.Status == cage.StatusRunning {
		return fmt.Errorf("cage '%s' is already running", name)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Starting cage '%s'...\n", name)

	client := libvirt.NewClient()

	// Start virtiofsd if shares are configured and using bridge network
	var virtiofsDaemon *virtiofs.Daemon
	var virtiofsSocket string

	if len(cfg.Shares) > 0 && cfg.Security.VirtiofsSandbox && state.NetworkMode == cage.NetworkBridge {
		// virtiofsd requires root (uses setgroups() on startup)
		if os.Getuid() != 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  File sharing requires root (virtiofsd limitation)")
		} else {
			share := cfg.Shares[0]
			sharedDir := virtiofs.ExpandPath(share.Host)

			fmt.Fprintf(cmd.OutOrStdout(), "  Starting virtiofsd (%s)...\n", sharedDir)

			virtiofsDaemon, err = virtiofs.Start(&virtiofs.DaemonConfig{
				CageName:  name,
				SharedDir: sharedDir,
				Sandbox:   true,
				Seccomp:   true,
			})
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: virtiofsd failed: %v\n", err)
				fmt.Fprintln(cmd.OutOrStdout(), "  Continuing without file sharing...")
			} else {
				virtiofsSocket = virtiofsDaemon.SocketPath
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	// Start the domain
	fmt.Fprintln(cmd.OutOrStdout(), "  Starting VM...")
	if err := client.StartDomain(name); err != nil {
		if virtiofsDaemon != nil {
			virtiofsDaemon.Stop()
		}
		return fmt.Errorf("failed to start VM: %w", err)
	}

	// Wait for VM to get an IP (only for bridge networking)
	var ip string
	if state.NetworkMode == cage.NetworkBridge {
		fmt.Fprint(cmd.OutOrStdout(), "  Waiting for IP address...")
		for i := 0; i < 30; i++ {
			time.Sleep(2 * time.Second)
			ip, _ = client.GetDomainIP(name)
			if ip != "" {
				break
			}
			fmt.Fprint(cmd.OutOrStdout(), ".")
		}
		fmt.Fprintln(cmd.OutOrStdout())

		if ip == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "  Warning: Could not get IP address")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  IP: %s\n", ip)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s networking: use 'cage console' to access\n", state.NetworkMode)
	}

	// Update state
	state.Status = cage.StatusRunning
	state.IP = ip
	state.StartedAt = time.Now()

	if virtiofsDaemon != nil {
		state.VirtiofsPID = virtiofsDaemon.PID
	}

	// Parse and setup port forwarding
	var forwardedPorts []cage.Port
	if len(ports) > 0 && ip != "" {
		defaultBind := cfg.Network.PortBind
		if defaultBind == "" {
			defaultBind = "127.0.0.1"
		}

		parsedPorts, err := network.ParsePortSpecs(ports, defaultBind)
		if err != nil {
			return fmt.Errorf("invalid port specification: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "  Setting up port forwarding...")
		forwarder, err := network.StartForwarding(name, ip, parsedPorts)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: port forwarding failed: %v\n", err)
		} else {
			for _, p := range parsedPorts {
				forwardedPorts = append(forwardedPorts, cage.Port{
					Host:         p.HostPort,
					Guest:        p.GuestPort,
					Protocol:     p.Protocol,
					Bind:         p.Bind,
					ForwarderPID: forwarder.PID,
				})
			}
		}
	}
	state.Ports = forwardedPorts

	if err := cage.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Wait for SSH if we have an IP
	if ip != "" {
		fmt.Fprint(cmd.OutOrStdout(), "  Waiting for SSH...")
		if err := ssh.WaitForSSH(name, ip, 60*time.Second); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), " timeout (VM may still be booting)")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), " ready")
		}
	}

	// Get profile for display
	profile, _ := cfg.GetProfile(state.Profile)

	fmt.Fprintf(cmd.OutOrStdout(), "Cage '%s' started\n", name)
	if profile != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s, Profile: %s (%d vCPU, %d MB RAM)\n",
			state.Image, state.Profile, profile.VCPU, profile.MemoryMB)
	}

	if ip != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage ssh %s' to connect\n", name)
	} else if state.NetworkMode != cage.NetworkBridge {
		fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage console %s' to connect\n", name)
	}

	if virtiofsSocket != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "  File sharing: /mnt/host")
	}

	return nil
}
