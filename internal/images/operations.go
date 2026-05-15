package images

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/imgstore"
	"github.com/s-oravec/claude-cage/internal/manifest"
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

	// Get source disk path (lives in VM artifacts dir, possibly /var/lib in root mode)
	sourceDisk := filepath.Join(cage.VMDir(cageName), "disk.qcow2")
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

// BaseDigest returns sha256:<hex> of the on-disk base image qcow2.
// Used by build/pull flows to populate or verify manifest.Base.Digest.
func BaseDigest(name string) (string, error) {
	f, err := os.Open(ImagePath(name))
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// SaveLayeredInput is the request shape for SaveLayered.
type SaveLayeredInput struct {
	OverlayPath string          // path to built qcow2 overlay with backing-file pointer set
	BaseName    string          // distro alias of the backing image (must already be in images/)
	Tag         string          // target ref, parseable by imgstore.ParseRef
	Config      manifest.Config // runtime config (os, arch, env, etc.) - validate must pass after Save
}

// SaveLayeredResult names the digests of the produced artifacts.
type SaveLayeredResult struct {
	ManifestDigest string
	LayerDigest    string
}

// SaveLayered turns a built overlay qcow2 into a layered registry-ready image:
//   - strips the backing-file pointer via `qemu-img rebase -u -b ""`
//   - stores the resulting layer in the content-addressed layer store
//   - builds + stores a manifest referencing the base and the layer
//   - writes the named ref pointing at the manifest digest
//
// MVP single-layer flow; future builds may emit multi-layer manifests.
func SaveLayered(in SaveLayeredInput) (*SaveLayeredResult, error) {
	// Copy overlay so we don't mutate the source.
	tmp, err := os.CreateTemp("", "cage-layer-*.qcow2")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := copyFile(in.OverlayPath, tmpPath); err != nil {
		return nil, fmt.Errorf("copy overlay: %w", err)
	}

	// Strip backing-file pointer (metadata only).
	cmd := exec.Command("qemu-img", "rebase", "-u", "-b", "", tmpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("qemu-img rebase: %s", string(out))
	}

	layerDigest, err := imgstore.HashFile(tmpPath)
	if err != nil {
		return nil, err
	}
	if err := imgstore.CopyFromFile(tmpPath, layerDigest); err != nil {
		return nil, err
	}
	info, err := os.Stat(imgstore.LayerPath(layerDigest))
	if err != nil {
		return nil, err
	}

	baseDigest, err := BaseDigest(in.BaseName)
	if err != nil {
		return nil, fmt.Errorf("base digest: %w", err)
	}

	m := &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeManifestV1,
		Base:          manifest.Base{Type: "distro", Name: in.BaseName, Digest: baseDigest},
		Layers:        []manifest.Layer{{Digest: layerDigest, Size: info.Size(), MediaType: manifest.MediaTypeLayerV1}},
		Config:        in.Config,
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	manifestBytes, err := manifest.Canonical(m)
	if err != nil {
		return nil, err
	}
	manifestDigest := manifest.DigestBytes(manifestBytes)
	if err := imgstore.PutManifestBytes(manifestDigest, manifestBytes); err != nil {
		return nil, err
	}

	ref, err := imgstore.ParseRef(in.Tag)
	if err != nil {
		return nil, err
	}
	if err := imgstore.WriteRef(ref, manifestDigest); err != nil {
		return nil, err
	}
	return &SaveLayeredResult{ManifestDigest: manifestDigest, LayerDigest: layerDigest}, nil
}

// copyFile copies src to dst, creating dst if needed. Used by SaveLayered to
// avoid mutating the caller's overlay when stripping the backing-file pointer.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
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
