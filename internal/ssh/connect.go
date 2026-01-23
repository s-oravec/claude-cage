package ssh

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
)

var (
	ErrCageNotRunning = errors.New("cage is not running")
	ErrSSHTimeout     = errors.New("SSH connection timed out")
)

// Connect establishes an SSH connection to a cage
func Connect(cageName string, command string) error {
	state, err := cage.LoadState(cageName)
	if err != nil {
		return fmt.Errorf("cage not found: %w", err)
	}

	if state.Status != cage.StatusRunning {
		return ErrCageNotRunning
	}

	if state.IP == "" {
		return errors.New("cage has no IP address")
	}

	return SSHExec(cageName, state.IP, command, true)
}

// SSHExec executes SSH with the given parameters
func SSHExec(cageName, ip, command string, interactive bool) error {
	keyPath := KeyPath(cageName)
	knownHostsPath := KnownHostsPath()

	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
	}

	if !interactive {
		args = append(args, "-o", "BatchMode=yes")
	}

	args = append(args, fmt.Sprintf("cage@%s", ip))

	if command != "" {
		args = append(args, command)
	}

	cmd := exec.Command("ssh", args...)

	if interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}
	return nil
}

// WaitForSSH waits for SSH to become available
func WaitForSSH(cageName, ip string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		err := SSHExec(cageName, ip, "true", false)
		if err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return ErrSSHTimeout
}

// ExecCapture executes a command via SSH and captures the output
func ExecCapture(cageName, ip, command string) (string, error) {
	keyPath := KeyPath(cageName)
	knownHostsPath := KnownHostsPath()

	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		fmt.Sprintf("cage@%s", ip),
		command,
	}

	cmd := exec.Command("ssh", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
