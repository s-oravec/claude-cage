package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/ssh"
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

	fmt.Fprintf(cmd.OutOrStdout(), "=== Network Isolation Verification: %s ===\n\n", name)

	var results []network.VerificationResult
	allPassed := true

	// Check for host-level isolation (network namespace)
	if state.IsolationNS != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Host-level isolation: ENABLED (namespace: %s)\n\n", state.IsolationNS)

		// Run namespace verification
		nsResults, err := network.VerifyNamespaceIsolation(state.IsolationNS)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: namespace verification failed: %v\n", err)
		} else {
			results = append(results, nsResults...)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Host-level isolation: DISABLED (run as root for full isolation)")
		fmt.Fprintln(cmd.OutOrStdout(), "Falling back to in-VM route verification...")
	}

	// Run VM-level verification if we have an IP
	if state.IP != "" {
		keyPath := ssh.KeyPath(name)
		if ssh.KeyExists(name) {
			vmResults, err := network.VerifyIsolation(name, state.IP, keyPath)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: VM verification failed: %v\n", err)
			} else {
				results = append(results, vmResults...)
			}
		}
	} else if state.SSHPort > 0 {
		// For SLIRP mode, try SSH via localhost port
		keyPath := ssh.KeyPath(name)
		if ssh.KeyExists(name) {
			vmResults, err := network.VerifyIsolationWithPort(name, "127.0.0.1", state.SSHPort, keyPath)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: VM verification failed: %v\n", err)
			} else {
				results = append(results, vmResults...)
			}
		}
	}

	// Print results
	fmt.Fprintln(cmd.OutOrStdout(), "=== Test Results ===")
	for _, r := range results {
		status := "✓"
		if !r.Passed {
			status = "✗"
			allPassed = false
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", status, r.TestName, r.Message)
	}

	fmt.Fprintln(cmd.OutOrStdout())

	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tests could be run. Make sure the cage is running.")
		return fmt.Errorf("verification incomplete")
	}

	if allPassed {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ All tests passed - network isolation is working correctly")
		return nil
	}

	return fmt.Errorf("SECURITY WARNING: Some isolation tests failed!")
}
