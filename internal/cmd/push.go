package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/s-oravec/cage/internal/auth"
	"github.com/s-oravec/cage/internal/config"
	"github.com/s-oravec/cage/internal/imgstore"
	"github.com/s-oravec/cage/internal/manifest"
	"github.com/s-oravec/cage/internal/progress"
	"github.com/s-oravec/cage/internal/registry"
	"github.com/s-oravec/cage/internal/tokensrc"
)

// NewPushCmd returns the cobra command for `cage push`.
func NewPushCmd() *cobra.Command {
	var asLatest bool
	var concurrency int
	c := &cobra.Command{
		Use:   "push <ref>",
		Short: "Push an image to a cage-hub registry",
		Long: "Push a local image to its cage-hub registry. <ref> MUST be a " +
			"fully-qualified reference of the form host/owner/name[:tag] " +
			"(defaults to \"latest\"). Requires `cage login <host>` first.\n\n" +
			"The push flow:\n" +
			"  1. HEAD each layer; uploaded layers already on the server are skipped.\n" +
			"  2. For missing layers, choose single-PUT or multipart upload by size\n" +
			"     (multipart kicks in at ~4x multipart_part_size from /auth/info).\n" +
			"  3. PUT the manifest with the X-As-Latest header set when --latest.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeLocalRefs(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			return printAPIErrorHint(runPush(cmd.OutOrStdout(), args[0], asLatest, concurrency))
		},
	}
	c.Flags().BoolVar(&asLatest, "latest", false, "Also update the `latest` tag pointer")
	c.Flags().IntVarP(&concurrency, "concurrency", "j", 3, "Max layers to upload in parallel")
	return c
}

func runPush(out io.Writer, refStr string, asLatest bool, concurrency int) error {
	ref, err := imgstore.ParseRef(refStr)
	if err != nil {
		return err
	}
	if !ref.IsRegistry() {
		return fmt.Errorf("ref must be a registry ref (host/owner/name:tag), got %q", refStr)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token, ok := auth.Token(ref.Host)
	if !ok {
		return fmt.Errorf("not logged in to %s - run `cage login %s`", ref.Host, ref.Host)
	}

	manifestDigest, err := imgstore.ReadRef(ref)
	if err != nil {
		return fmt.Errorf("no local image tagged %s", refStr)
	}
	manifestBytes, err := imgstore.GetManifestBytes(manifestDigest)
	if err != nil {
		return err
	}
	var m manifest.Manifest
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		return err
	}

	rc, err := registry.NewClient(ref.Host, registry.Options{
		Token: token, Insecure: cfg.IsInsecureRegistry(ref.Host),
	})
	if err != nil {
		return err
	}

	info, err := rc.AuthInfo()
	if err != nil {
		return err
	}

	// If we logged in via the device flow we have a refresh token; attach a
	// refreshing provider so a long push survives access-token expiry.
	if a, _ := auth.Load(); a != nil {
		if e, ok := a.Registries[ref.Host]; ok && e.RefreshToken != "" {
			rc.SetTokenProvider(tokensrc.NewRefreshing(ref.Host, info.ClientID, info.TokenEndpoint))
		}
	}

	// Push all missing layers concurrently.
	if err := pushLayers(out, rc, ref.Owner, ref.Name, m.Layers, info.MultipartPartSize, concurrency,
		func(d string) (io.ReadCloser, error) { return os.Open(imgstore.LayerPath(d)) }); err != nil {
		return err
	}

	res, err := rc.PutManifest(ref.Owner, ref.Name, ref.Tag, manifestBytes, asLatest)
	if err != nil {
		return err
	}
	printPushResult(out, fmt.Sprintf("%s/%s:%s", ref.Owner, ref.Name, ref.Tag), m.Config.Arch, res)
	return nil
}

// pushLayers uploads all missing layers concurrently (up to `concurrency` at
// once), rendering per-layer progress to out. openLayer returns the blob body
// for a digest (production: os.Open(imgstore.LayerPath(digest))). The first
// error from any worker is returned; the progress live region is always torn
// down so the terminal is not left dirty.
func pushLayers(
	out io.Writer,
	rc *registry.Client,
	owner, name string,
	layers []manifest.Layer,
	partSize int64,
	concurrency int,
	openLayer func(digest string) (io.ReadCloser, error),
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
			exists, err := rc.HeadBlob(owner, name, l.Digest)
			if err != nil {
				return err
			}
			bar := pg.AddBar(l.Size, shortDigest(l.Digest), "uploading")
			if exists {
				bar.Done("exists")
				return nil
			}
			body, err := openLayer(l.Digest)
			if err != nil {
				return err
			}
			defer body.Close()
			if err := rc.UploadBlob(owner, name, l.Digest, l.Size, partSize, body, func(u int64) {
				bar.SetCurrent(u)
			}); err != nil {
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

// printPushResult renders the post-push tag state. tagLabel is "<owner>/<name>:<tag>",
// arch is the pushed manifest's Config.Arch.
func printPushResult(out io.Writer, tagLabel, arch string, res *registry.PutManifestResult) {
	fmt.Fprintf(out, "Pushed: %s (%s)\n", shortDigest(res.ManifestDigest), arch)
	// tag_target_kind is "manifest" or "index"; treat any non-"index" value as a
	// direct manifest target (the default server contract).
	if res.TagTargetKind == "index" {
		fmt.Fprintf(out, "Tag %s -> index %s (auto-composed by server)\n", tagLabel, shortDigest(res.TagTargetDigest))
	} else {
		fmt.Fprintf(out, "Tag %s -> manifest %s\n", tagLabel, shortDigest(res.TagTargetDigest))
	}
	if res.LatestUpdated {
		fmt.Fprintf(out, "Updated latest -> %s\n", shortDigest(res.TagTargetDigest))
	}
}

// shortDigest truncates a "sha256:<64hex>" digest to "sha256:" + first 12 hex
// chars. Any other input is returned unchanged.
func shortDigest(d string) string {
	const prefix = "sha256:"
	if len(d) == len(prefix)+64 && strings.HasPrefix(d, prefix) {
		return d[:len(prefix)+12]
	}
	return d
}
