package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeDir(t *testing.T) {
	cageDir := "/home/user/.config/cage/cages/myproject"
	expected := filepath.Join(cageDir, "runtime")

	result := RuntimeDir(cageDir)

	assert.Equal(t, expected, result)
}

func TestEnvFilePath(t *testing.T) {
	cageDir := "/home/user/.config/cage/cages/myproject"
	expected := filepath.Join(cageDir, "runtime", "env.sh")

	result := EnvFilePath(cageDir)

	assert.Equal(t, expected, result)
}

func TestEnsureRuntimeDir(t *testing.T) {
	tmpDir := t.TempDir()
	cageDir := filepath.Join(tmpDir, "testcage")

	err := EnsureRuntimeDir(cageDir)
	require.NoError(t, err)

	// Directory should exist
	info, err := os.Stat(RuntimeDir(cageDir))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestEnsureRuntimeDir_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	cageDir := filepath.Join(tmpDir, "testcage")

	// Create it first
	err := os.MkdirAll(RuntimeDir(cageDir), 0755)
	require.NoError(t, err)

	// Should not fail if already exists
	err = EnsureRuntimeDir(cageDir)
	require.NoError(t, err)
}

func TestWriteEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	env := map[string]string{
		"FOO":         "bar",
		"HELLO":       "world",
		"CLAUDE_HOME": "/home/claude",
	}

	err := WriteEnvFile(envPath, env)
	require.NoError(t, err)

	// Read and verify content
	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Check header
	assert.Contains(t, contentStr, "# Cage runtime environment")
	assert.Contains(t, contentStr, "# This file is auto-generated - do not edit")

	// Check exports (should be sorted alphabetically)
	assert.Contains(t, contentStr, "export CLAUDE_HOME='/home/claude'")
	assert.Contains(t, contentStr, "export FOO='bar'")
	assert.Contains(t, contentStr, "export HELLO='world'")

	// Verify order is alphabetical
	lines := strings.Split(contentStr, "\n")
	var exports []string
	for _, line := range lines {
		if strings.HasPrefix(line, "export ") {
			exports = append(exports, line)
		}
	}
	assert.Len(t, exports, 3)
	assert.True(t, strings.HasPrefix(exports[0], "export CLAUDE_HOME="))
	assert.True(t, strings.HasPrefix(exports[1], "export FOO="))
	assert.True(t, strings.HasPrefix(exports[2], "export HELLO="))

	// Check file permissions
	info, err := os.Stat(envPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestWriteEnvFile_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	// Test with nil map
	err := WriteEnvFile(envPath, nil)
	require.NoError(t, err)

	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# Cage runtime environment")
	// Should not have any export lines
	assert.NotContains(t, contentStr, "export ")
}

func TestWriteEnvFile_EmptyMap(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	// Test with empty map
	err := WriteEnvFile(envPath, map[string]string{})
	require.NoError(t, err)

	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# Cage runtime environment")
	// Should not have any export lines
	assert.NotContains(t, contentStr, "export ")
}

func TestWriteEnvFile_EscapesQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	env := map[string]string{
		"SIMPLE":      "it's working",
		"DOUBLE":      "it's Claude's cage",
		"TRIPLE":      "'''",
		"EMPTY_QUOTE": "'",
	}

	err := WriteEnvFile(envPath, env)
	require.NoError(t, err)

	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	contentStr := string(content)

	// Single quote in value: it's -> it'\''s
	assert.Contains(t, contentStr, "export SIMPLE='it'\\''s working'")
	assert.Contains(t, contentStr, "export DOUBLE='it'\\''s Claude'\\''s cage'")
	assert.Contains(t, contentStr, "export TRIPLE=''\\'''\\'''\\'''")
	assert.Contains(t, contentStr, "export EMPTY_QUOTE=''\\'''")
}

func TestWriteEnvFile_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "deep", "nested", "env.sh")

	env := map[string]string{"FOO": "bar"}

	err := WriteEnvFile(envPath, env)
	require.NoError(t, err)

	// File should exist
	_, err = os.Stat(envPath)
	require.NoError(t, err)
}

func TestWriteEnvFile_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	env := map[string]string{
		"WITH_SPACES":   "hello world",
		"WITH_NEWLINE":  "line1\nline2",
		"WITH_DOLLAR":   "$HOME/path",
		"WITH_BACKTICK": "`command`",
	}

	err := WriteEnvFile(envPath, env)
	require.NoError(t, err)

	content, err := os.ReadFile(envPath)
	require.NoError(t, err)

	contentStr := string(content)

	// All these should be safely quoted in single quotes
	assert.Contains(t, contentStr, "export WITH_SPACES='hello world'")
	assert.Contains(t, contentStr, "export WITH_NEWLINE='line1\nline2'")
	assert.Contains(t, contentStr, "export WITH_DOLLAR='$HOME/path'")
	assert.Contains(t, contentStr, "export WITH_BACKTICK='`command`'")
}
