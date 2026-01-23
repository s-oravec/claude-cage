package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/doctor"
)

// NewDoctorCmd creates the doctor command
func NewDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check system requirements",
		Run: func(cmd *cobra.Command, args []string) {
			checks := doctor.DefaultChecks()
			results := doctor.RunChecks(checks)

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

			if !doctor.AllRequiredPassed(results) {
				fmt.Fprintln(cmd.OutOrStdout(), "\nSome required checks failed. Please fix them before using cage.")
			}
		},
	}
}
