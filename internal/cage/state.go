package cage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/s-oravec/claude-cage/internal/config"
)

const (
	StatusRunning = "running"
	StatusStopped = "stopped"
)

// Network modes
const (
	NetworkAuto   = "auto"   // SLIRP user-mode networking (default, no root)
	NetworkBridge = "bridge" // libvirt bridge with firewall (requires root)
)

// State holds the runtime state of a cage
type State struct {
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Image        string    `json:"image"`
	Profile      string    `json:"profile"`
	Mode         string    `json:"mode,omitempty"` // "user" or "root"; empty for pre-mode-split cages (treated as user)
	NetworkMode  string    `json:"network_mode,omitempty"`
	SSHPort      int       `json:"ssh_port,omitempty"`
	IP           string    `json:"ip,omitempty"`
	Ports        []Port    `json:"ports,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	VirtiofsPID  int       `json:"virtiofs_pid,omitempty"`
	ForwarderPID int       `json:"forwarder_pid,omitempty"`
	// Network isolation fields
	IsolationNS     string `json:"isolation_ns,omitempty"`        // Network namespace name
	IsolationPasst  int    `json:"isolation_passt_pid,omitempty"` // Passt process PID
	IsolationSocket string `json:"isolation_socket,omitempty"`    // Passt socket path
	IsolationIP     string `json:"isolation_ip,omitempty"`        // Namespace IP for SSH access
}

// Port represents a port forwarding rule
type Port struct {
	Host         int    `json:"host"`
	Guest        int    `json:"guest"`
	Protocol     string `json:"protocol"`
	Bind         string `json:"bind,omitempty"`
	ForwarderPID int    `json:"forwarder_pid,omitempty"`
}

// cagesDir / vmCagesDir can be overridden in tests
var (
	cagesDir   string
	vmCagesDir string
)

// CagesDir returns the metadata cages directory (state.json lives here).
func CagesDir() string {
	if cagesDir != "" {
		return cagesDir
	}
	return filepath.Join(config.Dir(), "cages")
}

// VMCagesDir returns the VM-artifacts cages directory (disk overlays,
// cloud-init ISOs, virtiofs sources). In user mode this equals CagesDir
// (so tests that override cagesDir keep working); in root mode it lives
// under /var/lib/libvirt/images/cage/cages/.
func VMCagesDir() string {
	if vmCagesDir != "" {
		return vmCagesDir
	}
	if os.Geteuid() != 0 {
		return CagesDir()
	}
	return filepath.Join(config.VMArtifactsDir(), "cages")
}

// SetCagesDir sets the cages directory (for testing)
func SetCagesDir(dir string) string {
	old := cagesDir
	cagesDir = dir
	return old
}

// SetVMCagesDir sets the VM cages directory (for testing)
func SetVMCagesDir(dir string) string {
	old := vmCagesDir
	vmCagesDir = dir
	return old
}

// Dir returns the metadata directory for a specific cage (state.json, etc).
func Dir(name string) string {
	return filepath.Join(CagesDir(), name)
}

// VMDir returns the VM-artifacts directory for a specific cage
// (disk.qcow2, cloud-init.iso, runtime/). In user mode this equals Dir;
// in root mode it diverges.
func VMDir(name string) string {
	return filepath.Join(VMCagesDir(), name)
}

// StatePath returns the path to a cage's state file
func StatePath(name string) string {
	return filepath.Join(Dir(name), "state.json")
}

// EnsureDir creates both the metadata and VM-artifacts directories for a cage.
func EnsureDir(name string) error {
	if err := os.MkdirAll(Dir(name), 0755); err != nil {
		return err
	}
	return os.MkdirAll(VMDir(name), 0755)
}

// Exists checks if a cage exists
func Exists(name string) bool {
	_, err := os.Stat(StatePath(name))
	return err == nil
}

// SaveState saves a cage's state to disk
func SaveState(state *State) error {
	if err := EnsureDir(state.Name); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(StatePath(state.Name), data, 0644)
}

// LoadState loads a cage's state from disk
func LoadState(name string) (*State, error) {
	data, err := os.ReadFile(StatePath(name))
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// RequireMode returns an error if the cage's saved mode differs from the
// current process mode. Pre-mode-split cages (state.Mode == "") are treated
// as user-mode (the historical default), so they only fail when invoked
// under sudo. Callers use this to refuse cross-mode lifecycle operations
// with a helpful hint to use (or drop) sudo.
func RequireMode(name, currentMode string) error {
	state, err := LoadState(name)
	if err != nil {
		return err
	}
	saved := state.Mode
	if saved == "" {
		saved = "user"
	}
	if saved == currentMode {
		return nil
	}
	if saved == "root" {
		return fmt.Errorf("cage '%s' was created in root mode; run 'sudo cage <op>' instead", name)
	}
	return fmt.Errorf("cage '%s' was created in user mode; run 'cage <op>' without sudo", name)
}

// DeleteState removes a cage's metadata directory and VM-artifacts directory.
// Both are removed even when they share a path (user mode); RemoveAll on a
// non-existent path is a no-op.
func DeleteState(name string) error {
	if err := os.RemoveAll(Dir(name)); err != nil {
		return err
	}
	if VMDir(name) != Dir(name) {
		if err := os.RemoveAll(VMDir(name)); err != nil {
			return err
		}
	}
	return nil
}

// RestartConfig holds configuration needed to restart a cage
type RestartConfig struct {
	Image   string `json:"image"`
	Profile string `json:"profile"`
	Ports   []Port `json:"ports,omitempty"`
}

// RestartConfigPath returns the path to a cage's restart config file
func RestartConfigPath(name string) string {
	return filepath.Join(Dir(name), "restart.json")
}

// SaveRestartConfig saves restart configuration for a cage
func SaveRestartConfig(name string, cfg *RestartConfig) error {
	if err := EnsureDir(name); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(RestartConfigPath(name), data, 0644)
}

// LoadRestartConfig loads restart configuration for a cage
func LoadRestartConfig(name string) (*RestartConfig, error) {
	data, err := os.ReadFile(RestartConfigPath(name))
	if err != nil {
		return nil, err
	}

	var cfg RestartConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// DeleteRestartConfig removes restart configuration for a cage
func DeleteRestartConfig(name string) error {
	return os.Remove(RestartConfigPath(name))
}

// List returns all cages (from state files)
func List() ([]*State, error) {
	entries, err := os.ReadDir(CagesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var states []*State
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		state, err := LoadState(entry.Name())
		if err != nil {
			continue // skip invalid states
		}
		states = append(states, state)
	}

	return states, nil
}
