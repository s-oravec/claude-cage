package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/imgstore"
	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/s-oravec/claude-cage/internal/progress"
	"github.com/s-oravec/claude-cage/internal/registry"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// NewPullCmd creates the pull command
func NewPullCmd() *cobra.Command {
	var base string
	var list bool
	var platform string
	var concurrency int

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
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeBaseImageNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return listImages(cmd)
			}

			arch, err := resolvePlatform(platform)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				if ref, err := imgstore.ParseRef(args[0]); err == nil && ref.IsRegistry() {
					return printAPIErrorHint(runRegistryPull(cmd, ref, arch, concurrency))
				}
				base = args[0]
			}

			if base == "" {
				base = "alpine" // default
			}

			return pullImage(cmd, base, arch)
		},
	}

	cmd.Flags().StringVarP(&base, "base", "b", "", "Base image to download (e.g., ubuntu-24.04)")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "List available base images")
	cmd.Flags().StringVar(&platform, "platform", "", "Target architecture (amd64|arm64). Defaults to host architecture.")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "j", 3, "Max layers to download in parallel")

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
	fmt.Fprintln(cmd.OutOrStdout(), "Aliases: "+strings.Join(images.AliasNames(), ", "))
	fmt.Fprintln(cmd.OutOrStdout(), "Use 'cage pull --base <name>' to download an image")
	return nil
}

func pullImage(cmd *cobra.Command, name, arch string) error {
	// Already have this base for the requested arch -> nothing to do. A cached
	// copy for a different arch is overwritten by the download below.
	if images.IsDownloaded(name) && images.BaseArch(name) == arch {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Image already downloaded: %s\n", name)
		return nil
	}

	// Validate image name and get size info for the requested arch
	src, err := images.GetSource(name, arch)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s (%s)...\n", name, src.Description)

	// Create progress bar lazily: the size is only known once the HTTP response
	// arrives. Using a lazily-created Group means that if no bar is ever needed
	// (total stays unknown), no mpb container is created at all.
	var pg *progress.Group
	var bar *progress.LayerBar
	tornDown := false
	// teardown completes the bar (so pg.Wait does not block on an incomplete
	// bar) and then waits for the render goroutine to exit. It is idempotent and
	// runs on every exit path via defer, so a mid-download failure (where status
	// is never called) still tears the live region down.
	teardown := func() {
		if tornDown {
			return
		}
		tornDown = true
		if bar != nil {
			bar.Done("")
		}
		if pg != nil {
			pg.Wait()
		}
	}
	defer teardown()

	progressFn := func(written, total int64) {
		if bar == nil && total > 0 {
			pg = progress.NewGroup(cmd.OutOrStdout())
			bar = pg.AddBar(total, name, "downloading")
		}
		if bar != nil {
			bar.SetCurrent(written)
		}
	}
	status := func(msg string) {
		teardown() // Done + Wait before printing, so the final frame flushes first
		fmt.Fprintln(cmd.OutOrStdout(), msg)
	}
	return images.Setup(name, arch, progressFn, status)
}

// selectArchManifest dispatches on the manifest endpoint's Content-Type and returns
// the concrete single-arch manifest (body, digest, parsed) to materialize locally.
//   - manifest content type: requires m.Config.Arch == arch, else an error telling the
//     user to retry with --platform <that arch>.
//   - index content type: picks the entry whose platform.architecture == arch and
//     fetches that manifest via fetch(entry.Digest); errors (listing available arches)
//     if no entry matches.
//
// fetch(reference) returns (body, dockerDigest, err) for a manifest reference.
func selectArchManifest(arch, contentType string, body []byte, digest string,
	fetch func(reference string) ([]byte, string, error),
) (selBody []byte, selDigest string, m *manifest.Manifest, err error) {
	switch contentType {
	case manifest.MediaTypeManifestV1:
		var mm manifest.Manifest
		if err := json.Unmarshal(body, &mm); err != nil {
			return nil, "", nil, err
		}
		if mm.Config.Arch != arch {
			return nil, "", nil, fmt.Errorf(
				"image architecture is %s, but %s was requested; retry with --platform %s to pull it",
				mm.Config.Arch, arch, mm.Config.Arch)
		}
		return body, digest, &mm, nil
	case manifest.MediaTypeIndexV1:
		var idx manifest.IndexBody
		if err := json.Unmarshal(body, &idx); err != nil {
			return nil, "", nil, err
		}
		var pick *manifest.IndexEntry
		for i := range idx.Manifests {
			if idx.Manifests[i].Platform.Architecture == arch {
				pick = &idx.Manifests[i]
				break
			}
		}
		if pick == nil {
			return nil, "", nil, fmt.Errorf("image does not provide %s; available: %s",
				arch, strings.Join(archesOf(idx), ", "))
		}
		mBody, mDigest, err := fetch(pick.Digest)
		if err != nil {
			return nil, "", nil, err
		}
		var mm manifest.Manifest
		if err := json.Unmarshal(mBody, &mm); err != nil {
			return nil, "", nil, err
		}
		if mm.Config.Arch != arch {
			return nil, "", nil, fmt.Errorf("index entry for %s resolved to a %s manifest", arch, mm.Config.Arch)
		}
		return mBody, mDigest, &mm, nil
	default:
		return nil, "", nil, fmt.Errorf("unexpected Content-Type %q from manifest endpoint", contentType)
	}
}

