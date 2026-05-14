package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/ssh"
	"github.com/spf13/cobra"
)

// NewSSHCmd creates the ssh command
func NewSSHCmd() *cobra.Command {
	var forwardAgent bool

	cmd := &cobra.Command{
		Use:   "ssh [name] [command...]",
		Short: "SSH into a running cage",
		Long: `Connect to a running cage via SSH.

Without a command, opens an interactive shell.
With a command, executes it and returns.

When run from a directory with .cage.yml, the cage name is optional.
In that case, all arguments are treated as the command to execute.

Use -A to forward your local ssh-agent into the cage so git/ssh inside
the cage can authenticate against private hosts (Gitea, GitHub, …) with
your existing keys, without ever copying them in.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, command, err := resolveSSHArgs(args)
			if err != nil {
				return err
			}
			return sshToCage(cmd, name, command, forwardAgent)
		},
	}

	cmd.Flags().BoolVarP(&forwardAgent, "forward-agent", "A", false,
		"Forward the local ssh-agent into the cage (for git clone / ssh to upstream hosts from inside)")

	return cmd
}

// resolveSSHArgs determines cage name and command from arguments.
// Logic:
// - If args[0] exists as a cage, treat it as cage name and rest as command
// - If project config exists and args[0] is not a cage, use project cage and all args as command
// - If no project config and args[0] is not a cage, return error
func resolveSSHArgs(args []string) (name string, command string, err error) {
	// No args: try to get cage from project config
	if len(args) == 0 {
		name, _, err = resolveCageName(args)
		return name, "", err
	}

	// Check if first arg is an existing cage
	if cage.Exists(args[0]) {
		name = args[0]
		if len(args) > 1 {
			command = strings.Join(args[1:], " ")
		}
		return name, command, nil
	}

	// First arg is not a cage - check for project config
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current directory: %w", err)
	}

	if config.ProjectConfigExists(cwd) {
		cfg, err := config.LoadProjectConfig(cwd)
		if err != nil {
			return "", "", err
		}
		// Use project cage, all args are command
		return cfg.Cage, strings.Join(args, " "), nil
	}

	// No project config and first arg is not a cage
	return "", "", fmt.Errorf("cage '%s' not found", args[0])
}

func sshToCage(cmd *cobra.Command, name, command string, forwardAgent bool) error {
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

	// Check SSH key exists
	if !ssh.KeyExists(name) {
		return fmt.Errorf("SSH key not found for cage '%s'", name)
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

	// Connect
	return ssh.SSHExecWithOpts(name, host, port, command, ssh.SSHOptions{
		Interactive:  command == "",
		ForwardAgent: forwardAgent,
	})
}
