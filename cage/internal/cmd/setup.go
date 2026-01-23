package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/images"
)

// NewSetupCmd creates the setup command
func NewSetupCmd() *cobra.Command {
	var base string
	var list bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Download and prepare base images",
		Long: `Download and prepare base images for cage VMs.

Without arguments, downloads the default image (ubuntu-24.04).
Use --base to specify a different image.
Use --list to see available images.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return listImages(cmd)
			}

			if base == "" {
				base = "ubuntu-24.04" // default
			}

			return setupImage(cmd, base)
		},
	}

	cmd.Flags().StringVarP(&base, "base", "b", "", "Base image to download (e.g., ubuntu-24.04)")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "List available base images")

	return cmd
}

func listImages(cmd *cobra.Command) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Available base images:")
	fmt.Fprintln(cmd.OutOrStdout())

	sources := images.BaseImages()
	downloaded := make(map[string]bool)
	for _, name := range images.ListDownloaded() {
		downloaded[name] = true
	}

	for name, src := range sources {
		status := "  "
		if downloaded[name] {
			status = "✓ "
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s%-15s %s\n", status, name, src.Description)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Use 'cage setup --base <name>' to download an image")
	return nil
}

func setupImage(cmd *cobra.Command, name string) error {
	// Check if already downloaded
	if images.IsDownloaded(name) {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Image already downloaded: %s\n", name)
		return nil
	}

	// Validate image name
	if _, err := images.GetSource(name); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", name)

	var lastPercent int
	progress := func(written, total int64) {
		if total <= 0 {
			return
		}
		percent := int(written * 100 / total)
		if percent > lastPercent && percent%10 == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d%% (%d MB / %d MB)\n",
				percent, written/1024/1024, total/1024/1024)
			lastPercent = percent
		}
	}

	status := func(msg string) {
		fmt.Fprintln(cmd.OutOrStdout(), msg)
	}

	return images.Setup(name, progress, status)
}
