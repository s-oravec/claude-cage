package images

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImagesDir(t *testing.T) {
	dir := Dir()
	assert.Contains(t, dir, ".claude-cage")
	assert.Contains(t, dir, "images")
}

func TestImagePath(t *testing.T) {
	path := ImagePath("ubuntu-24.04")
	assert.Contains(t, path, "ubuntu-24.04.qcow2")
}

func TestIsDownloaded_False(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	assert.False(t, IsDownloaded("ubuntu-24.04"))
}

func TestIsDownloaded_True(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// Create fake image file
	os.WriteFile(filepath.Join(tmpDir, "ubuntu-24.04.qcow2"), []byte("fake"), 0644)

	assert.True(t, IsDownloaded("ubuntu-24.04"))
}

func TestListDownloaded_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	list := ListDownloaded()
	assert.Empty(t, list)
}

func TestListDownloaded_WithImages(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// Create fake image files
	os.WriteFile(filepath.Join(tmpDir, "ubuntu-24.04.qcow2"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "debian-12.qcow2"), []byte("fake"), 0644)

	list := ListDownloaded()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "ubuntu-24.04")
	assert.Contains(t, list, "debian-12")
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = filepath.Join(tmpDir, "images")
	defer func() { imagesDir = oldDir }()

	err := EnsureDir()
	require.NoError(t, err)

	info, err := os.Stat(imagesDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
