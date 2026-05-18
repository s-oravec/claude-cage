package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/s-oravec/claude-cage/internal/imgstore"
)

// NewTagCmd returns the cobra command for `cage tag`.
func NewTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <src> <dst>",
		Short: "Create a new tag pointing at an existing local image",
		Long: `Create a new local reference that points at the same manifest as <src>.

Both arguments use the standard ref format (local "name[:tag]" or
"host/owner/name[:tag]"). No bytes are moved; only a ref file is
created. If <dst> already exists, it is overwritten (matches
docker tag).`,
		Args: cobra.ExactArgs(2),
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
}
