package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/s-oravec/cage/internal/cage"
	"github.com/s-oravec/cage/internal/images"
	"github.com/s-oravec/cage/internal/imgstore"
	"github.com/s-oravec/cage/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageCmd(t *testing.T) {
	// Create temp dirs for tests
	tmpDir := t.TempDir()

	// Override images dir
	imagesDir := filepath.Join(tmpDir, "images")
	os.MkdirAll(imagesDir, 0755)
	oldImagesDir := images.Dir()
	images.SetDir(imagesDir)
	defer images.SetDir(oldImagesDir)

	// Override cages dir
	oldCagesDir := cage.CagesDir()
	cage.SetCagesDir(filepath.Join(tmpDir, "cages"))
	defer cage.SetCagesDir(oldCagesDir)

	// Isolate imgstore root so the user's real ~/.cage/refs/ tree
	// does not leak custom images into this test.
	imgstore.SetRoot(filepath.Join(tmpDir, "store"))
	defer imgstore.SetRoot("")

	t.Run("image has subcommands", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"image", "--help"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		output := buf.String()
		if !contains(output, "list") {
			t.Error("image should have list subcommand")
		}
		if !contains(output, "save") {
			t.Error("image should have save subcommand")
		}
		if !contains(output, "remove") {
			t.Error("image should have remove subcommand")
		}
		if !contains(output, "inspect") {
			t.Error("image should have inspect subcommand")
		}
	})

	t.Run("image list empty", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"image", "list"})

		err := cmd.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		output := buf.String()
		if !contains(output, "No images found") {
			t.Error("should indicate no images")
		}
	})

	t.Run("image save requires cage name", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"image", "save"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when cage name not provided")
		}
	})

	t.Run("image save requires --name flag", func(t *testing.T) {
		// Create test cage
		state := &cage.State{
			Name:      "savecage",
			Status:    cage.StatusRunning,
			Image:     "ubuntu-24.04",
			Profile:   "default",
			StartedAt: time.Now(),
		}
		if err := cage.SaveState(state); err != nil {
			t.Fatal(err)
		}
		defer cage.DeleteState("savecage")

		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"image", "save", "savecage"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when --name not provided")
		}
	})

	t.Run("image save of non-existent cage", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"image", "save", "nonexistent", "--name", "newimg"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent cage")
		}
	})

	t.Run("image delete non-existent", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"image", "delete", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent image")
		}
	})

	t.Run("image inspect non-existent", func(t *testing.T) {
		cmd := NewRootCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"image", "inspect", "nonexistent"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent image")
		}
	})

	t.Run("image remove has force flag", func(t *testing.T) {
		cmd := newImageRemoveCmd()
		flag := cmd.Flags().Lookup("force")
		if flag == nil {
			t.Error("remove should have --force flag")
		}
	})

	t.Run("image save has description flag", func(t *testing.T) {
		cmd := newImageSaveCmd()
		flag := cmd.Flags().Lookup("description")
		if flag == nil {
			t.Error("save should have --description flag")
		}
	})
}

// TestImageList_HasArchColumn verifies the ARCH header is present and a custom
// image's recorded architecture appears in its row.
func TestImageList_HasArchColumn(t *testing.T) {
	tmpDir := t.TempDir()

	imagesDir := filepath.Join(tmpDir, "images")
	require.NoError(t, os.MkdirAll(imagesDir, 0755))
	oldImagesDir := images.Dir()
	images.SetDir(imagesDir)
	defer images.SetDir(oldImagesDir)

	imgstore.SetRoot(filepath.Join(tmpDir, "store"))
	defer imgstore.SetRoot("")

	// Seed a custom image: a single-arch manifest plus a ref pointing at it.
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV1,
		MediaType:     manifest.MediaTypeManifestV1,
		Base:          manifest.Base{Type: "distro", Name: "alpine", Digest: "sha256:base"},
		Config:        manifest.Config{OS: "linux", Arch: "arm64"},
	}
	body, err := json.Marshal(&m)
	require.NoError(t, err)
	digest := manifest.DigestBytes(body)
	require.NoError(t, imgstore.PutManifestBytes(digest, body))

	ref, err := imgstore.ParseRef("custom:1.0")
	require.NoError(t, err)
	require.NoError(t, imgstore.WriteRef(ref, digest))

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"image", "list"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "ARCH")
	assert.Contains(t, out, "custom:1.0")
	assert.Contains(t, out, "arm64")
}

func TestImageRm_HasRmSubcommand(t *testing.T) {
	cmd := NewImageCmd()
	rm, _, err := cmd.Find([]string{"rm"})
	require.NoError(t, err)
	assert.NotNil(t, rm)
}

// Helper to check if images package is properly set up
func TestImagesPackageIntegration(t *testing.T) {
	t.Run("FormatSize works", func(t *testing.T) {
		size := images.FormatSize(1024 * 1024 * 100)
		if size != "100 MB" {
			t.Errorf("expected '100 MB', got '%s'", size)
		}
	})
}
