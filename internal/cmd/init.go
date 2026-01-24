package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/s-oravec/claude-cage/internal/config"
)

// NewInitCmd creates the init command
func NewInitCmd() *cobra.Command {
	var (
		image  string
		cage   string
		memory string
		vcpu   int
		disk   int
		ssh    string
		force  bool
		dir    string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new cage configuration in the current directory",
		Long: `Initialize a new .claude-cage.yml configuration file.

This creates a project-level configuration that defines the cage settings
for this directory. The configuration includes the base image, resources,
and a default share mapping the current directory to /workspace.

Example:
  cage init --image ubuntu-24.04
  cage init --image debian-12 --memory 8G --vcpu 4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, image, cage, memory, vcpu, disk, ssh, force, dir)
		},
	}

	cmd.Flags().StringVar(&image, "image", "", "Base image name (required)")
	cmd.Flags().StringVar(&cage, "cage", "", "Cage name (default: directory name)")
	cmd.Flags().StringVar(&memory, "memory", "", "Memory allocation (e.g., 4G, 8G)")
	cmd.Flags().IntVar(&vcpu, "vcpu", 0, "Number of virtual CPUs")
	cmd.Flags().IntVar(&disk, "disk", 0, "Disk size in GB")
	cmd.Flags().StringVar(&ssh, "ssh", "auto", "SSH port or 'auto'")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing configuration")
	cmd.Flags().StringVar(&dir, "dir", "", "Target directory (default: current directory)")

	return cmd
}

func runInit(cmd *cobra.Command, image, cage, memory string, vcpu, disk int, ssh string, force bool, dir string) error {
	// Validate required flag
	if image == "" {
		return fmt.Errorf("--image is required")
	}

	// Get target directory
	targetDir := dir
	if targetDir == "" {
		var err error
		targetDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	configPath := filepath.Join(targetDir, config.ProjectConfigFile)

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		if !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", config.ProjectConfigFile)
		}
	}

	// Build project config
	cfg := config.ProjectConfig{
		Image: image,
		Network: config.ProjectNetwork{
			SSH: ssh,
		},
		Shares: []config.ShareConfig{
			{
				Host:  ".",
				Guest: "/workspace",
			},
		},
	}

	// Set optional fields only if provided
	if cage != "" {
		cfg.Cage = cage
	}
	if memory != "" {
		cfg.Memory = memory
	}
	if vcpu > 0 {
		cfg.VCPU = vcpu
	}
	if disk > 0 {
		cfg.DiskGB = disk
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := `# Cage configuration for this project
# See: https://github.com/anthropics/claude-cage

`
	content := header + string(data)

	// Write file
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", configPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s\n", image)
	if cage != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Cage: %s\n", cage)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nRun 'cage start' to create and start your cage.\n")

	return nil
}
