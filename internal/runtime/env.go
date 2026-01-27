package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RuntimeDir returns the runtime directory for a cage
func RuntimeDir(cageDir string) string {
	return filepath.Join(cageDir, "runtime")
}

// EnvFilePath returns the path to the env file for a cage
func EnvFilePath(cageDir string) string {
	return filepath.Join(RuntimeDir(cageDir), "env.sh")
}

// EnsureRuntimeDir creates the runtime directory for a cage
func EnsureRuntimeDir(cageDir string) error {
	return os.MkdirAll(RuntimeDir(cageDir), 0755)
}

// WriteEnvFile writes environment variables to a shell-sourceable file
// The file can be sourced by shell profiles in the VM
func WriteEnvFile(path string, env map[string]string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Build content
	var builder strings.Builder

	// Write header
	builder.WriteString("# Cage runtime environment\n")
	builder.WriteString("# This file is auto-generated - do not edit\n")
	builder.WriteString("\n")

	// Sort keys for deterministic output
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write exports
	for _, key := range keys {
		value := env[key]
		escapedValue := escapeShellQuote(value)
		builder.WriteString(fmt.Sprintf("export %s='%s'\n", key, escapedValue))
	}

	// Write file with 0644 permissions
	return os.WriteFile(path, []byte(builder.String()), 0644)
}

// escapeShellQuote escapes single quotes in a string for use inside single-quoted shell strings
// The pattern: ' -> '\” (end quote, escaped quote, start quote)
func escapeShellQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}
