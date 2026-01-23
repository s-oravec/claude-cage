package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/cage"
	"github.com/stiivo/cage/internal/config"
	"github.com/stiivo/cage/internal/images"
	"github.com/stiivo/cage/internal/libvirt"
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

	// Create minimal cloud-init ISO (for now, just a placeholder)
	// Full cloud-init with SSH keys will be in Phase 05
	cloudInitPath := filepath.Join(cageDir, "cloud-init.iso")
	if err := createMinimalCloudInit(cloudInitPath); err != nil {
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
		cage.DeleteState(name)
		return err
	}

	// Save state
	state := &cage.State{
		Name:      name,
		Status:    cage.StatusRunning,
		Image:     imageName,
		Profile:   profileName,
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

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Cage '%s' started\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s, Profile: %s (%d vCPU, %d MB RAM)\n",
		imageName, profileName, profile.VCPU, profile.MemoryMB)

	return nil
}

// createMinimalCloudInit creates a minimal cloud-init ISO
// This is a placeholder - full implementation in Phase 05
func createMinimalCloudInit(path string) error {
	// For now, create an empty ISO using genisoimage or mkisofs
	// This allows the VM to boot without cloud-init errors

	// Check which tool is available
	var cmd *exec.Cmd
	if _, err := exec.LookPath("genisoimage"); err == nil {
		cmd = exec.Command("genisoimage", "-output", path, "-volid", "cidata",
			"-joliet", "-rock", "/dev/null")
	} else if _, err := exec.LookPath("mkisofs"); err == nil {
		cmd = exec.Command("mkisofs", "-output", path, "-volid", "cidata",
			"-joliet", "-rock", "/dev/null")
	} else {
		// Create empty file as fallback
		return exec.Command("touch", path).Run()
	}

	return cmd.Run()
}
