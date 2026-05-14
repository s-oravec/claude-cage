package ssh

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/logging"
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

// SSHOptions tunes optional behaviour of an SSH invocation. Zero-value is
// safe (non-interactive, no agent forwarding).
type SSHOptions struct {
	Interactive  bool
	ForwardAgent bool // adds `-A` — only meaningful with Interactive
}

// SSHExec executes SSH with the given parameters (default port 22)
func SSHExec(cageName, ip, command string, interactive bool) error {
	return SSHExecWithPort(cageName, ip, 22, command, interactive)
}

// SSHExecWithPort executes SSH with explicit port (no agent forwarding).
func SSHExecWithPort(cageName, host string, port int, command string, interactive bool) error {
	return SSHExecWithOpts(cageName, host, port, command, SSHOptions{Interactive: interactive})
}

// SSHExecWithOpts is the full-options variant of SSHExecWithPort.
func SSHExecWithOpts(cageName, host string, port int, command string, opts SSHOptions) error {
	keyPath := KeyPath(cageName)
	knownHostsPath := KnownHostsPath()

	// Use StrictHostKeyChecking=no for localhost connections (user-mode networking)
	// because VM restarts may regenerate host keys. This is safe because:
	// 1. We're connecting to VMs we created using our own keypair
	// 2. The connection is through localhost port forwarding
	// For non-localhost, use accept-new for better security
	strictHostKey := "accept-new"
	if host == "127.0.0.1" || host == "localhost" {
		strictHostKey = "no"
	}

	args := []string{
		"-i", keyPath,
		"-o", fmt.Sprintf("StrictHostKeyChecking=%s", strictHostKey),
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", fmt.Sprintf("LogLevel=%s", logging.SSHLogLevel()),
		"-o", "ConnectTimeout=5",
		"-p", fmt.Sprintf("%d", port),
	}

	if !opts.Interactive {
		// Disable PTY allocation and redirect stdin from /dev/null so SSH
		// can never modify the parent terminal's tty modes. Without this,
		// a timing-out SSH probe leaves the controlling tty in raw mode
		// (no ONLCR), making subsequent newlines render without a CR.
		args = append(args, "-T", "-n", "-o", "BatchMode=yes")
	}

	if opts.ForwardAgent {
		args = append(args, "-A")
	}

	args = append(args, fmt.Sprintf("cage@%s", host))

	if command != "" {
		args = append(args, command)
	}

	cmd := exec.Command("ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if opts.Interactive {
		cmd.Stdin = os.Stdin
	}

	return cmd.Run()
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

// WaitForSSHWithPort waits for SSH to become available on specific port
func WaitForSSHWithPort(cageName, host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		err := SSHExecWithPort(cageName, host, port, "true", false)
		if err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return ErrSSHTimeout
}

// ExecCapture executes a command via SSH and captures the output
func ExecCapture(cageName, ip, command string) (string, error) {
	return ExecCaptureWithPort(cageName, ip, 22, command)
}

// ExecCaptureWithPort executes a command via SSH with explicit port and captures the output
func ExecCaptureWithPort(cageName, host string, port int, command string) (string, error) {
	return ExecCaptureWithOpts(cageName, host, port, command, SSHOptions{})
}

// ExecCaptureWithOpts is the full-options variant; only ForwardAgent is
// honoured (Interactive is implicitly false — capture mode never has a tty).
func ExecCaptureWithOpts(cageName, host string, port int, command string, opts SSHOptions) (string, error) {
	keyPath := KeyPath(cageName)
	knownHostsPath := KnownHostsPath()

	// Use StrictHostKeyChecking=no for localhost connections (user-mode networking)
	// because VM restarts may regenerate host keys
	strictHostKey := "accept-new"
	if host == "127.0.0.1" || host == "localhost" {
		strictHostKey = "no"
	}

	args := []string{
		"-T", // no PTY — never touch the parent tty modes
		"-n", // stdin from /dev/null
		"-i", keyPath,
		"-o", fmt.Sprintf("StrictHostKeyChecking=%s", strictHostKey),
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", fmt.Sprintf("LogLevel=%s", logging.SSHLogLevel()),
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		"-p", fmt.Sprintf("%d", port),
	}

	if opts.ForwardAgent {
		args = append(args, "-A")
	}

	args = append(args,
		fmt.Sprintf("cage@%s", host),
		command,
	)

	cmd := exec.Command("ssh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}
