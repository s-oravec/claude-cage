package virtiofs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// socketBaseDir can be overridden in tests
var socketBaseDir = "/tmp/cage-virtiofs"

// Common locations for virtiofsd binary
var virtiofsdPaths = []string{
	"virtiofsd",               // In PATH
	"/usr/lib/qemu/virtiofsd", // Ubuntu/Debian
	"/usr/libexec/virtiofsd",  // Fedora/RHEL
}

// FindVirtiofsd returns the path to virtiofsd or empty string if not found
func FindVirtiofsd() string {
	for _, path := range virtiofsdPaths {
		if path == "virtiofsd" {
			if p, err := exec.LookPath(path); err == nil {
				return p
			}
		} else {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

// DaemonConfig holds configuration for virtiofsd
type DaemonConfig struct {
	CageName  string
	SharedDir string
	Sandbox   bool // use --sandbox=chroot
	Seccomp   bool // use --seccomp=kill
}

// Daemon represents a running virtiofsd process
type Daemon struct {
	CageName   string
	SocketPath string
	SharedDir  string
	PID        int
	cmd        *exec.Cmd
}

// ExpandPath expands ~ to home directory
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// SocketDir returns the socket directory for a cage
func SocketDir(cageName string) string {
	return filepath.Join(socketBaseDir, cageName)
}

// SocketPath returns the socket path for a cage
func SocketPath(cageName string) string {
	return filepath.Join(SocketDir(cageName), "virtiofs.sock")
}

// EnsureSocketDir creates the socket directory
func EnsureSocketDir(cageName string) error {
	return os.MkdirAll(SocketDir(cageName), 0755)
}

// CleanupSocket removes the socket directory
func CleanupSocket(cageName string) error {
	return os.RemoveAll(SocketDir(cageName))
}

// ValidateShareDir validates a directory for sharing
func ValidateShareDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

// BuildArgs builds virtiofsd command line arguments
func BuildArgs(cfg *DaemonConfig) []string {
	socketPath := SocketPath(cfg.CageName)

	args := []string{
		"--socket-path=" + socketPath,
		"--shared-dir=" + cfg.SharedDir,
		"--cache=auto",
	}

	if cfg.Sandbox {
		args = append(args, "--sandbox=chroot")
	}

	if cfg.Seccomp {
		args = append(args, "--seccomp=kill")
	}

	return args
}

// Start starts a virtiofsd daemon for a cage
func Start(cfg *DaemonConfig) (*Daemon, error) {
	// Expand and validate shared directory
	sharedDir := ExpandPath(cfg.SharedDir)
	if err := ValidateShareDir(sharedDir); err != nil {
		return nil, fmt.Errorf("invalid share directory: %w", err)
	}

	// Create socket directory
	if err := EnsureSocketDir(cfg.CageName); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Update config with expanded path
	cfg.SharedDir = sharedDir

	// Build arguments
	args := BuildArgs(cfg)

	// Check if virtiofsd is available
	virtiofsdPath := FindVirtiofsd()
	if virtiofsdPath == "" {
		return nil, errors.New("virtiofsd not found (checked PATH, /usr/lib/qemu/, /usr/libexec/)")
	}

	// Start virtiofsd
	cmd := exec.Command(virtiofsdPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start virtiofsd: %w", err)
	}

	return &Daemon{
		CageName:   cfg.CageName,
		SocketPath: SocketPath(cfg.CageName),
		SharedDir:  sharedDir,
		PID:        cmd.Process.Pid,
		cmd:        cmd,
	}, nil
}

// Stop stops the virtiofsd daemon
func (d *Daemon) Stop() error {
	if d.PID > 0 {
		// Send SIGTERM
		if err := syscall.Kill(d.PID, syscall.SIGTERM); err != nil {
			// Process might already be dead
			if !errors.Is(err, syscall.ESRCH) {
				return err
			}
		}
	}

	// Cleanup socket
	CleanupSocket(d.CageName)

	return nil
}

// StopByPID stops a virtiofsd by PID and cleans up socket
func StopByPID(cageName string, pid int) error {
	if pid > 0 {
		syscall.Kill(pid, syscall.SIGTERM)
	}
	return CleanupSocket(cageName)
}
