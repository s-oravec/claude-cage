package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stiivo/cage/internal/cage"
	"github.com/stiivo/cage/internal/images"
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
		if !contains(output, "delete") {
			t.Error("image should have delete subcommand")
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

	t.Run("image delete has force flag", func(t *testing.T) {
		cmd := newImageDeleteCmd()
		flag := cmd.Flags().Lookup("force")
		if flag == nil {
			t.Error("delete should have --force flag")
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

// Helper to check if images package is properly set up
func TestImagesPackageIntegration(t *testing.T) {
	t.Run("FormatSize works", func(t *testing.T) {
		size := images.FormatSize(1024 * 1024 * 100)
		if size != "100 MB" {
			t.Errorf("expected '100 MB', got '%s'", size)
		}
	})
}
