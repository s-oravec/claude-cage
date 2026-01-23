package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/s-oravec/claude-cage/internal/cage"
)

// NewConsoleCmd creates the console command
func NewConsoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console [name]",
		Short: "Connect to cage serial console",
		Long: `Connect to the cage VM's serial console.

This is useful when SSH is not available (e.g., with --user-network mode).

To exit the console, press Ctrl+] (control + right bracket).

Login credentials (set by cloud-init):
  Username: cage
  Password: cage`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConsole(args[0])
		},
	}

	return cmd
}

func runConsole(name string) error {
	// Check if cage exists
	if !cage.Exists(name) {
		return fmt.Errorf("cage '%s' not found", name)
	}

	// Check if running
	state, err := cage.LoadState(name)
	if err != nil {
		return err
	}

	if state.Status != cage.StatusRunning {
		return fmt.Errorf("cage '%s' is not running", name)
	}

	domainName := "cage-" + name

	fmt.Println("Connecting to console... (exit with Ctrl+])")
	fmt.Println("Login: cage / cage")
	fmt.Println()

	// Use virsh console
	virsh := exec.Command("virsh", "-c", "qemu:///session", "console", domainName)
	virsh.Stdin = os.Stdin
	virsh.Stdout = os.Stdout
	virsh.Stderr = os.Stderr

	return virsh.Run()
}
