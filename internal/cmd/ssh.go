package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/ssh"
)

// NewSSHCmd creates the ssh command
func NewSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [name] [command]",
		Short: "SSH into a running cage",
		Long: `Connect to a running cage via SSH.

Without a command, opens an interactive shell.
With a command, executes it and returns.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			command := ""
			if len(args) > 1 {
				command = strings.Join(args[1:], " ")
			}
			return sshToCage(cmd, name, command)
		},
	}

	return cmd
}

func sshToCage(cmd *cobra.Command, name, command string) error {
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

	// Connect
	return ssh.SSHExec(name, state.IP, command, command == "")
}
