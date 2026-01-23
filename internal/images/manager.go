package images

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/s-oravec/claude-cage/internal/config"
)

// imagesDir can be overridden in tests
var imagesDir string

// Dir returns the images directory path
func Dir() string {
	if imagesDir != "" {
		return imagesDir
	}
	return filepath.Join(config.Dir(), "images")
}

// SetDir sets the images directory (for testing)
func SetDir(dir string) {
	imagesDir = dir
}

// ImagePath returns the full path to an image file
func ImagePath(name string) string {
	return filepath.Join(Dir(), name+".qcow2")
}

// EnsureDir creates the images directory if it doesn't exist
func EnsureDir() error {
	return os.MkdirAll(Dir(), 0755)
}

// IsDownloaded checks if an image is already downloaded
func IsDownloaded(name string) bool {
	_, err := os.Stat(ImagePath(name))
	return err == nil
}

// ListDownloaded returns names of downloaded images
func ListDownloaded() []string {
	var names []string
	files, err := filepath.Glob(filepath.Join(Dir(), "*.qcow2"))
	if err != nil {
		return names
	}
	for _, f := range files {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, ".qcow2")
		names = append(names, name)
	}
	return names
}

// ProgressWriter wraps an io.Writer to track progress
type ProgressWriter struct {
	Total      int64
	Written    int64
	OnProgress func(written, total int64)
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Written += int64(n)
	if pw.OnProgress != nil {
		pw.OnProgress(pw.Written, pw.Total)
	}
	return n, nil
}

// Download downloads and prepares a base image
func Download(name string, progress func(written, total int64)) error {
	src, err := GetSource(name)
	if err != nil {
		return err
	}

	if err := EnsureDir(); err != nil {
		return fmt.Errorf("failed to create images directory: %w", err)
	}

	// Download to temp file first
	tmpPath := ImagePath(name) + ".tmp"
	defer os.Remove(tmpPath)

	resp, err := http.Get(src.URL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	pw := &ProgressWriter{
		Total:      resp.ContentLength,
		OnProgress: progress,
	}

	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	out.Close()
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Convert to qcow2 if needed
	finalPath := ImagePath(name)
	if strings.HasSuffix(src.URL, ".qcow2") {
		// Already qcow2, just rename
		if err := os.Rename(tmpPath, finalPath); err != nil {
			return fmt.Errorf("failed to save image: %w", err)
		}
	} else {
		// Convert from raw/other format
		cmd := exec.Command("qemu-img", "convert", "-O", "qcow2", tmpPath, finalPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("qcow2 conversion failed: %s", string(out))
		}
	}

	return nil
}

// Setup downloads and prepares a base image with customization
func Setup(name string, progress func(written, total int64), status func(msg string)) error {
	if status != nil {
		status(fmt.Sprintf("Downloading %s...", name))
	}

	if err := Download(name, progress); err != nil {
		return err
	}

	// TODO: Add image customization (Docker, SSH) in future iteration
	// For now, we rely on cloud-init to configure the VM at boot

	if status != nil {
		status(fmt.Sprintf("✓ Base image ready: %s", name))
	}

	return nil
}
