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

func TestSetDir(t *testing.T) {
	oldDir := imagesDir
	defer func() { imagesDir = oldDir }()

	SetDir("/custom/images/path")
	assert.Equal(t, "/custom/images/path", Dir())

	SetDir("")
	// When empty, Dir() should return config default
	assert.Contains(t, Dir(), "images")
}

func TestProgressWriter(t *testing.T) {
	var lastWritten, lastTotal int64
	progressFn := func(written, total int64) {
		lastWritten = written
		lastTotal = total
	}

	pw := &ProgressWriter{
		Total:      100,
		OnProgress: progressFn,
	}

	n, err := pw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, int64(5), pw.Written)
	assert.Equal(t, int64(5), lastWritten)
	assert.Equal(t, int64(100), lastTotal)

	n, err = pw.Write([]byte("world"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, int64(10), pw.Written)
	assert.Equal(t, int64(10), lastWritten)
}

func TestProgressWriter_NoCallback(t *testing.T) {
	pw := &ProgressWriter{
		Total:      100,
		OnProgress: nil,
	}

	n, err := pw.Write([]byte("test"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, int64(4), pw.Written)
}

func TestImagePath_WithAlias(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// "alpine" alias should resolve to "alpine-3.21"
	path := ImagePath("alpine")
	assert.Contains(t, path, "alpine-3.21.qcow2")
}

func TestIsDownloaded_WithAlias(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// Create file with resolved name
	os.WriteFile(filepath.Join(tmpDir, "alpine-3.21.qcow2"), []byte("fake"), 0644)

	// Should find it via alias
	assert.True(t, IsDownloaded("alpine"))
}

func TestDownload_UnknownImage(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	err := Download("nonexistent-image-xyz", HostArchitecture(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown image")
}

func TestListDownloaded_NonExistentDir(t *testing.T) {
	oldDir := imagesDir
	imagesDir = "/nonexistent/path/12345"
	defer func() { imagesDir = oldDir }()

	// Should not panic, just return empty list
	list := ListDownloaded()
	assert.Empty(t, list)
}

func TestListDownloaded_IgnoresNonQcow2(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// Create qcow2 and non-qcow2 files
	os.WriteFile(filepath.Join(tmpDir, "ubuntu.qcow2"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "image.raw"), []byte("raw"), 0644)

	list := ListDownloaded()
	assert.Len(t, list, 1)
	assert.Contains(t, list, "ubuntu")
}
