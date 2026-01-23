package cage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/stiivo/cage/internal/config"
)

const (
	StatusRunning = "running"
	StatusStopped = "stopped"
)

// State holds the runtime state of a cage
type State struct {
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Image        string    `json:"image"`
	Profile      string    `json:"profile"`
	IP           string    `json:"ip,omitempty"`
	Ports        []Port    `json:"ports,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	VirtiofsPID  int       `json:"virtiofs_pid,omitempty"`
	ForwarderPID int       `json:"forwarder_pid,omitempty"`
}

// Port represents a port forwarding rule
type Port struct {
	Host         int    `json:"host"`
	Guest        int    `json:"guest"`
	Protocol     string `json:"protocol"`
	Bind         string `json:"bind,omitempty"`
	ForwarderPID int    `json:"forwarder_pid,omitempty"`
}

// cagesDir can be overridden in tests
var cagesDir string

// CagesDir returns the cages directory
func CagesDir() string {
	if cagesDir != "" {
		return cagesDir
	}
	return filepath.Join(config.Dir(), "cages")
}

// SetCagesDir sets the cages directory (for testing)
func SetCagesDir(dir string) string {
	old := cagesDir
	cagesDir = dir
	return old
}

// Dir returns the directory for a specific cage
func Dir(name string) string {
	return filepath.Join(CagesDir(), name)
}

// StatePath returns the path to a cage's state file
func StatePath(name string) string {
	return filepath.Join(Dir(name), "state.json")
}

// EnsureDir creates the cage directory if it doesn't exist
func EnsureDir(name string) error {
	return os.MkdirAll(Dir(name), 0755)
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

// DeleteState removes a cage's state and directory
func DeleteState(name string) error {
	return os.RemoveAll(Dir(name))
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
