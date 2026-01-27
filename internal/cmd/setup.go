package cmd

import (
	"fmt"
	"sort"

	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/progress"
	"github.com/spf13/cobra"
)

// NewSetupCmd creates the setup command
func NewSetupCmd() *cobra.Command {
	var base string
	var list bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Download and prepare base images",
		Long: `Download and prepare base images for cage VMs.

Without arguments, downloads the default image (alpine).
Use --base to specify a different image.
Use --list to see available images.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return listImages(cmd)
			}

			if base == "" {
				base = "alpine" // default
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

	// Sort image names for consistent output
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		src := sources[name]
		status := "  "
		if downloaded[name] {
			status = "✓ "
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s%-18s %s\n", status, name, src.Description)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Aliases: alpine, ubuntu, debian, rocky, alma, fedora, opensuse, centos")
	fmt.Fprintln(cmd.OutOrStdout(), "Use 'cage setup --base <name>' to download an image")
	return nil
}

func setupImage(cmd *cobra.Command, name string) error {
	// Check if already downloaded
	if images.IsDownloaded(name) {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Image already downloaded: %s\n", name)
		return nil
	}

	// Validate image name and get size info
	src, err := images.GetSource(name)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s (%s)...\n", name, src.Description)

	// Create progress bar (size will be set from HTTP response)
	var bar *progress.Bar

	progressFn := func(written, total int64) {
		if bar == nil && total > 0 {
			bar = progress.NewBar(total, name, cmd.OutOrStdout())
		}
		if bar != nil {
			bar.Update(written)
		}
	}

	status := func(msg string) {
		if bar != nil {
			bar.Finish()
		}
		fmt.Fprintln(cmd.OutOrStdout(), msg)
	}

	return images.Setup(name, progressFn, status)
}
