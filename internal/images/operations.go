package images

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
)

// List returns all available images
func List() ([]Image, error) {
	var images []Image

	// List qcow2 files
	files, err := filepath.Glob(filepath.Join(Dir(), "*.qcow2"))
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".qcow2")

		// Try to load metadata
		meta, _ := LoadMetadata(name)
		if meta != nil {
			// Update path in case it changed
			meta.Path = f
			images = append(images, *meta)
		} else {
			// Base image without metadata
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			images = append(images, Image{
				Name: name,
				Type: "base",
				Size: info.Size(),
				Path: f,
			})
		}
	}

	return images, nil
}

// SaveResult contains the result of saving an image
type SaveResult struct {
	VirtCustomizeUsed  bool
	VirtCustomizeError string // Non-fatal error if virt-customize failed
}

// Save creates a new image from a stopped cage
func Save(cageName, imageName, description string) (*SaveResult, error) {
	// Check cage exists
	if !cage.Exists(cageName) {
		return nil, fmt.Errorf("cage '%s' not found", cageName)
	}

	// Load cage state
	state, err := cage.LoadState(cageName)
	if err != nil {
		return nil, fmt.Errorf("failed to load cage state: %w", err)
	}

	// Cage must be stopped to avoid corrupted disk state
	if state.Status == cage.StatusRunning {
		return nil, fmt.Errorf("cage '%s' is running. Stop it first: cage stop %s", cageName, cageName)
	}

	// Check image name not taken
	if Exists(imageName) {
		return nil, ErrImageExists
	}

	// Get source disk path
	sourceDisk := filepath.Join(cage.Dir(cageName), "disk.qcow2")
	if _, err := os.Stat(sourceDisk); err != nil {
		return nil, fmt.Errorf("cage disk not found: %w", err)
	}

	// Ensure images directory exists
	if err := EnsureDir(); err != nil {
		return nil, err
	}

	// Create destination path
	destPath := ImagePath(imageName)

	// Convert and compress the image
	cmd := exec.Command("qemu-img", "convert",
		"-O", "qcow2",
		"-c", // compress
		sourceDisk,
		destPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to save image: %s", strings.TrimSpace(string(output)))
	}

	// Prepare image for reuse: clear SSH keys and reset cloud-init
	// This uses virt-customize to modify the image while it's not running
	prepResult, err := prepareImageForReuse(destPath)
	if err != nil {
		os.Remove(destPath)
		return nil, fmt.Errorf("failed to prepare image: %w", err)
	}

	// Get size of new image
	info, err := os.Stat(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat image: %w", err)
	}

	// Save metadata
	img := &Image{
		Name:        imageName,
		Type:        "custom",
		Base:        state.Image,
		Size:        info.Size(),
		Description: description,
		CreatedAt:   time.Now(),
		Path:        destPath,
	}

	if err := SaveMetadata(img); err != nil {
		// Try to clean up the image file
		os.Remove(destPath)
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return &SaveResult{
		VirtCustomizeUsed:  prepResult.VirtCustomizeUsed,
		VirtCustomizeError: prepResult.VirtCustomizeError,
	}, nil
}

// PrepareResult indicates what preparation was done on the image
type PrepareResult struct {
	VirtCustomizeUsed  bool
	VirtCustomizeError string // Non-fatal error message if virt-customize failed
}

// prepareImageForReuse modifies a qcow2 image to prepare it for reuse
// It clears SSH authorized_keys and resets cloud-init so it re-runs on next boot
// Returns PrepareResult indicating what was done
func prepareImageForReuse(imagePath string) (*PrepareResult, error) {
	result := &PrepareResult{}

	// Check if virt-customize is available
	if _, err := exec.LookPath("virt-customize"); err != nil {
		// virt-customize not available, skip preparation
		// Cloud-init runcmd will inject SSH keys on boot, so this should still work
		return result, nil
	}

	// Run virt-customize to prepare the image
	// - Remove authorized_keys so new keys can be injected via cloud-init
	// - Reset cloud-init so it re-runs on next boot
	cmd := exec.Command("virt-customize",
		"-a", imagePath,
		"--run-command", "rm -f /home/cage/.ssh/authorized_keys",
		"--run-command", "rm -f /root/.ssh/authorized_keys",
		"--run-command", "cloud-init clean --logs 2>/dev/null || rm -rf /var/lib/cloud/instances",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// virt-customize failed (common with libguestfs misconfiguration)
		// Don't fail the save - cloud-init runcmd will inject SSH keys on boot
		result.VirtCustomizeError = fmt.Sprintf("virt-customize failed: %s", strings.TrimSpace(string(output)))
		return result, nil
	}

	result.VirtCustomizeUsed = true
	return result, nil
}

// Delete removes an image
func Delete(imageName string, force bool) error {
	// Check image exists
	if !Exists(imageName) {
		return ErrImageNotFound
	}

	// Check if base image
	if IsBaseImage(imageName) && !force {
		return fmt.Errorf("cannot delete base image '%s', use --force", imageName)
	}

	// Check not in use by any cage
	cages, err := cage.List()
	if err != nil {
		return fmt.Errorf("failed to list cages: %w", err)
	}

	for _, c := range cages {
		if c.Image == imageName {
			return fmt.Errorf("image in use by cage '%s'", c.Name)
		}
	}

	// Delete image file
	imagePath := ImagePath(imageName)
	if err := os.Remove(imagePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	// Delete metadata
	_ = DeleteMetadata(imageName)

	return nil
}

// Inspect returns detailed information about an image
func Inspect(imageName string) (*ImageDetails, error) {
	// Check image exists
	if !Exists(imageName) {
		return nil, ErrImageNotFound
	}

	imagePath := ImagePath(imageName)

	// Load metadata
	meta, _ := LoadMetadata(imageName)

	// Get basic info if no metadata
	var img Image
	if meta != nil {
		img = *meta
	} else {
		info, err := os.Stat(imagePath)
		if err != nil {
			return nil, err
		}
		img = Image{
			Name: imageName,
			Type: "base",
			Size: info.Size(),
			Path: imagePath,
		}
	}

	// Get qcow2 info
	cmd := exec.Command("qemu-img", "info", "--output=json", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get image info: %w", err)
	}

	var qcowInfo struct {
		VirtualSize int64  `json:"virtual-size"`
		ActualSize  int64  `json:"actual-size"`
		Format      string `json:"format"`
		BackingFile string `json:"backing-filename"`
	}
	if err := json.Unmarshal(output, &qcowInfo); err != nil {
		return nil, fmt.Errorf("failed to parse image info: %w", err)
	}

	return &ImageDetails{
		Image:       img,
		VirtualSize: qcowInfo.VirtualSize,
		ActualSize:  qcowInfo.ActualSize,
		Format:      qcowInfo.Format,
		BackingFile: qcowInfo.BackingFile,
	}, nil
}

// FormatSize formats bytes as human readable string
func FormatSize(bytes int64) string {
	const (
		MB = 1024 * 1024
		GB = 1024 * MB
	)

	if bytes >= GB {
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	}
	return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
}
