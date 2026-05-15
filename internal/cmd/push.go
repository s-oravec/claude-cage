package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/s-oravec/claude-cage/internal/auth"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/imgstore"
	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/s-oravec/claude-cage/internal/registry"
)

// NewPushCmd returns the cobra command for `cage push`.
func NewPushCmd() *cobra.Command {
	var asLatest bool
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
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPush(cmd.OutOrStdout(), args[0], asLatest)
		},
	}
	c.Flags().BoolVar(&asLatest, "latest", false, "Also update the `latest` tag pointer")
	return c
}

func runPush(out io.Writer, refStr string, asLatest bool) error {
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
		return fmt.Errorf("auth/info: %w", err)
	}

	// Push each missing layer.
	for _, l := range m.Layers {
		exists, err := rc.HeadBlob(ref.Owner, ref.Name, l.Digest)
		if err != nil {
			return err
		}
		if exists {
			fmt.Fprintf(out, "  %s: exists\n", l.Digest)
			continue
		}
		f, err := os.Open(imgstore.LayerPath(l.Digest))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "  %s: uploading %d bytes\n", l.Digest, l.Size)
		err = rc.UploadBlob(ref.Owner, ref.Name, l.Digest, l.Size, info.MultipartPartSize, f)
		f.Close()
		if err != nil {
			return err
		}
	}

	res, err := rc.PutManifest(ref.Owner, ref.Name, ref.Tag, manifestBytes, asLatest)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Pushed %s -> %s (latest_updated=%v)\n", refStr, res.ManifestDigest, res.LatestUpdated)
	return nil
}
