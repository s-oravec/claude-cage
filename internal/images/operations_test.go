package images

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
)

func TestList(t *testing.T) {
	// Create temp dir for tests
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

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

	t.Run("save non-existent cage", func(t *testing.T) {
		err := Save("nonexistent", "new-image", "")
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

		// Create existing image
		imgPath := filepath.Join(tmpDir, "existing.qcow2")
		os.WriteFile(imgPath, []byte("test"), 0644)

		err := Save("test-cage", "existing", "")
		if err != ErrImageExists {
			t.Errorf("expected ErrImageExists, got %v", err)
		}

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
