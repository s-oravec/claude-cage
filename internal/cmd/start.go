package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/cloudinit"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/runtime"
	"github.com/s-oravec/claude-cage/internal/ssh"
	"github.com/s-oravec/claude-cage/internal/virtiofs"
)

// NewStartCmd creates the start command
func NewStartCmd() *cobra.Command {
	var ports []string

	cmd := &cobra.Command{
		Use:   "start [name]",
		Short: "Start a cage VM (creates if needed)",
		Long: `Start a cage VM. If the cage doesn't exist and a .claude-cage.yml
config file is present, the cage will be created automatically.

Use 'cage init' to create a project configuration.
Use 'cage start' in a project directory to start the configured cage.
Use 'cage start <name>' to start a specific existing cage.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStartCmd(cmd, args, ports)
		},
	}

	cmd.Flags().StringArrayVar(&ports, "port", nil, "Port forwarding (e.g., 8080:80)")

	return cmd
}

func runStartCmd(cmd *cobra.Command, args []string, ports []string) error {
	// Resolve cage name (from args or project config)
	name, projectCfg, err := resolveCageName(args)
	if err != nil {
		return err
	}

	// Load global config
	globalCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve project config if available
	var resolved *config.ResolvedConfig
	if projectCfg != nil {
		cwd, _ := os.Getwd()
		resolved, err = config.ResolveProjectConfig(globalCfg, projectCfg, cwd)
		if err != nil {
			return fmt.Errorf("failed to resolve config: %w", err)
		}
	}

	// Check if cage exists
	if !cage.Exists(name) {
		// Cage doesn't exist - need project config to create
		if resolved == nil {
			return fmt.Errorf("cage '%s' not found, use 'cage create' first or run from a directory with %s", name, config.ProjectConfigFile)
		}

		// Create the cage
		if err := createCageFromConfig(cmd, name, resolved, globalCfg); err != nil {
			return err
		}
	} else {
		// Cage exists - validate image if project config available
		if resolved != nil {
			state, err := cage.LoadState(name)
			if err != nil {
				return fmt.Errorf("failed to load cage state: %w", err)
			}

			// Resolve image alias for comparison
			resolvedImage := images.ResolveAlias(resolved.Image)
			if state.Image != resolvedImage {
				return fmt.Errorf("cage image mismatch: cage uses '%s', config specifies '%s'. Use 'cage rm %s' and restart to recreate", state.Image, resolvedImage, name)
			}
		}
	}

	// Write runtime env file if project config has env vars
	if resolved != nil && len(resolved.Env) > 0 {
		cageDir := cage.Dir(name)
		envPath := runtime.EnvFilePath(cageDir)
		if err := runtime.WriteEnvFile(envPath, resolved.Env); err != nil {
			return fmt.Errorf("failed to write runtime env: %w", err)
		}
	}

	// Start the cage
	return startCage(cmd, name, ports, globalCfg)
}

// createCageFromConfig creates a new cage from resolved project config
func createCageFromConfig(cmd *cobra.Command, name string, resolved *config.ResolvedConfig, globalCfg *config.Config) error {
	// Resolve image name
	imageName := images.ResolveAlias(resolved.Image)

	// Check image exists
	if !images.IsDownloaded(imageName) {
		return fmt.Errorf("image '%s' not found, run 'cage setup --base %s' first", imageName, imageName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Creating cage '%s'...\n", name)

	// Create cage directory
	cageDir := cage.Dir(name)
	if err := cage.EnsureDir(name); err != nil {
		return fmt.Errorf("failed to create cage directory: %w", err)
	}

	// Determine network mode and SSH port
	networkMode := cage.NetworkAuto // project config uses auto mode
	var sshPort int

	if resolved.SSHPort != "" {
		if resolved.SSHPort == "auto" {
			port, err := network.FindFreePort()
			if err != nil {
				return fmt.Errorf("failed to find free port: %w", err)
			}
			sshPort = port
		} else {
			if _, err := fmt.Sscanf(resolved.SSHPort, "%d", &sshPort); err != nil {
				return fmt.Errorf("invalid SSH port '%s'", resolved.SSHPort)
			}
		}
	}

	// Auto mode network setup
	if network.HasPasst() {
		fmt.Fprintln(cmd.OutOrStdout(), "  Using passt networking (auto-detected)...")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  Using SLIRP networking (passt not found)...")
	}

	// Create qcow2 overlay with specified disk size
	baseImage := images.ImagePath(imageName)
	overlayPath := filepath.Join(cageDir, "disk.qcow2")
	diskSize := fmt.Sprintf("%dG", resolved.DiskGB)

	fmt.Fprintf(cmd.OutOrStdout(), "  Creating disk overlay (%s)...\n", diskSize)
	createCmd := exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", baseImage, "-F", "qcow2", overlayPath, diskSize)
	if out, err := createCmd.CombinedOutput(); err != nil {
		cage.DeleteState(name)
		return fmt.Errorf("failed to create overlay: %s", string(out))
	}

	// Generate SSH keys
	fmt.Fprintln(cmd.OutOrStdout(), "  Generating SSH keys...")
	if err := ssh.GenerateKeyPair(name); err != nil {
		cage.DeleteState(name)
		return fmt.Errorf("failed to generate SSH keys: %w", err)
	}

	pubKey, err := ssh.GetPublicKey(name)
	if err != nil {
		cage.DeleteState(name)
		return fmt.Errorf("failed to read public key: %w", err)
	}

	// Create runtime directory and write initial env file
	if err := runtime.EnsureRuntimeDir(cageDir); err != nil {
		cage.DeleteState(name)
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	if len(resolved.Env) > 0 {
		envPath := runtime.EnvFilePath(cageDir)
		if err := runtime.WriteEnvFile(envPath, resolved.Env); err != nil {
			cage.DeleteState(name)
			return fmt.Errorf("failed to write runtime env: %w", err)
		}
	}

	// Create cloud-init ISO with UseRuntimeEnv=true
	fmt.Fprintln(cmd.OutOrStdout(), "  Creating cloud-init...")
	cloudInitPath, err := cloudinit.GenerateISOWithConfig(cageDir, &cloudinit.CloudInitConfig{
		CageName:      name,
		PubKey:        pubKey,
		MountVirtiofs: false, // Will be set at start time if virtiofsd is available
		UseRuntimeEnv: true,  // Use runtime env via virtiofs
		InstallSSH:    sshPort > 0,
	})
	if err != nil {
		cage.DeleteState(name)
		return fmt.Errorf("failed to create cloud-init: %w", err)
	}

	// Generate domain XML with RuntimeDir set
	runtimeDir := runtime.RuntimeDir(cageDir)
	domainCfg := &libvirt.DomainConfig{
		Name:           name,
		MemoryMB:       resolved.MemoryMB,
		VCPU:           resolved.VCPU,
		DiskPath:       overlayPath,
		CloudInitISO:   cloudInitPath,
		NetworkName:    "", // Empty for user-mode networking
		VirtiofsSocket: "", // Set at start time
		RuntimeDir:     runtimeDir,
		SSHPort:        sshPort,
	}

	xml, err := libvirt.GenerateDomainXML(domainCfg)
	if err != nil {
		return fmt.Errorf("failed to generate domain XML: %w", err)
	}

	// Define domain (but don't start it)
	fmt.Fprintln(cmd.OutOrStdout(), "  Defining VM...")
	client := libvirt.NewClient()
	if err := client.DefineDomain(xml); err != nil {
		ssh.DeleteKeys(name)
		cage.DeleteState(name)
		return fmt.Errorf("failed to define domain: %w", err)
	}

	// Save state as stopped with Image field
	state := &cage.State{
		Name:        name,
		Status:      cage.StatusStopped,
		Image:       imageName,
		Profile:     "custom", // Mark as custom since we use resolved config
		NetworkMode: networkMode,
		SSHPort:     sshPort,
	}

	if err := cage.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Cage '%s' created\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s (%d vCPU, %d MB RAM)\n",
		imageName, resolved.VCPU, resolved.MemoryMB)

	return nil
}

func startCage(cmd *cobra.Command, name string, ports []string, cfg *config.Config) error {
	// Load state
	state, err := cage.LoadState(name)
	if err != nil {
		return err
	}

	client := libvirt.NewClient()

	// Check if domain is already active in libvirt (state might be out of sync)
	isActive, _ := client.IsDomainActive(name)
	if isActive {
		// Domain is running, update state to match
		if state.Status != cage.StatusRunning {
			state.Status = cage.StatusRunning
			state.StartedAt = time.Now()
			cage.SaveState(state)
		}
		return fmt.Errorf("cage '%s' is already running", name)
	}

	if state.Status == cage.StatusRunning {
		// State says running but libvirt says not active - fix state
		state.Status = cage.StatusStopped
		cage.SaveState(state)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Starting cage '%s'...\n", name)

	// Start virtiofsd if shares are configured and using bridge network
	// (virtiofsd requires root, which is only available with bridge mode)
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
		// Auto mode (passt/slirp) - no IP from libvirt
		fmt.Fprintln(cmd.OutOrStdout(), "  User-mode networking: use 'cage console' to access")
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
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s\n", state.Image)
	}

	if ip != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage ssh %s' to connect\n", name)
	} else if state.NetworkMode != cage.NetworkBridge {
		if state.SSHPort > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  SSH: localhost:%d (once VM is ready)\n", state.SSHPort)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage console %s' to connect\n", name)
	}

	if virtiofsSocket != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "  File sharing: /mnt/host")
	}

	return nil
}
