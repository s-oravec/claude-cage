package cage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCageState_Structure(t *testing.T) {
	state := State{
		Name:      "test",
		Status:    StatusRunning,
		Image:     "ubuntu-24.04",
		Profile:   "default",
		IP:        "192.168.122.10",
		StartedAt: time.Now(),
	}

	assert.Equal(t, "test", state.Name)
	assert.Equal(t, StatusRunning, state.Status)
	assert.Equal(t, "ubuntu-24.04", state.Image)
}

func TestCageDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	dir := Dir("myproject")
	assert.Contains(t, dir, "myproject")
}

func TestSaveAndLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	state := &State{
		Name:      "test",
		Status:    StatusRunning,
		Image:     "ubuntu-24.04",
		Profile:   "default",
		StartedAt: time.Now().Truncate(time.Second),
	}

	err := SaveState(state)
	require.NoError(t, err)

	loaded, err := LoadState("test")
	require.NoError(t, err)

	assert.Equal(t, state.Name, loaded.Name)
	assert.Equal(t, state.Status, loaded.Status)
	assert.Equal(t, state.Image, loaded.Image)
}

func TestLoadState_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	_, err := LoadState("nonexistent")
	assert.Error(t, err)
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	assert.False(t, Exists("test"))

	// Create state
	SaveState(&State{Name: "test", Status: StatusRunning})

	assert.True(t, Exists("test"))
}

func TestListCages(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	// Empty initially
	list, err := List()
	require.NoError(t, err)
	assert.Empty(t, list)

	// Add some cages
	SaveState(&State{Name: "cage1", Status: StatusRunning})
	SaveState(&State{Name: "cage2", Status: StatusStopped})

	list, err = List()
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestDeleteState(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	SaveState(&State{Name: "test", Status: StatusRunning})
	assert.True(t, Exists("test"))

	err := DeleteState("test")
	require.NoError(t, err)
	assert.False(t, Exists("test"))

	// Directory should be removed
	_, err = os.Stat(filepath.Join(tmpDir, "test"))
	assert.True(t, os.IsNotExist(err))
}

func TestSaveAndLoadRestartConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	cfg := &RestartConfig{
		Image:   "ubuntu-24.04",
		Profile: "heavy",
		Ports: []Port{
			{Host: 8080, Guest: 80, Protocol: "tcp"},
		},
	}

	err := SaveRestartConfig("test", cfg)
	require.NoError(t, err)

	loaded, err := LoadRestartConfig("test")
	require.NoError(t, err)

	assert.Equal(t, cfg.Image, loaded.Image)
	assert.Equal(t, cfg.Profile, loaded.Profile)
	assert.Len(t, loaded.Ports, 1)
	assert.Equal(t, 8080, loaded.Ports[0].Host)
}

func TestLoadRestartConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	_, err := LoadRestartConfig("nonexistent")
	assert.Error(t, err)
}

func TestDeleteRestartConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	cfg := &RestartConfig{Image: "test"}
	SaveRestartConfig("test", cfg)

	// Verify it exists
	_, err := LoadRestartConfig("test")
	require.NoError(t, err)

	// Delete it
	err = DeleteRestartConfig("test")
	require.NoError(t, err)

	// Should no longer exist
	_, err = LoadRestartConfig("test")
	assert.Error(t, err)
}

func TestRestartConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	path := RestartConfigPath("myproject")
	assert.Contains(t, path, "myproject")
	assert.Contains(t, path, "restart.json")
}

func TestStatePath(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	path := StatePath("myproject")
	assert.Contains(t, path, "myproject")
	assert.Contains(t, path, "state.json")
}

func TestPort_Structure(t *testing.T) {
	port := Port{
		Host:         8080,
		Guest:        80,
		Protocol:     "tcp",
		Bind:         "127.0.0.1",
		ForwarderPID: 12345,
	}

	assert.Equal(t, 8080, port.Host)
	assert.Equal(t, 80, port.Guest)
	assert.Equal(t, "tcp", port.Protocol)
	assert.Equal(t, "127.0.0.1", port.Bind)
	assert.Equal(t, 12345, port.ForwarderPID)
}

func TestState_WithPorts(t *testing.T) {
	state := State{
		Name:   "test",
		Status: StatusRunning,
		Ports: []Port{
			{Host: 8080, Guest: 80, Protocol: "tcp"},
			{Host: 3000, Guest: 3000, Protocol: "tcp"},
		},
	}

	assert.Len(t, state.Ports, 2)
	assert.Equal(t, 8080, state.Ports[0].Host)
	assert.Equal(t, 3000, state.Ports[1].Host)
}

func TestState_WithVirtiofsPID(t *testing.T) {
	state := State{
		Name:        "test",
		Status:      StatusRunning,
		VirtiofsPID: 54321,
	}

	assert.Equal(t, 54321, state.VirtiofsPID)
}

func TestSetCagesDir(t *testing.T) {
	// Save original
	original := cagesDir

	// Set new dir
	old := SetCagesDir("/custom/path")
	assert.Equal(t, original, old)
	assert.Equal(t, "/custom/path", CagesDir())

	// Restore
	SetCagesDir(original)
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	err := EnsureDir("newcage")
	require.NoError(t, err)

	// Directory should exist
	info, err := os.Stat(filepath.Join(tmpDir, "newcage"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestList_WithInvalidState(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := cagesDir
	cagesDir = tmpDir
	defer func() { cagesDir = oldDir }()

	// Create a valid state
	SaveState(&State{Name: "valid", Status: StatusRunning})

	// Create an invalid state (corrupt JSON)
	invalidDir := filepath.Join(tmpDir, "invalid")
	os.MkdirAll(invalidDir, 0755)
	os.WriteFile(filepath.Join(invalidDir, "state.json"), []byte("not json"), 0644)

	// List should still work, just skipping invalid states
	list, err := List()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "valid", list[0].Name)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "running", StatusRunning)
	assert.Equal(t, "stopped", StatusStopped)
}
