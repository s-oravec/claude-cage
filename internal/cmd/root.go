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
	rootCmd.AddCommand(NewSetupCmd())
	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewStartCmd())
	rootCmd.AddCommand(NewStopCmd())
	rootCmd.AddCommand(NewRemoveCmd())
	rootCmd.AddCommand(NewListCmd())
	rootCmd.AddCommand(NewSSHCmd())
	rootCmd.AddCommand(NewVerifyCmd())
	rootCmd.AddCommand(NewStatusCmd())
	rootCmd.AddCommand(NewExecCmd())
	rootCmd.AddCommand(NewLogsCmd())
	rootCmd.AddCommand(NewPortCmd())
	rootCmd.AddCommand(NewRestartCmd())
	rootCmd.AddCommand(NewSnapshotCmd())
	rootCmd.AddCommand(NewImageCmd())
	rootCmd.AddCommand(NewConsoleCmd())

	return rootCmd
}
