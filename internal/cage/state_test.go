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
