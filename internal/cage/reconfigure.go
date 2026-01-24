package cage

import (
	"fmt"
	"path/filepath"

	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/runtime"
)

// Reconfigure updates cage configuration (shares, network).
// Cage must be stopped. Returns error if cage is running.
// This regenerates the domain XML with the new settings and redefines it in libvirt.
func Reconfigure(name string, cfg *config.ResolvedConfig) error {
	// Load current state
	state, err := LoadState(name)
	if err != nil {
		return fmt.Errorf("failed to load cage state: %w", err)
	}

	// Check if cage is running
	if state.Status == StatusRunning {
		return fmt.Errorf("cage must be stopped to reconfigure")
	}

	// Get cage directory
	cageDir := Dir(name)

	// Build paths for domain config
	overlayPath := filepath.Join(cageDir, "disk.qcow2")
	cloudInitPath := filepath.Join(cageDir, "cloud-init.iso")
	runtimeDir := runtime.RuntimeDir(cageDir)

	// Generate new domain XML with updated config
	domainCfg := &libvirt.DomainConfig{
		Name:           name,
		MemoryMB:       cfg.MemoryMB,
		VCPU:           cfg.VCPU,
		DiskPath:       overlayPath,
		CloudInitISO:   cloudInitPath,
		NetworkName:    "", // Empty for user-mode networking
		VirtiofsSocket: "", // Set at start time
		RuntimeDir:     runtimeDir,
		SSHPort:        state.SSHPort,
	}

	xml, err := libvirt.GenerateDomainXML(domainCfg)
	if err != nil {
		return fmt.Errorf("failed to generate domain XML: %w", err)
	}

	// Redefine domain in libvirt (undefine + define)
	client := libvirt.NewClient()
	if err := client.RedefineDomain(name, xml); err != nil {
		return fmt.Errorf("failed to redefine domain: %w", err)
	}

	return nil
}
