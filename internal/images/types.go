package images

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrImageNotFound = errors.New("image not found")
	ErrImageInUse    = errors.New("image is in use")
	ErrImageExists   = errors.New("image already exists")
)

// Image represents a VM base image
type Image struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`           // base, custom
	Base        string    `json:"base,omitempty"` // parent image (for custom)
	Arch        string    `json:"arch,omitempty"` // recorded architecture of the cached image
	Size        int64     `json:"size_bytes"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	Path        string    `json:"path"`
}

// ImageDetails extends Image with qcow2 specific info
type ImageDetails struct {
	Image
	VirtualSize int64  `json:"virtual_size"`
	ActualSize  int64  `json:"actual_size"`
	Format      string `json:"format"`
	BackingFile string `json:"backing_file,omitempty"`
}

// MetadataDir returns the directory for image metadata
func MetadataDir() string {
	return filepath.Join(Dir(), "metadata")
}

// MetadataPath returns the path to an image's metadata file
func MetadataPath(name string) string {
	return filepath.Join(MetadataDir(), name+".json")
}

// EnsureMetadataDir creates the metadata directory if it doesn't exist
func EnsureMetadataDir() error {
	return os.MkdirAll(MetadataDir(), 0755)
}

// SaveMetadata saves image metadata to disk
func SaveMetadata(img *Image) error {
	if err := EnsureMetadataDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(img, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(MetadataPath(img.Name), data, 0644)
}

// LoadMetadata loads image metadata from disk
func LoadMetadata(name string) (*Image, error) {
	data, err := os.ReadFile(MetadataPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No metadata, not an error
		}
		return nil, err
	}

	var img Image
	if err := json.Unmarshal(data, &img); err != nil {
		return nil, err
	}

	return &img, nil
}

// DeleteMetadata removes image metadata from disk
func DeleteMetadata(name string) error {
	err := os.Remove(MetadataPath(name))
	if os.IsNotExist(err) {
		return nil // Already deleted
	}
	return err
}

// Exists checks if an image exists (either as file or with metadata)
func Exists(name string) bool {
	return IsDownloaded(name)
}

// IsBaseImage checks if an image is a base image (no metadata or type=base)
func IsBaseImage(name string) bool {
	meta, err := LoadMetadata(name)
	if err != nil || meta == nil {
		return true // No metadata means base image
	}
	return meta.Type == "base"
}

// BaseArch returns the recorded architecture of a downloaded base image.
//
// It loads metadata for the resolved name and returns meta.Arch when present.
// When there is no metadata (or the recorded Arch is empty), it falls back to
// the host architecture: legacy flat caches predate multi-arch and were always
// host-arch downloads, so this keeps existing setups working.
func BaseArch(name string) string {
	meta, err := LoadMetadata(ResolveAlias(name))
	if err != nil || meta == nil || meta.Arch == "" {
		return HostArchitecture()
	}
	return meta.Arch
}
