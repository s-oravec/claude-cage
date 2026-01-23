package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/cage"
	"github.com/stiivo/cage/internal/cloudinit"
	"github.com/stiivo/cage/internal/config"
	"github.com/stiivo/cage/internal/images"
	"github.com/stiivo/cage/internal/libvirt"
	"github.com/stiivo/cage/internal/ssh"
)

// NewStartCmd creates the start command
func NewStartCmd() *cobra.Command {
	var name string
	var profile string
	var image string
	var ports []string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new cage VM",
		Long: `Start a new cage VM with the specified configuration.

The VM is created with a copy-on-write overlay of the base image,
so changes inside the VM don't affect the base image.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return startCage(cmd, name, profile, image, ports)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Name for the cage (required)")
	cmd.Flags().StringVarP(&profile, "profile", "p", "default", "Resource profile (default, heavy, light)")
	cmd.Flags().StringVarP(&image, "image", "i", "", "Base image (defaults to config default)")
	cmd.Flags().StringArrayVar(&ports, "port", nil, "Port forwarding (e.g., 8080:80)")

	cmd.MarkFlagRequired("name")

	return cmd
}

func startCage(cmd *cobra.Command, name, profileName, imageName string, ports []string) error {
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

	fmt.Fprintf(cmd.OutOrStdout(), "Starting cage '%s'...\n", name)

	// Create cage directory
	cageDir := cage.Dir(name)
	if err := cage.EnsureDir(name); err != nil {
		return fmt.Errorf("failed to create cage directory: %w", err)
	}

	// Create qcow2 overlay
	baseImage := images.ImagePath(imageName)
	overlayPath := filepath.Join(cageDir, "disk.qcow2")

	fmt.Fprintln(cmd.OutOrStdout(), "  Creating disk overlay...")
	createCmd := exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", baseImage, "-F", "qcow2", overlayPath)
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

	// Create cloud-init ISO with SSH key
	fmt.Fprintln(cmd.OutOrStdout(), "  Creating cloud-init...")
	cloudInitPath, err := cloudinit.GenerateISO(cageDir, name, pubKey)
	if err != nil {
		cage.DeleteState(name)
		return fmt.Errorf("failed to create cloud-init: %w", err)
	}

	// Generate domain XML
	domainCfg := &libvirt.DomainConfig{
		Name:         name,
		MemoryMB:     profile.MemoryMB,
		VCPU:         profile.VCPU,
		DiskPath:     overlayPath,
		CloudInitISO: cloudInitPath,
		NetworkName:  "default", // Use libvirt default network for now
	}

	xml, err := libvirt.GenerateDomainXML(domainCfg)
	if err != nil {
		return fmt.Errorf("failed to generate domain XML: %w", err)
	}

	// Create libvirt client
	client := libvirt.NewClient()

	// Define and start domain
	fmt.Fprintln(cmd.OutOrStdout(), "  Creating VM...")
	if err := client.DefineDomain(xml); err != nil {
		cage.DeleteState(name) // cleanup
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "  Starting VM...")
	if err := client.StartDomain(name); err != nil {
		client.UndefineDomain(name) // cleanup
		ssh.DeleteKeys(name)
		cage.DeleteState(name)
		return err
	}

	// Wait for VM to get an IP
	fmt.Fprint(cmd.OutOrStdout(), "  Waiting for IP address...")
	var ip string
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

	// Save state
	state := &cage.State{
		Name:      name,
		Status:    cage.StatusRunning,
		Image:     imageName,
		Profile:   profileName,
		IP:        ip,
		StartedAt: time.Now(),
	}

	// Parse ports
	for _, p := range ports {
		// TODO: Parse port spec in Phase 09
		_ = p
	}

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

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Cage '%s' started\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s, Profile: %s (%d vCPU, %d MB RAM)\n",
		imageName, profileName, profile.VCPU, profile.MemoryMB)

	if ip != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Use 'cage ssh %s' to connect\n", name)
	}

	return nil
}
