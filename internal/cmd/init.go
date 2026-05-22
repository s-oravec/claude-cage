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
		image    string
		cage     string
		memory   string
		vcpu     int
		disk     int
		ssh      string
		force    bool
		dir      string
		rootMode bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new cage configuration in the current directory",
		Long: `Initialize a new .cage.yml configuration file.

By default the config is for user mode: image, SSH and resources only. The
cage runs under your regular user with libvirt session mode. Use --root to
add a default share that maps the current directory to /workspace (this
requires running the cage with 'sudo cage start' — see docs/modes.md).

If --image is not specified, uses the default image from ~/.claude-cage/config.yaml.

Example:
  cage init                              # user-mode cagefile (no shares)
  cage init --root                       # root-mode cagefile (workspace share, sudo)
  cage init --image ubuntu-24.04
  cage init --image debian-12 --memory 8G --vcpu 4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, image, cage, memory, vcpu, disk, ssh, force, dir, rootMode)
		},
	}

	cmd.Flags().StringVar(&image, "image", "", "Base image name (default: from config)")
	cmd.Flags().StringVar(&cage, "cage", "", "Cage name (default: directory name)")
	cmd.Flags().StringVar(&memory, "memory", "", "Memory allocation (e.g., 4G, 8G)")
	cmd.Flags().IntVar(&vcpu, "vcpu", 0, "Number of virtual CPUs")
	cmd.Flags().IntVar(&disk, "disk", 0, "Disk size in GB")
	cmd.Flags().StringVar(&ssh, "ssh", "auto", "SSH port or 'auto'")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing configuration")
	cmd.Flags().StringVar(&dir, "dir", "", "Target directory (default: current directory)")
	cmd.Flags().BoolVar(&rootMode, "root", false, "Generate a root-mode cagefile (adds workspace share, requires sudo cage start)")

	return cmd
}

func runInit(cmd *cobra.Command, image, cage, memory string, vcpu, disk int, ssh string, force bool, dir string, rootMode bool) error {
	// If no image specified, try to get default from global config
	if image == "" {
		cfg, err := config.Load()
		if err == nil && cfg.Images.Default != "" {
			image = cfg.Images.Default
		} else {
			return fmt.Errorf("--image is required (or set images.default in ~/.claude-cage/config.yaml)")
		}
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
	}

	if rootMode {
		cfg.Shares = []config.ShareConfig{
			{Host: ".", Guest: "/workspace"},
		}
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
# See: https://github.com/s-oravec/claude-cage
#
# User mode (this file, default): no shares, no env injection. Run with 'cage start'.
# Root mode: add 'shares:' or 'env:' below. Then run 'sudo cage start' (required
# for virtiofs). See docs/modes.md for details.

`
	// Commented-out network example: discoverable without changing defaults.
	// On the default auto/SLIRP path the cage is isolated from the LAN/private
	// ranges. Uncomment under 'network:' to reach specific subnets while staying
	// isolated, or to disable isolation entirely (less secure).
	networkExample := `# network:
#   isolation: true                    # block LAN/private ranges (default true)
#   allowed_subnets: [192.168.1.0/24]  # extra subnets the cage may reach while isolated
`
	content := header + string(data) + networkExample

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
