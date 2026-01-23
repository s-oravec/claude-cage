package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cage",
		Short: "Claude Cage - Secure VM sandbox for Claude Code",
		Long: `Claude Cage creates isolated QEMU/KVM virtual machines
for running Claude Code in a secure sandbox with full Docker support.`,
	}

	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewDoctorCmd())
	rootCmd.AddCommand(NewConfigCmd())

	return rootCmd
}
