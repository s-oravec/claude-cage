package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s-oravec/claude-cage/internal/logging"
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	var (
		verbosity int
		logLevel  string
	)

	rootCmd := &cobra.Command{
		Use:   "cage",
		Short: "Claude Cage - Secure VM sandbox for Claude Code",
		Long: `Claude Cage creates isolated QEMU/KVM virtual machines
for running Claude Code in a secure sandbox with full Docker support.`,
		SilenceUsage: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return logging.Configure(verbosity, logLevel)
		},
	}

	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v",
		"Increase verbosity (-v for debug, -vv for trace; propagates to subprocesses at trace)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "",
		"Explicit log level: trace, debug, info, warn, error (overrides -v and CAGE_LOG_LEVEL)")

	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewDoctorCmd())
	rootCmd.AddCommand(NewConfigCmd())
	rootCmd.AddCommand(NewPullCmd())
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
	rootCmd.AddCommand(NewBuildCmd())

	return rootCmd
}
