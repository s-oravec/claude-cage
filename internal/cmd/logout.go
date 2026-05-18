package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/s-oravec/claude-cage/internal/auth"
)

// NewLogoutCmd returns the cobra command for `cage logout`.
func NewLogoutCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "logout [host]",
		Short: "Remove stored credentials for a registry",
		Long: `Remove credentials for a cage-hub registry stored in
~/.claude-cage/auth.yaml. This is a local-only operation; the token
itself remains valid on the server until it expires or you revoke it
via the cage-hub web UI (/settings/tokens) or DELETE /api/v1/me/pats/:id.

With --all, clears every host. Otherwise the single positional <host>
argument is required.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return auth.RemoveAll()
			}
			if len(args) != 1 {
				return fmt.Errorf("usage: cage logout <host>  (or --all)")
			}
			return auth.RemoveHost(args[0])
		},
	}
	c.Flags().BoolVar(&all, "all", false, "Remove all stored credentials")
	return c
}
