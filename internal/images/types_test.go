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

func TestMetadataDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	dir := MetadataDir()
	if dir == "" {
		t.Error("MetadataDir should not be empty")
	}
	if !contains(dir, "metadata") {
		t.Errorf("MetadataDir should contain 'metadata': %s", dir)
	}
}

func TestMetadataPath(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	path := MetadataPath("ubuntu-24.04")
	if !contains(path, "ubuntu-24.04.json") {
		t.Errorf("MetadataPath should end with name.json: %s", path)
	}
	if !contains(path, "metadata") {
		t.Errorf("MetadataPath should contain metadata dir: %s", path)
	}
}

func TestEnsureMetadataDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := imagesDir
	imagesDir = tmpDir
	defer func() { imagesDir = oldDir }()

	err := EnsureMetadataDir()
	if err != nil {
		t.Errorf("EnsureMetadataDir failed: %v", err)
	}

	// Check directory exists
	info, err := os.Stat(MetadataDir())
	if err != nil {
		t.Errorf("MetadataDir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("MetadataDir should be a directory")
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrImageNotFound, "image not found"},
		{ErrImageInUse, "image is in use"},
		{ErrImageExists, "image already exists"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.want {
			t.Errorf("error = %q, want %q", tt.err.Error(), tt.want)
		}
	}
}

func TestImage_Structure(t *testing.T) {
	now := time.Now()
	img := Image{
		Name:        "test",
		Type:        "custom",
		Base:        "ubuntu",
		Size:        1024,
		Description: "Test image",
		CreatedAt:   now,
		Path:        "/path/to/image.qcow2",
	}

	if img.Name != "test" {
		t.Errorf("Name = %q, want %q", img.Name, "test")
	}
	if img.Type != "custom" {
		t.Errorf("Type = %q, want %q", img.Type, "custom")
	}
	if img.Base != "ubuntu" {
		t.Errorf("Base = %q, want %q", img.Base, "ubuntu")
	}
	if img.Size != 1024 {
		t.Errorf("Size = %d, want %d", img.Size, 1024)
	}
}

func TestImageDetails_Structure(t *testing.T) {
	details := ImageDetails{
		Image: Image{
			Name: "test",
		},
		VirtualSize: 10240,
		ActualSize:  1024,
		Format:      "qcow2",
		BackingFile: "/path/to/base.qcow2",
	}

	if details.Name != "test" {
		t.Errorf("Name = %q, want %q", details.Name, "test")
	}
	if details.VirtualSize != 10240 {
		t.Errorf("VirtualSize = %d, want %d", details.VirtualSize, 10240)
	}
	if details.ActualSize != 1024 {
		t.Errorf("ActualSize = %d, want %d", details.ActualSize, 1024)
	}
	if details.Format != "qcow2" {
		t.Errorf("Format = %q, want %q", details.Format, "qcow2")
	}
	if details.BackingFile != "/path/to/base.qcow2" {
		t.Errorf("BackingFile = %q, want %q", details.BackingFile, "/path/to/base.qcow2")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
