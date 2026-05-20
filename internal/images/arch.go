package images

import (
	"runtime"

	"github.com/s-oravec/claude-cage/internal/manifest"
)

// HostArchitecture returns the Go GOARCH of the current host (e.g. "amd64", "arm64").
// Matches the wire values used by cage-hub (ArchitectureSchema) and manifest.Config.Arch.
func HostArchitecture() string { return runtime.GOARCH }

// SupportedArchitectures is the closed arch whitelist. Single source of truth is
// manifest.SupportedArch (what manifest.Validate enforces); re-exported here for
// CLI-side flag validation.
var SupportedArchitectures = manifest.SupportedArch
