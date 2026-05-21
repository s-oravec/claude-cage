package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/imgstore"
	"github.com/s-oravec/claude-cage/internal/manifest"
	"github.com/s-oravec/claude-cage/internal/registry"
)

// NewTagCmd returns the cobra command for `cage tag`.
//
// The bare command (`cage tag <src> <dst>`) creates a local ref. It also carries
// an `inspect` subcommand: cobra routes `cage tag inspect <ref>` to the subcommand
// while `cage tag a b` still hits this parent's RunE.
func NewTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <src> <dst>",
		Short: "Create a new tag pointing at an existing local image",
		Long: `Create a new local reference that points at the same manifest as <src>.

Both arguments use the standard ref format (local "name[:tag]" or
"host/owner/name[:tag]"). No bytes are moved; only a ref file is
created. If <dst> already exists, it is overwritten (matches
docker tag).`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completeLocalRefs(false)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := imgstore.ParseRef(args[0])
			if err != nil {
				return fmt.Errorf("src: %w", err)
			}
			dst, err := imgstore.ParseRef(args[1])
			if err != nil {
				return fmt.Errorf("dst: %w", err)
			}
			digest, err := imgstore.ReadRef(src)
			if err != nil {
				return fmt.Errorf("image not found: %s (run `cage image list` to see available)", args[0])
			}
			return imgstore.WriteRef(dst, digest)
		},
	}

	cmd.AddCommand(newTagInspectCmd())

	return cmd
}

// newTagInspectCmd returns the `cage tag inspect <ref>` subcommand.
func newTagInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <ref>",
		Short: "Show kind, digest and architectures for a ref",
		Long: `Inspect a ref and report its kind (manifest or index), its digest, and
the architecture(s) it provides.

For a registry ref (host/owner/name[:tag]) the manifest endpoint is queried:
a single-arch manifest reports one architecture, a multi-arch index reports
all of them. For a local ref the recorded single-arch manifest is read.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return completeLocalRefs(false)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return inspectTag(cmd, args[0])
		},
	}
}

// inspectTag resolves refStr (registry or local), determines the kind, digest and
// architectures, and prints them. Registry refs query the manifest endpoint and
// dispatch on Content-Type; local refs read the recorded single-arch manifest.
func inspectTag(cmd *cobra.Command, refStr string) error {
	ref, err := imgstore.ParseRef(refStr)
	if err != nil {
		return err
	}

	if ref.IsRegistry() {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		rc, err := registry.NewClient(ref.Host, registry.Options{Insecure: cfg.IsInsecureRegistry(ref.Host)})
		if err != nil {
			return err
		}
		body, ct, dockerDigest, err := rc.GetManifest(ref.Owner, ref.Name, ref.Tag)
		if err != nil {
			return err
		}
		kind, digest, arches, err := tagInspectFromManifest(ct, body, dockerDigest)
		if err != nil {
			return err
		}
		return printTagInspect(cmd.OutOrStdout(), kind, digest, arches)
	}

	// Local ref: always a single-arch manifest after pull dispatch.
	digest, err := imgstore.ReadRef(ref)
	if err != nil {
		return fmt.Errorf("image not found: %s", refStr)
	}
	body, err := imgstore.GetManifestBytes(digest)
	if err != nil {
		return err
	}
	var m manifest.Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	return printTagInspect(cmd.OutOrStdout(), "manifest", digest, m.Config.Arch)
}

// tagInspectFromManifest dispatches on a manifest endpoint's Content-Type and
// returns the kind ("manifest" or "index"), the server-reported digest, and the
// comma-joined architecture(s) for printing.
func tagInspectFromManifest(contentType string, body []byte, dockerDigest string) (kind, digest, arches string, err error) {
	switch contentType {
	case manifest.MediaTypeManifestV1:
		var m manifest.Manifest
		if err := json.Unmarshal(body, &m); err != nil {
			return "", "", "", err
		}
		return "manifest", dockerDigest, m.Config.Arch, nil
	case manifest.MediaTypeIndexV1:
		var idx manifest.IndexBody
		if err := json.Unmarshal(body, &idx); err != nil {
			return "", "", "", err
		}
		return "index", dockerDigest, strings.Join(archesOf(idx), ", "), nil
	default:
		return "", "", "", fmt.Errorf("unexpected Content-Type %q", contentType)
	}
}

// printTagInspect renders the aligned Kind/Digest/Architectures block.
func printTagInspect(out io.Writer, kind, digest, arches string) error {
	tw := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
	fmt.Fprintf(tw, "Kind:\t%s\n", kind)
	fmt.Fprintf(tw, "Digest:\t%s\n", digest)
	fmt.Fprintf(tw, "Architectures:\t%s\n", arches)
	return tw.Flush()
}
