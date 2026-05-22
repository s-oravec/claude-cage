package images

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/s-oravec/cage/internal/cage"
	"github.com/s-oravec/cage/internal/imgstore"
	"github.com/s-oravec/cage/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	// Create temp dir for tests
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// Isolate imgstore root so the real ~/.claude-cage/refs/ does not bleed in.
	storeRoot := t.TempDir()
	imgstore.SetRoot(storeRoot)
	defer imgstore.SetRoot("")

	t.Run("empty directory", func(t *testing.T) {
		images, err := List()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(images) != 0 {
			t.Errorf("expected empty list, got %d", len(images))
		}
	})

	t.Run("list images with and without metadata", func(t *testing.T) {
		// Create base image (no metadata)
		baseImg := filepath.Join(tmpDir, "base-image.qcow2")
		if err := os.WriteFile(baseImg, []byte("base"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create custom image with metadata
		customImg := filepath.Join(tmpDir, "custom-image.qcow2")
		if err := os.WriteFile(customImg, []byte("custom"), 0644); err != nil {
			t.Fatal(err)
		}
		SaveMetadata(&Image{
			Name:        "custom-image",
			Type:        "custom",
			Base:        "base-image",
			Description: "Test custom",
			CreatedAt:   time.Now(),
			Path:        customImg,
		})

		images, err := List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(images) != 2 {
			t.Errorf("expected 2 images, got %d", len(images))
		}

		// Check types
		typeMap := make(map[string]string)
		for _, img := range images {
			typeMap[img.Name] = img.Type
		}

		if typeMap["base-image"] != "base" {
			t.Errorf("expected base type for base-image")
		}
		if typeMap["custom-image"] != "custom" {
			t.Errorf("expected custom type for custom-image")
		}
	})
}

func TestSave(t *testing.T) {
	// Create temp dirs for tests
	tmpDir := t.TempDir()
	oldImagesDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldImagesDir }()

	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(filepath.Join(tmpDir, "cages"))
	defer cage.SetCagesDir(oldCagesDir)

	imgstore.SetRoot(tmpDir)
	defer imgstore.SetRoot("")

	t.Run("save non-existent cage", func(t *testing.T) {
		_, err := Save("nonexistent", "new-image", "")
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("save to existing image name", func(t *testing.T) {
		// Create cage (must be stopped)
		state := &cage.State{
			Name:    "test-cage",
			Status:  cage.StatusStopped,
			Image:   "ubuntu-24.04",
			Profile: "default",
		}
		cage.SaveState(state)

		// Pre-create a ref so the existence check fires before any disk work.
		ref, err := imgstore.ParseRef("existing")
		require.NoError(t, err)
		require.NoError(t, imgstore.WriteRef(ref, "sha256:0000000000000000000000000000000000000000000000000000000000000000"))

		_, err = Save("test-cage", "existing", "")
		if err != ErrImageExists {
			t.Errorf("expected ErrImageExists, got %v", err)
		}

		require.NoError(t, imgstore.DeleteRef(ref))
		cage.DeleteState("test-cage")
	})
}

func TestDelete(t *testing.T) {
	// Create temp dir for tests
	tmpDir := t.TempDir()
	oldImagesDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldImagesDir }()

	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(filepath.Join(tmpDir, "cages"))
	defer cage.SetCagesDir(oldCagesDir)

	t.Run("delete non-existent image", func(t *testing.T) {
		err := Delete("nonexistent", false)
		if err != ErrImageNotFound {
			t.Errorf("expected ErrImageNotFound, got %v", err)
		}
	})

	t.Run("delete base image without force", func(t *testing.T) {
		// Create base image (no metadata)
		imgPath := filepath.Join(tmpDir, "base-only.qcow2")
		os.WriteFile(imgPath, []byte("base"), 0644)

		err := Delete("base-only", false)
		if err == nil {
			t.Error("expected error when deleting base without force")
		}
	})

	t.Run("delete base image with force", func(t *testing.T) {
		// Create base image
		imgPath := filepath.Join(tmpDir, "base-force.qcow2")
		os.WriteFile(imgPath, []byte("base"), 0644)

		err := Delete("base-force", true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if Exists("base-force") {
			t.Error("image should be deleted")
		}
	})

	t.Run("delete custom image", func(t *testing.T) {
		// Create custom image
		imgPath := filepath.Join(tmpDir, "custom-del.qcow2")
		os.WriteFile(imgPath, []byte("custom"), 0644)
		SaveMetadata(&Image{
			Name: "custom-del",
			Type: "custom",
			Path: imgPath,
		})

		err := Delete("custom-del", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if Exists("custom-del") {
			t.Error("image should be deleted")
		}

		meta, _ := LoadMetadata("custom-del")
		if meta != nil {
			t.Error("metadata should be deleted")
		}
	})

	t.Run("cannot delete image in use", func(t *testing.T) {
		// Create image
		imgPath := filepath.Join(tmpDir, "in-use.qcow2")
		os.WriteFile(imgPath, []byte("inuse"), 0644)
		SaveMetadata(&Image{
			Name: "in-use",
			Type: "custom",
			Path: imgPath,
		})

		// Create cage using the image
		state := &cage.State{
			Name:    "user-cage",
			Status:  cage.StatusRunning,
			Image:   "in-use",
			Profile: "default",
		}
		cage.SaveState(state)

		err := Delete("in-use", false)
		if err == nil {
			t.Error("expected error when deleting image in use")
		}

		cage.DeleteState("user-cage")
	})
}

func TestInspect(t *testing.T) {
	// Create temp dir for tests
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	t.Run("inspect non-existent image", func(t *testing.T) {
		_, err := Inspect("nonexistent")
		if err != ErrImageNotFound {
			t.Errorf("expected ErrImageNotFound, got %v", err)
		}
	})
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{100 * 1024 * 1024, "100 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{2560 * 1024 * 1024, "2.5 GB"},
		{512 * 1024 * 1024, "512 MB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestBaseDigest_ReadsFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	// Write a fake base image
	path := filepath.Join(tmpDir, "ubuntu-24.04.qcow2")
	require.NoError(t, os.WriteFile(path, []byte("fakebase"), 0644))

	d, err := BaseDigest("ubuntu-24.04")
	require.NoError(t, err)
	// sha256("fakebase") - we don't need to know the exact value; just verify the format.
	assert.True(t, strings.HasPrefix(d, "sha256:"))
	assert.Len(t, d, len("sha256:")+64)
}

func TestSaveLayered_WritesAllArtifacts(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed; run on dev host")
	}
	root := t.TempDir()
	imagesDir = root
	imgstore.SetRoot(root)
	defer func() { imagesDir = ""; imgstore.SetRoot("") }()

	// Make a fake base + overlay.
	base := filepath.Join(root, "ubuntu-24.04.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", base, "1M").Run())

	overlayDir := t.TempDir()
	overlay := filepath.Join(overlayDir, "disk.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", base, "-F", "qcow2", overlay, "10M").Run())

	r, err := SaveLayered(SaveLayeredInput{
		OverlayPath: overlay,
		BaseName:    "ubuntu-24.04",
		Tag:         "myimage:v1",
		Config:      manifest.Config{OS: "linux", Arch: "amd64"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, r.ManifestDigest)
	assert.NotEmpty(t, r.LayerDigest)

	// Layer + manifest + ref are present.
	assert.True(t, imgstore.HasLayer(r.LayerDigest))
	assert.True(t, imgstore.HasManifest(r.ManifestDigest))

	ref, _ := imgstore.ParseRef("myimage:v1")
	got, err := imgstore.ReadRef(ref)
	require.NoError(t, err)
	assert.Equal(t, r.ManifestDigest, got)
}

func TestSave_BuildsValidManifestWithEmptyCagefile(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed")
	}
	root := t.TempDir()
	imagesDir = root
	imgstore.SetRoot(root)
	defer func() { imagesDir = ""; imgstore.SetRoot("") }()

	base := filepath.Join(root, "ubuntu-24.04.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", base, "1M").Run())

	overlay := filepath.Join(t.TempDir(), "disk.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", base, "-F", "qcow2", overlay, "10M").Run())

	r, err := SaveLayered(SaveLayeredInput{
		OverlayPath: overlay,
		BaseName:    "ubuntu-24.04",
		Tag:         "savedimage:latest",
		Config:      manifest.Config{OS: "linux", Arch: "amd64"},
	})
	require.NoError(t, err)

	// Read the stored manifest, verify validate passes and Cagefile is empty.
	body, err := imgstore.GetManifestBytes(r.ManifestDigest)
	require.NoError(t, err)
	var m manifest.Manifest
	require.NoError(t, json.Unmarshal(body, &m))
	require.NoError(t, m.Validate())
	assert.Empty(t, m.Config.Cagefile)
}

// TestSave_DerivesArchFromBase asserts the saved manifest's Config.Arch comes
// from BaseArch(base) - i.e. the base's recorded arch - not the host arch. We
// record a non-host arch in the base metadata and expect Save to label the
// manifest with it.
func TestSave_DerivesArchFromBase(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img not installed")
	}

	root := t.TempDir()
	oldImagesDir := imagesDir
	imagesDir = root
	imgstore.SetRoot(root)
	defer func() { imagesDir = oldImagesDir; imgstore.SetRoot("") }()

	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(filepath.Join(root, "cages"))
	defer cage.SetCagesDir(oldCagesDir)

	// Pick a base arch that differs from the host so a host-arch assumption
	// would visibly fail.
	baseArch := "arm64"
	if HostArchitecture() == "arm64" {
		baseArch = "amd64"
	}

	// Base image qcow2 + metadata recording the non-host arch.
	baseName := "ubuntu-24.04"
	basePath := filepath.Join(root, baseName+".qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2", basePath, "1M").Run())
	require.NoError(t, SaveMetadata(&Image{Name: baseName, Type: "base", Arch: baseArch, Path: basePath}))
	require.Equal(t, baseArch, BaseArch(baseName))

	// Stopped cage layered onto that base.
	cageVMDir := cage.VMDir("arch-cage")
	require.NoError(t, cage.EnsureDir("arch-cage"))
	overlay := filepath.Join(cageVMDir, "disk.qcow2")
	require.NoError(t, exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", basePath, "-F", "qcow2", overlay, "10M").Run())
	require.NoError(t, cage.SaveState(&cage.State{
		Name:    "arch-cage",
		Status:  cage.StatusStopped,
		Image:   baseName,
		Profile: "custom",
	}))

	_, err := Save("arch-cage", "arch-saved:latest", "")
	require.NoError(t, err)

	ref, err := imgstore.ParseRef("arch-saved:latest")
	require.NoError(t, err)
	digest, err := imgstore.ReadRef(ref)
	require.NoError(t, err)
	body, err := imgstore.GetManifestBytes(digest)
	require.NoError(t, err)
	var m manifest.Manifest
	require.NoError(t, json.Unmarshal(body, &m))

	assert.Equal(t, baseArch, m.Config.Arch, "manifest arch should come from BaseArch(base), not host")
}
