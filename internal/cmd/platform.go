package cmd

import (
	"fmt"
	"slices"

	"github.com/s-oravec/cage/internal/images"
)

// resolvePlatform turns the --platform flag value into a validated arch.
// Empty means "host architecture". Returns an error for anything not in the whitelist.
func resolvePlatform(flag string) (string, error) {
	arch := flag
	if arch == "" {
		arch = images.HostArchitecture()
	}
	if !slices.Contains(images.SupportedArchitectures, arch) {
		return "", fmt.Errorf("--platform: must be one of %v, got %q", images.SupportedArchitectures, arch)
	}
	return arch, nil
}
