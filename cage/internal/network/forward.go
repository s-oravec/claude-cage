package network

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/stiivo/cage/internal/ssh"
)

var (
	ErrForwarderNotRunning = errors.New("forwarder is not running")
	ErrNoPortsToForward    = errors.New("no ports to forward")
)

// Forwarder manages SSH-based port forwarding for a cage
type Forwarder struct {
	CageName string
	Forwards []PortForward
	IP       string
	Process  *os.Process
	PID      int
}

// StartForwarding starts SSH-based port forwarding for a cage
func StartForwarding(cageName, ip string, forwards []PortForward) (*Forwarder, error) {
	if len(forwards) == 0 {
		return nil, ErrNoPortsToForward
	}

	if ip == "" {
		return nil, errors.New("cage IP address required")
	}

	keyPath := ssh.KeyPath(cageName)
	knownHostsPath := ssh.KnownHostsPath()

	// Build SSH command with port forwarding
	args := []string{
		"-N", // no command
		"-T", // no TTY
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "LogLevel=ERROR",
	}

	// Add local port forwards
	for _, fwd := range forwards {
		args = append(args, "-L",
			fmt.Sprintf("%s:%d:%s:%d",
				fwd.Bind, fwd.HostPort,
				ip, fwd.GuestPort))
	}

	args = append(args, fmt.Sprintf("cage@%s", ip))

	cmd := exec.Command("ssh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port forwarding: %w", err)
	}

	return &Forwarder{
		CageName: cageName,
		Forwards: forwards,
		IP:       ip,
		Process:  cmd.Process,
		PID:      cmd.Process.Pid,
	}, nil
}

// Stop terminates the port forwarder
func (f *Forwarder) Stop() error {
	if f.Process == nil {
		return ErrForwarderNotRunning
	}

	// Try graceful termination first
	if err := f.Process.Signal(syscall.SIGTERM); err != nil {
		// Process may have already exited
		return nil
	}

	// Wait for process to exit
	f.Process.Wait()
	return nil
}

// StopForwarderByPID stops a forwarder by its PID
func StopForwarderByPID(pid int) error {
	if pid <= 0 {
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil // Process not found, probably already exited
	}

	// Try graceful termination
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return nil // Process may have already exited
	}

	// Don't wait - just signal and continue
	return nil
}

// IsForwarderRunning checks if a forwarder process is still running
func IsForwarderRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// AddSingleForward adds a single port forward to a running cage
// This starts a new SSH process for the single forward
func AddSingleForward(cageName, ip string, fwd PortForward) (*Forwarder, error) {
	return StartForwarding(cageName, ip, []PortForward{fwd})
}
