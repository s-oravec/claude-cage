package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/s-oravec/cage/internal/cage"
	"github.com/s-oravec/cage/internal/logging"
	"github.com/s-oravec/cage/internal/ssh"
	"github.com/spf13/cobra"
)

// NewExecCmd creates the exec command
func NewExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <name> -- <command> [args...]",
		Short: "Execute a command in a cage without TTY",
		Long: `Execute a command in a running cage without TTY allocation.

Useful for scripting and automation. Output is returned directly.
Use 'cage ssh' for interactive sessions.`,
		Args:               cobra.MinimumNArgs(1),
		ValidArgsFunction:  completeCageNames(true),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Find the -- separator
			name := args[0]

			// Check if we have a command
			if len(args) < 2 {
				return fmt.Errorf("command required: cage exec <name> -- <command>")
			}

			// Join remaining args as the command
			// The -- is stripped by cobra, so args[1:] is the command
			command := args[1:]
			if len(command) == 0 {
				return fmt.Errorf("command required: cage exec <name> -- <command>")
			}

			return execInCage(cmd, name, command)
		},
	}

	return cmd
}

func execInCage(cmd *cobra.Command, name string, command []string) error {
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

	// Determine SSH target
	var host string
	var port int

	if state.SSHPort > 0 {
		// User-mode networking with port forwarding
		host = "127.0.0.1"
		port = state.SSHPort
	} else if state.IP != "" {
		// Bridge networking with direct IP
		host = state.IP
		port = 22
	} else {
		return fmt.Errorf("cage '%s' has no SSH access configured (use --ssh when creating or use bridge network)", name)
	}

	// Check SSH key exists
	if !ssh.KeyExists(name) {
		return fmt.Errorf("SSH key not found for cage '%s'", name)
	}

	// Build SSH command without TTY
	keyPath := ssh.KeyPath(name)
	knownHostsPath := ssh.KnownHostsPath()

	sshArgs := []string{
		"-T", // no TTY
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", fmt.Sprintf("LogLevel=%s", logging.SSHLogLevel()),
		"-o", "ConnectTimeout=10",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("cage@%s", host),
	}

	// Add the command
	sshArgs = append(sshArgs, strings.Join(command, " "))

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdin = os.Stdin

	return sshCmd.Run()
}