// archesOf collects the architecture of each index entry, preserving order.
func archesOf(idx manifest.IndexBody) []string {
	arches := make([]string, 0, len(idx.Manifests))
	for _, e := range idx.Manifests {
		arches = append(arches, e.Platform.Architecture)
	}
	return arches
}

// runRegistryPull pulls an image from a cage-hub registry. The arch parameter is
// the resolved target architecture; the manifest endpoint's Content-Type is used
// to dispatch between a single-arch manifest and a multi-arch index, selecting the
// concrete arch-specific manifest to materialize locally.
func runRegistryPull(cmd *cobra.Command, ref imgstore.Ref, arch string, concurrency int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rc, err := registry.NewClient(ref.Host, registry.Options{Insecure: cfg.IsInsecureRegistry(ref.Host)})
	if err != nil {
		return err
	}

	// Manifest. The endpoint may return either a single-arch manifest or a
	// multi-arch index; selectArchManifest dispatches on Content-Type and yields
	// the concrete arch-specific manifest (selBody/selDigest/m).
	body, ct, digest, err := rc.GetManifest(ref.Owner, ref.Name, ref.Tag)
	if err != nil {
		return err
	}
	fetch := func(reference string) ([]byte, string, error) {
		b, _, d, e := rc.GetManifest(ref.Owner, ref.Name, reference)
		return b, d, e
	}
	selBody, selDigest, m, err := selectArchManifest(arch, ct, body, digest, fetch)
	if err != nil {
		return err
	}
	if manifest.DigestBytes(selBody) != selDigest {
		return fmt.Errorf("manifest digest mismatch: server %s vs computed %s", selDigest, manifest.DigestBytes(selBody))
	}
	if err := imgstore.PutManifestBytes(selDigest, selBody); err != nil {
		return err
	}

	if err := m.Validate(); err != nil {
		return err
	}

	// Layers. Download all uncached layers concurrently with per-layer progress.
	if err := pullLayers(cmd.OutOrStdout(), rc, ref.Owner, ref.Name, m.Layers, concurrency,
		imgstore.HasLayer, imgstore.PutLayerStreamed); err != nil {
		return err
	}

	// Base image check. The base must match the manifest's architecture; (re)pull
	// it when missing or when the cached copy is for a different arch.
	if !images.IsDownloaded(m.Base.Name) || images.BaseArch(m.Base.Name) != m.Config.Arch {
		fmt.Fprintf(cmd.OutOrStdout(), "  base %s (%s): pulling...\n", m.Base.Name, m.Config.Arch)
		if err := pullImage(cmd, m.Base.Name, m.Config.Arch); err != nil {
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

	// Ref. The local ref points at the arch-specific manifest digest (selDigest),
	// never the index digest.
	if err := imgstore.WriteRef(ref, selDigest); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Pulled %s\n", ref.Host+"/"+ref.Owner+"/"+ref.Name+":"+ref.Tag)
	return nil
}

// pullLayers downloads all uncached layers concurrently (up to `concurrency`
// at once), rendering per-layer progress to out. hasLayer/putLayer are injected
// (production: imgstore.HasLayer / imgstore.PutLayerStreamed) so the pool is
// testable without touching the local store. The first error from any worker is
// returned; the progress live region is always torn down so the terminal is not
// left dirty.
func pullLayers(
	out io.Writer,
	rc *registry.Client,
	owner, name string,
	layers []manifest.Layer,
	concurrency int,
	hasLayer func(digest string) bool,
	putLayer func(digest string, fetch imgstore.FetchFn) error,
) error {
	if concurrency < 1 {
		concurrency = 1
	}

	pg := progress.NewGroup(out)
	var g errgroup.Group
	g.SetLimit(concurrency)

	for _, l := range layers {
		l := l // capture loop var
		g.Go(func() error {
			bar := pg.AddBar(l.Size, shortDigest(l.Digest), "downloading")
			if hasLayer(l.Digest) {
				bar.Done("cached")
				return nil
			}
			err := putLayer(l.Digest, func(offset int64) (io.ReadCloser, error) {
				if offset > 0 {
					bar.SetCurrent(offset)
				}
				rc2, err := rc.GetBlob(owner, name, l.Digest, offset)
				if err != nil {
					return nil, err
				}
				return &progressReadCloser{rc: rc2, n: offset, bar: bar}, nil
			})
			if err != nil {
				return err
			}
			bar.Done("")
			return nil
		})
	}

	// Wait on the group FIRST to collect the error, THEN tear down the live
	// region so the terminal is clean even on error.
	err := g.Wait()
	pg.Wait()
	return err
}
