package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/doctor"
)

// NewDoctorCmd creates the doctor command
func NewDoctorCmd() *cobra.Command {
	var showFix bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check system requirements",
		Long: `Check if all system requirements for running cage are met.

Use --fix to see installation commands for missing dependencies.`,
		Run: func(cmd *cobra.Command, args []string) {
			checks := doctor.DefaultChecks()
			results := doctor.RunChecks(checks)

			var failedChecks []doctor.CheckResult

			for _, r := range results {
				status := "✓"
				if !r.Passed {
					status = "✗"
					failedChecks = append(failedChecks, r)
				}
				suffix := ""
				if !r.Check.Required {
					suffix = " (optional)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s%s\n", status, r.Check.Name, suffix)
				if r.Error != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", r.Error)
				}
			}

			if !doctor.AllRequiredPassed(results) {
				fmt.Fprintln(cmd.OutOrStdout(), "\nSome required checks failed. Please fix them before using cage.")

				if showFix {
					fmt.Fprintln(cmd.OutOrStdout(), "\n=== Fix Instructions ===")
					for _, r := range failedChecks {
						if r.Check.FixHint != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n  %s\n", r.Check.Name, r.Check.FixHint)
						}
					}
					fmt.Fprintln(cmd.OutOrStdout(), "\n=== Or install everything at once ===")
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", doctor.InstallAllHint())
					fmt.Fprintln(cmd.OutOrStdout(), "\nNote: You may need to log out and back in for group changes to take effect.")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "\nRun 'cage doctor --fix' to see installation instructions.")
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "\n✓ All checks passed. Cage is ready to use.")
			}
		},
	}

	cmd.Flags().BoolVar(&showFix, "fix", false, "Show installation commands for missing dependencies")

	return cmd
}
