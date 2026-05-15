package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/imgstore"
	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/s-oravec/claude-cage/internal/progress"
	"github.com/s-oravec/claude-cage/internal/registry"
	"github.com/spf13/cobra"
)

// NewPullCmd creates the pull command
func NewPullCmd() *cobra.Command {
	var base string
	var list bool

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Download and prepare base images, or pull from a registry",
		Long: `Download and prepare base images for cage VMs, or pull an image from
a cage-hub registry.

Without arguments, downloads the default image (alpine).
Use --base or a positional name to specify a different distro image.
Use --list to see available distro images.

A positional argument of the form host/owner/name[:tag] is treated as a
registry reference and pulled from the cage-hub registry; the manifest
and any missing layers are stored locally and the tag is written into
the local image store.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return listImages(cmd)
			}

			if len(args) == 1 {
				if ref, err := imgstore.ParseRef(args[0]); err == nil && ref.IsRegistry() {
					return runRegistryPull(cmd, ref)
				}
				base = args[0]
			}

			if base == "" {
				base = "alpine" // default
			}

			return pullImage(cmd, base)
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
	fmt.Fprintln(cmd.OutOrStdout(), "Use 'cage pull --base <name>' to download an image")
	return nil
}

func pullImage(cmd *cobra.Command, name string) error {
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

func runRegistryPull(cmd *cobra.Command, ref imgstore.Ref) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rc, err := registry.NewClient(ref.Host, registry.Options{Insecure: cfg.IsInsecureRegistry(ref.Host)})
	if err != nil {
		return err
	}

	// Manifest.
	body, digest, err := rc.GetManifest(ref.Owner, ref.Name, ref.Tag)
	if err != nil {
		return err
	}
	if manifest.DigestBytes(body) != digest {
		return fmt.Errorf("manifest digest mismatch: server %s vs computed %s", digest, manifest.DigestBytes(body))
	}
	if err := imgstore.PutManifestBytes(digest, body); err != nil {
		return err
	}

	var m manifest.Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}

	// Layers.
	for _, l := range m.Layers {
		if imgstore.HasLayer(l.Digest) {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: cached\n", l.Digest)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s: downloading\n", l.Digest)
		rc2, err := rc.GetBlob(ref.Owner, ref.Name, l.Digest, 0)
		if err != nil {
			return err
		}
		buf, err := io.ReadAll(rc2)
		rc2.Close()
		if err != nil {
			return err
		}
		if manifest.DigestBytes(buf) != l.Digest {
			return fmt.Errorf("layer digest mismatch: server %s, got %s", l.Digest, manifest.DigestBytes(buf))
		}
		if err := imgstore.PutLayerBytes(l.Digest, buf); err != nil {
			return err
		}
	}

	// Base image check.
	if !images.IsDownloaded(m.Base.Name) {
		fmt.Fprintf(cmd.OutOrStdout(), "  base %s: not found locally, pulling...\n", m.Base.Name)
		if err := pullImage(cmd, m.Base.Name); err != nil {
			return err
		}
	}
	have, err := images.BaseDigest(m.Base.Name)
	if err != nil {
		return err
	}
	if have != m.Base.Digest {
		return fmt.Errorf("local base image %s differs from one used to build this image (have %s, need %s); run `cage image rm %s` and `cage pull --base %s`",
			m.Base.Name, have, m.Base.Digest, m.Base.Name, m.Base.Name)
	}

	// Ref.
	if err := imgstore.WriteRef(ref, digest); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Pulled %s\n", ref.Host+"/"+ref.Owner+"/"+ref.Name+":"+ref.Tag)
	return nil
}
