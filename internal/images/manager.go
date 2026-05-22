package images

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/s-oravec/cage/internal/config"
)

// imagesDir can be overridden in tests
var imagesDir string

// Dir returns the base-images cache directory.
//
// Lives next to the VM artifacts: in user mode that's ~/.claude-cage/images/,
// in root mode /var/lib/libvirt/images/cage/images/. Disk overlays back to
// files here, so qemu (running as libvirt-qemu under sudo) needs the
// directory to be on the default virt-aa-helper apparmor allow-list.
func Dir() string {
	if imagesDir != "" {
		return imagesDir
	}
	return filepath.Join(config.VMArtifactsDir(), "images")
}

// SetDir sets the images directory (for testing)
func SetDir(dir string) {
	imagesDir = dir
}

// ImagePath returns the full path to an image file (supports aliases)
func ImagePath(name string) string {
	name = ResolveAlias(name)
	return filepath.Join(Dir(), name+".qcow2")
}

// EnsureDir creates the images directory if it doesn't exist
func EnsureDir() error {
	return os.MkdirAll(Dir(), 0755)
}

// IsDownloaded checks if an image is already downloaded (supports aliases)
func IsDownloaded(name string) bool {
	name = ResolveAlias(name)
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

// Download downloads and prepares a base image for the given architecture.
// On success it writes base metadata recording the downloaded arch (the cache
// itself stays flat: images/<name>.qcow2).
func Download(name, arch string, progress func(written, total int64)) error {
	src, err := GetSource(name, arch)
	if err != nil {
		return err
	}
	if src.URL == "" {
		return fmt.Errorf("base %q has no %s cloud-image; pick a different base or architecture", name, arch)
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

	// Record base metadata so BaseArch can report the downloaded arch.
	var size int64
	if fi, err := os.Stat(finalPath); err == nil {
		size = fi.Size()
	}
	if err := SaveMetadata(&Image{
		Name:        ResolveAlias(name),
		Type:        "base",
		Arch:        arch,
		Description: src.Description,
		Path:        finalPath,
		Size:        size,
		CreatedAt:   time.Now(),
	}); err != nil {
		// Keep Download all-or-nothing: without metadata IsDownloaded() would
		// report true while BaseArch falls back to the host arch. Remove the
		// image so the next run re-downloads cleanly.
		os.Remove(finalPath)
		return fmt.Errorf("failed to record image metadata: %w", err)
	}

	return nil
}

// Setup downloads and prepares a base image with customization
func Setup(name, arch string, progress func(written, total int64), status func(msg string)) error {
	if err := Download(name, arch, progress); err != nil {
		return err
	}

	// TODO: Add image customization (Docker, SSH) in future iteration
	// For now, we rely on cloud-init to configure the VM at boot

	if status != nil {
		status(fmt.Sprintf("✓ Image ready: %s", name))
	}

	return nil
}
