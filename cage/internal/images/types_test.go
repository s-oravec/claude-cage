package images

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImageMetadata(t *testing.T) {
	// Create temp dir for tests
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	t.Run("save and load metadata", func(t *testing.T) {
		img := &Image{
			Name:        "test-image",
			Type:        "custom",
			Base:        "ubuntu-24.04",
			Size:        1024 * 1024 * 100, // 100 MB
			Description: "Test image",
			CreatedAt:   time.Now(),
			Path:        filepath.Join(tmpDir, "test-image.qcow2"),
		}

		if err := SaveMetadata(img); err != nil {
			t.Fatalf("failed to save metadata: %v", err)
		}

		loaded, err := LoadMetadata("test-image")
		if err != nil {
			t.Fatalf("failed to load metadata: %v", err)
		}

		if loaded.Name != img.Name {
			t.Errorf("expected name '%s', got '%s'", img.Name, loaded.Name)
		}
		if loaded.Type != img.Type {
			t.Errorf("expected type '%s', got '%s'", img.Type, loaded.Type)
		}
		if loaded.Base != img.Base {
			t.Errorf("expected base '%s', got '%s'", img.Base, loaded.Base)
		}
	})

	t.Run("load non-existent metadata", func(t *testing.T) {
		loaded, err := LoadMetadata("nonexistent")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if loaded != nil {
			t.Error("expected nil for non-existent metadata")
		}
	})

	t.Run("delete metadata", func(t *testing.T) {
		img := &Image{
			Name: "to-delete",
			Type: "custom",
			Path: filepath.Join(tmpDir, "to-delete.qcow2"),
		}

		if err := SaveMetadata(img); err != nil {
			t.Fatal(err)
		}

		if err := DeleteMetadata("to-delete"); err != nil {
			t.Errorf("failed to delete metadata: %v", err)
		}

		loaded, _ := LoadMetadata("to-delete")
		if loaded != nil {
			t.Error("metadata should be deleted")
		}
	})

	t.Run("delete non-existent metadata", func(t *testing.T) {
		err := DeleteMetadata("nonexistent")
		if err != nil {
			t.Errorf("should not error on non-existent: %v", err)
		}
	})
}

func TestIsBaseImage(t *testing.T) {
	// Create temp dir for tests
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	t.Run("no metadata means base image", func(t *testing.T) {
		if !IsBaseImage("unknown") {
			t.Error("expected true for image without metadata")
		}
	})

	t.Run("explicit base type", func(t *testing.T) {
		img := &Image{
			Name: "base-test",
			Type: "base",
			Path: filepath.Join(tmpDir, "base-test.qcow2"),
		}
		SaveMetadata(img)

		if !IsBaseImage("base-test") {
			t.Error("expected true for base type")
		}
	})

	t.Run("custom type is not base", func(t *testing.T) {
		img := &Image{
			Name: "custom-test",
			Type: "custom",
			Path: filepath.Join(tmpDir, "custom-test.qcow2"),
		}
		SaveMetadata(img)

		if IsBaseImage("custom-test") {
			t.Error("expected false for custom type")
		}
	})
}

func TestExists(t *testing.T) {
	// Create temp dir for tests
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	t.Run("non-existent image", func(t *testing.T) {
		if Exists("nonexistent") {
			t.Error("expected false for non-existent image")
		}
	})

	t.Run("existing image", func(t *testing.T) {
		// Create a dummy qcow2 file
		imgPath := filepath.Join(tmpDir, "exists-test.qcow2")
		if err := os.WriteFile(imgPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		if !Exists("exists-test") {
			t.Error("expected true for existing image")
		}
	})
}
