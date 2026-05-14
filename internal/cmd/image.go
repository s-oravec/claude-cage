package cmd

import (
	"fmt"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/spf13/cobra"
)

// NewImageCmd creates the image command with subcommands
func NewImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage cage images",
		Long: `Manage base and custom images for cages.

Images are the templates used to create new cages. You can create
custom images from running cages to save your environment setup.`,
	}

	cmd.AddCommand(newImageListCmd())
	cmd.AddCommand(newImageSaveCmd())
	cmd.AddCommand(newImageRemoveCmd())
	cmd.AddCommand(newImageInspectCmd())

	return cmd
}

func newImageListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available images",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listAvailableImages(cmd)
		},
	}
}

func newImageSaveCmd() *cobra.Command {
	var name string
	var description string

	cmd := &cobra.Command{
		Use:   "save [cage-name]",
		Short: "Save a cage as a new image",
		Long: `Save the current state of a cage as a new custom image.

The image can then be used to create new cages with the same
software and configuration.

When run from a directory with .cage.yml, the cage name is optional.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cageName, _, err := resolveCageName(args)
			if err != nil {
				return err
			}
			return saveImage(cmd, cageName, name, description)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Image name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Image description")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newImageRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove <image-name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove an image",
		Long: `Remove an image from the system.

Base images require --force to remove. Images in use by cages cannot be removed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeImage(cmd, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove (required for base images)")

	return cmd
}

func newImageInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <image-name>",
		Short: "Show detailed information about an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return inspectImage(cmd, args[0])
		},
	}
}

func listAvailableImages(cmd *cobra.Command) error {
	imgList, err := images.List()
	if err != nil {
		return err
	}

	if len(imgList) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No images found. Run 'cage pull' to download base images.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-8s %-20s %-10s %s\n", "NAME", "TYPE", "BASE", "SIZE", "CREATED")
	for _, img := range imgList {
		created := "-"
		if !img.CreatedAt.IsZero() {
			created = img.CreatedAt.Format("2006-01-02")
		}
		base := "-"
		if img.Base != "" {
			base = truncateString(img.Base, 20)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-8s %-20s %-10s %s\n",
			truncateString(img.Name, 20),
			img.Type,
			base,
			images.FormatSize(img.Size),
			created)
	}

	return nil
}

func saveImage(cmd *cobra.Command, cageName, imageName, description string) error {
	// Check cage exists
	if !cage.Exists(cageName) {
		return fmt.Errorf("cage '%s' not found", cageName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Saving cage '%s' as image '%s'...\n", cageName, imageName)
	fmt.Fprintln(cmd.OutOrStdout(), "  Converting disk image...")

	result, err := images.Save(cageName, imageName, description)
	if err != nil {
		return err
	}

	// Get image info for size display
	img, err := images.Inspect(imageName)
	if err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Image '%s' saved (%s)\n", imageName, images.FormatSize(img.ActualSize))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Image '%s' saved\n", imageName)
	}

	if result.VirtCustomizeUsed {
		fmt.Fprintln(cmd.OutOrStdout(), "  SSH keys cleared, cloud-init reset - image ready for reuse")
	} else if result.VirtCustomizeError != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "  Warning: virt-customize failed (libguestfs issue)")
		fmt.Fprintln(cmd.OutOrStdout(), "  Image saved - SSH keys will be injected on first boot via cloud-init")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "  Image ready for reuse (SSH keys will be injected on first boot)")
	}

	return nil
}

func removeImage(cmd *cobra.Command, imageName string, force bool) error {
	if !images.Exists(imageName) {
		return fmt.Errorf("image '%s' not found", imageName)
	}

	if err := images.Delete(imageName, force); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Image '%s' removed\n", imageName)
	return nil
}

func inspectImage(cmd *cobra.Command, imageName string) error {
	details, err := images.Inspect(imageName)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Name:         %s\n", details.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Type:         %s\n", details.Type)
	if details.Base != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Base:         %s\n", details.Base)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Format:       %s\n", details.Format)
	fmt.Fprintf(cmd.OutOrStdout(), "Virtual Size: %s\n", images.FormatSize(details.VirtualSize))
	fmt.Fprintf(cmd.OutOrStdout(), "Actual Size:  %s\n", images.FormatSize(details.ActualSize))
	if !details.CreatedAt.IsZero() {
		fmt.Fprintf(cmd.OutOrStdout(), "Created:      %s\n", details.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	if details.Description != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Description:  %s\n", details.Description)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Path:         %s\n", details.Path)
	if details.BackingFile != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Backing:      %s\n", details.BackingFile)
	}

	return nil
}
