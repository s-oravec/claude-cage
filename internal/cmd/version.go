package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time
var Version = "0.1.0"

// NewVersionCmd creates the version command
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print cage version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "cage version %s\n", Version)
			fmt.Fprintf(cmd.OutOrStdout(), "QEMU/KVM backend\n")
		},
	}
}
