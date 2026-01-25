// test/e2e/build_test.go
package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	// Check prerequisites
	stdout, _, err := runCage("doctor")
	if err != nil {
		t.Skipf("Prerequisites not met: %v", err)
	}
	if !strings.Contains(stdout, "All checks passed") {
		t.Skip("Doctor checks failed, skipping e2e tests")
	}

	// Check base image available
	stdout, _, _ = runCage("image", "list")
	if !strings.Contains(stdout, testImage) {
		t.Skipf("Test image %s not available", testImage)
	}

	t.Run("build simple image", func(t *testing.T) {
		// Create temp directory for build context
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create Cagefile
		cagefile := `FROM ` + testImage + `
RUN echo "hello from build" > /tmp/build-test.txt
`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		// Build image
		imageName := uniqueName(t) + "-image"
		t.Cleanup(func() {
			runCage("image", "remove", imageName, "--force")
		})

		stdout, stderr, err := runCageWithTimeout(5*time.Minute, "build", "-t", imageName, tmpDir)
		if err != nil {
			t.Fatalf("build failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		if !strings.Contains(stdout, "Successfully built image") {
			t.Errorf("expected success message, got: %s", stdout)
		}

		// Verify image exists
		stdout, _, _ = runCage("image", "list")
		if !strings.Contains(stdout, imageName) {
			t.Errorf("image not found in list")
		}
	})

	t.Run("build with COPY", func(t *testing.T) {
		// Create temp directory for build context
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create file to copy
		if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Create Cagefile
		cagefile := `FROM ` + testImage + `
WORKDIR /app
COPY ./test.txt /app/test.txt
RUN cat /app/test.txt
`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		// Build image
		imageName := uniqueName(t) + "-image"
		t.Cleanup(func() {
			runCage("image", "remove", imageName, "--force")
		})

		stdout, stderr, err := runCageWithTimeout(5*time.Minute, "build", "-t", imageName, tmpDir)
		if err != nil {
			t.Fatalf("build failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		// Should see the file content in output
		if !strings.Contains(stdout, "test content") {
			t.Errorf("expected to see file content in output")
		}
	})

	t.Run("build with ARG", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		cagefile := `FROM ` + testImage + `
ARG MESSAGE=default
RUN echo ${MESSAGE} > /tmp/message.txt && cat /tmp/message.txt
`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		imageName := uniqueName(t) + "-image"
		t.Cleanup(func() {
			runCage("image", "remove", imageName, "--force")
		})

		stdout, stderr, err := runCageWithTimeout(5*time.Minute, "build", "-t", imageName, "--build-arg", "MESSAGE=custom-value", tmpDir)
		if err != nil {
			t.Fatalf("build failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}

		if !strings.Contains(stdout, "custom-value") {
			t.Errorf("expected custom-value in output, got: %s", stdout)
		}
	})

	t.Run("build error handling", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cage-build-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Invalid Cagefile - missing FROM
		cagefile := `RUN echo hello`
		if err := os.WriteFile(filepath.Join(tmpDir, "Cagefile"), []byte(cagefile), 0644); err != nil {
			t.Fatalf("failed to write Cagefile: %v", err)
		}

		imageName := uniqueName(t) + "-image"
		_, stderr, err := runCage("build", "-t", imageName, tmpDir)
		if err == nil {
			t.Error("expected error for invalid Cagefile")
			runCage("image", "remove", imageName, "--force")
		}

		if !strings.Contains(stderr, "FROM") {
			t.Errorf("expected FROM error message, got: %s", stderr)
		}
	})
}
