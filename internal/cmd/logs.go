package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/ssh"
)

// NewLogsCmd creates the logs command
func NewLogsCmd() *cobra.Command {
	var follow bool
	var lines int

	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Show logs from a cage",
		Long: `Display system logs from a running cage.

By default shows the last 100 lines. Use -f to follow (stream) logs.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return showLogs(cmd, args[0], follow, lines)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVarP(&lines, "lines", "n", 100, "Number of lines to show")

	return cmd
}

func showLogs(cmd *cobra.Command, name string, follow bool, lines int) error {
	// Check cage exists
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	// Load state
	state, err := cage.LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	if state.Status != cage.StatusRunning {
		return fmt.Errorf("cage '%s' is not running", name)
	}

	if state.IP == "" {
		return fmt.Errorf("cage '%s' has no IP address yet", name)
	}

	// Check SSH key exists
	if !ssh.KeyExists(name) {
		return fmt.Errorf("SSH key not found for cage '%s'", name)
	}

	// Build journalctl command
	journalCmd := fmt.Sprintf("journalctl -n %d", lines)
	if follow {
		journalCmd += " -f"
	}

	// Build SSH command
	keyPath := ssh.KeyPath(name)
	knownHostsPath := ssh.KnownHostsPath()

	sshArgs := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("cage@%s", state.IP),
		journalCmd,
	}

	// Use -t for follow mode to ensure proper output
	if follow {
		sshArgs = append([]string{"-t"}, sshArgs...)
	} else {
		sshArgs = append([]string{"-T"}, sshArgs...)
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if follow {
		sshCmd.Stdin = os.Stdin
	}

	return sshCmd.Run()
}
