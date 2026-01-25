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
	"github.com/s-oravec/claude-cage/internal/ssh"
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

// Save creates a new image from a running cage
func Save(cageName, imageName, description string) error {
	// Check cage exists
	if !cage.Exists(cageName) {
		return fmt.Errorf("cage '%s' not found", cageName)
	}

	// Load cage state
	state, err := cage.LoadState(cageName)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	// Cage must be running so we can SSH in and prepare it
	if state.Status != cage.StatusRunning {
		return fmt.Errorf("cage '%s' must be running to save (need to prepare image for reuse)", cageName)
	}

	// Check image name not taken
	if Exists(imageName) {
		return ErrImageExists
	}

	// Prepare the image for reuse by clearing SSH keys and resetting cloud-init
	if err := prepareForSave(cageName, state); err != nil {
		return fmt.Errorf("failed to prepare image: %w", err)
	}

	// Get source disk path
	sourceDisk := filepath.Join(cage.Dir(cageName), "disk.qcow2")
	if _, err := os.Stat(sourceDisk); err != nil {
		return fmt.Errorf("cage disk not found: %w", err)
	}

	// Ensure images directory exists
	if err := EnsureDir(); err != nil {
		return err
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
		return fmt.Errorf("failed to save image: %s", strings.TrimSpace(string(output)))
	}

	// Get size of new image
	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("failed to stat image: %w", err)
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
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// prepareForSave prepares a running cage for saving as a reusable image
// It clears SSH authorized_keys and resets cloud-init so it re-runs on next boot
func prepareForSave(cageName string, state *cage.State) error {
	// Determine SSH target
	var host string
	var port int

	if state.SSHPort > 0 {
		host = "127.0.0.1"
		port = state.SSHPort
	} else if state.IP != "" {
		host = state.IP
		port = 22
	} else {
		return fmt.Errorf("no SSH access to cage")
	}

	// Commands to prepare the image for reuse:
	// 1. Remove authorized_keys so new keys can be injected
	// 2. Reset cloud-init so it re-runs on next boot
	prepareCommands := []string{
		"rm -f ~/.ssh/authorized_keys",
		"sudo cloud-init clean --logs 2>/dev/null || sudo rm -rf /var/lib/cloud/instances",
	}

	for _, cmd := range prepareCommands {
		_, err := ssh.ExecCaptureWithPort(cageName, host, port, cmd)
		if err != nil {
			// Log but don't fail on cleanup errors
			continue
		}
	}

	return nil
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
	DeleteMetadata(imageName)

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
