package images

import "runtime"

// HostArchitecture returns the Go GOARCH of the current host (e.g. "amd64", "arm64").
// Matches the wire values used by cage-hub (ArchitectureSchema) and manifest.Config.Arch.
func HostArchitecture() string { return runtime.GOARCH }

// SupportedArchitectures is the closed whitelist enforced by both CLI and server.
var SupportedArchitectures = []string{"amd64", "arm64"}
