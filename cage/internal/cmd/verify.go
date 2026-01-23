package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stiivo/cage/internal/cage"
	"github.com/stiivo/cage/internal/network"
	"github.com/stiivo/cage/internal/ssh"
)

// NewVerifyCmd creates the verify command
func NewVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [name]",
		Short: "Verify network isolation for a cage",
		Long: `Run network isolation tests on a running cage.

Tests include:
- Internet access (should work)
- DNS resolution (should work)
- RFC 1918 subnets (should be blocked)
- Link-local addresses (should be blocked)

This is a security verification - use after starting a cage.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return verifyCage(cmd, args[0])
		},
	}

	return cmd
}

func verifyCage(cmd *cobra.Command, name string) error {
	// Check cage exists and get state
	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("cage '%s' not found or not running", name)
	}

	if state.IP == "" {
		return fmt.Errorf("cage '%s' has no IP address", name)
	}

	// Get SSH key path
	keyPath := ssh.KeyPath(name)
	if !ssh.KeyExists(name) {
		return fmt.Errorf("SSH key not found for cage '%s'", name)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "=== Network Isolation Verification: %s ===\n\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "IP: %s\n\n", state.IP)

	// Run verification tests
	results, err := network.VerifyIsolation(name, state.IP, keyPath)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Print results
	allPassed := true
	for _, r := range results {
		status := "✓"
		if !r.Passed {
			status = "✗"
			allPassed = false
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", status, r.TestName, r.Message)
	}

	fmt.Fprintln(cmd.OutOrStdout())

	if allPassed {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ All tests passed - network isolation is working correctly")
		return nil
	}

	return fmt.Errorf("SECURITY WARNING: Some isolation tests failed!")
}
