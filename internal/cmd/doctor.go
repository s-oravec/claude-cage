package cmd

import (
	"fmt"

	"github.com/s-oravec/claude-cage/internal/doctor"
	"github.com/spf13/cobra"
)

// NewDoctorCmd creates the doctor command
func NewDoctorCmd() *cobra.Command {
	var applyFix bool
	var rootMode bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check system requirements",
		Long: `Check if all system requirements for running cage are met.

By default checks prerequisites for user mode (running as a regular user
with libvirt session). Use --root to additionally check root-mode (sudo
cage) prerequisites: libvirt system mode connectivity, home-dir
traversability by libvirt-qemu, and virtiofsd.

Use --fix to apply auto-fixable issues (those not requiring sudo) and
print installation commands for the remaining problems.`,
		Run: func(cmd *cobra.Command, args []string) {
			runDoctor(cmd, applyFix, rootMode)
		},
	}

	cmd.Flags().BoolVar(&applyFix, "fix", false, "Apply auto-fixable issues and show install commands for the rest")
	cmd.Flags().BoolVar(&rootMode, "root", false, "Also check root-mode (sudo cage) prerequisites")

	return cmd
}

func runDoctor(cmd *cobra.Command, applyFix, rootMode bool) {
	var checks []doctor.Check
	if rootMode {
		checks = doctor.RootChecks()
		fmt.Fprintln(cmd.OutOrStdout(), "Checking user-mode + root-mode prerequisites:")
	} else {
		checks = doctor.DefaultChecks()
	}
	results := doctor.RunChecks(checks)
	printResults(cmd, results)

	if applyFix {
		fixes := doctor.RunFixes(results)
		if len(fixes) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\n=== Applying auto-fixes ===")
			for _, f := range fixes {
				if f.Applied {
					fmt.Fprintf(cmd.OutOrStdout(), "✓ Fixed: %s\n", f.Check.Name)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "✗ Could not fix '%s': %v\n", f.Check.Name, f.Error)
				}
			}

			// Re-run checks to show new state
			fmt.Fprintln(cmd.OutOrStdout(), "\n=== Re-checking ===")
			results = doctor.RunChecks(checks)
			printResults(cmd, results)
		}
	}

	if doctor.AllRequiredPassed(results) {
		fmt.Fprintln(cmd.OutOrStdout(), "\n✓ All checks passed. Cage is ready to use.")
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nSome required checks failed.")

	if applyFix {
		// Already tried to fix what we could; show hints for the rest.
		fmt.Fprintln(cmd.OutOrStdout(), "\n=== Manual fix instructions ===")
		for _, r := range results {
			if r.Passed || r.Check.FixHint == "" {
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n  %s\n", r.Check.Name, r.Check.FixHint)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\n=== Or install everything at once ===")
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", doctor.InstallAllHint())
		fmt.Fprintln(cmd.OutOrStdout(), "\nNote: log out and back in for group changes to take effect.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'cage doctor --fix' to auto-apply fixes and see install instructions.")
	}
}

func printResults(cmd *cobra.Command, results []doctor.CheckResult) {
	for _, r := range results {
		status := "✓"
		if !r.Passed {
			status = "✗"
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
}
