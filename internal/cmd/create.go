package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/cloudinit"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/ssh"
)

// NewCreateCmd creates the create command
func NewCreateCmd() *cobra.Command {
	var name string
	var profile string
	var image string
	var userNetwork bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new cage",
		Long: `Create a new cage VM without starting it.

This creates the disk overlay, network, SSH keys, and VM definition.
Use 'cage start' to start the cage after creation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createCage(cmd, name, profile, image, userNetwork)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Name for the cage (required)")
	cmd.Flags().StringVarP(&profile, "profile", "p", "default", "Resource profile (default, heavy, light)")
	cmd.Flags().StringVarP(&image, "image", "i", "", "Base image (defaults to config default)")
	cmd.Flags().BoolVar(&userNetwork, "user-network", false, "Use user-mode networking (no root required, limited features)")

	cmd.MarkFlagRequired("name")

	return cmd
}

func createCage(cmd *cobra.Command, name, profileName, imageName string, userNetwork bool) error {
	// Check if cage already exists
	if cage.Exists(name) {
		return fmt.Errorf("cage '%s' already exists", name)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get profile
	profile, err := cfg.GetProfile(profileName)
	if err != nil {
		return err
	}

	// Determine image
	if imageName == "" {
		imageName = cfg.Images.Default
	}

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

	// Create network (unless user-network mode)
	var networkName string
	if !userNetwork {
		fmt.Fprintln(cmd.OutOrStdout(), "  Creating network...")
		if err := network.CreateNetwork(name); err != nil {
			cage.DeleteState(name)
			return fmt.Errorf("failed to create network: %w", err)
		}
		networkName = network.BridgeName(name)

		// Setup firewall rules
		fmt.Fprintln(cmd.OutOrStdout(), "  Setting up firewall...")
		firewallCfg := &network.FirewallConfig{
			BridgeName:        networkName,
			BlockedInterfaces: cfg.Network.BlockedInterfaces,
			BlockedSubnets:    cfg.Network.BlockedSubnets,
			AllowedDNS:        cfg.Network.DNS,
		}
		if err := network.SetupFirewall(name, firewallCfg); err != nil {
			network.DestroyNetwork(name)
			cage.DeleteState(name)
			return fmt.Errorf("failed to setup firewall: %w", err)
		}

		// Setup DNS DNAT
		if len(cfg.Network.DNS) > 0 {
			if err := network.SetupDNAT(name, cfg.Network.DNS[0]); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: DNS DNAT setup failed: %v\n", err)
			}
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  Using user-mode networking (SLIRP)...")
	}

	// Create qcow2 overlay with specified disk size
	baseImage := images.ImagePath(imageName)
	overlayPath := filepath.Join(cageDir, "disk.qcow2")
	diskSize := fmt.Sprintf("%dG", profile.DiskGB)

	fmt.Fprintf(cmd.OutOrStdout(), "  Creating disk overlay (%s)...\n", diskSize)
	createCmd := exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", baseImage, "-F", "qcow2", overlayPath, diskSize)
	if out, err := createCmd.CombinedOutput(); err != nil {
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

	// Create cloud-init ISO
	fmt.Fprintln(cmd.OutOrStdout(), "  Creating cloud-init...")
	cloudInitPath, err := cloudinit.GenerateISOWithConfig(cageDir, &cloudinit.CloudInitConfig{
		CageName:      name,
		PubKey:        pubKey,
		MountVirtiofs: false, // Will be set at start time if virtiofsd is available
		Env:           cfg.Env,
	})
	if err != nil {
		cage.DeleteState(name)
		return fmt.Errorf("failed to create cloud-init: %w", err)
	}

	// Generate domain XML
	domainCfg := &libvirt.DomainConfig{
		Name:           name,
		MemoryMB:       profile.MemoryMB,
		VCPU:           profile.VCPU,
		DiskPath:       overlayPath,
		CloudInitISO:   cloudInitPath,
		NetworkName:    networkName, // Empty for user-mode networking
		VirtiofsSocket: "",          // Set at start time
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

	// Save state as stopped
	state := &cage.State{
		Name:        name,
		Status:      cage.StatusStopped,
		Image:       imageName,
		Profile:     profileName,
		UserNetwork: userNetwork,
	}

	if err := cage.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Cage '%s' created\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s, Profile: %s (%d vCPU, %d MB RAM)\n",
		imageName, profileName, profile.VCPU, profile.MemoryMB)
	fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage start %s' to start\n", name)

	return nil
}
