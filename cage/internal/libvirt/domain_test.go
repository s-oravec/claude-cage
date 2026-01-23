package libvirt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainConfig_Structure(t *testing.T) {
	cfg := DomainConfig{
		Name:         "test",
		MemoryMB:     4096,
		VCPU:         4,
		DiskPath:     "/path/to/disk.qcow2",
		CloudInitISO: "/path/to/cloud-init.iso",
		NetworkName:  "cage-test",
	}

	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, 4096, cfg.MemoryMB)
}

func TestGenerateDomainXML(t *testing.T) {
	cfg := &DomainConfig{
		Name:         "test",
		MemoryMB:     4096,
		VCPU:         4,
		DiskPath:     "/home/user/.claude-cage/cages/test/disk.qcow2",
		CloudInitISO: "/home/user/.claude-cage/cages/test/cloud-init.iso",
		NetworkName:  "default",
	}

	xml, err := GenerateDomainXML(cfg)
	assert.NoError(t, err)

	// Check required elements
	assert.Contains(t, xml, "<name>cage-test</name>")
	assert.Contains(t, xml, "<memory unit='MiB'>4096</memory>")
	assert.Contains(t, xml, "<vcpu>4</vcpu>")
	assert.Contains(t, xml, cfg.DiskPath)
	assert.Contains(t, xml, cfg.CloudInitISO)
	assert.Contains(t, xml, "type='kvm'")
	assert.Contains(t, xml, "host-passthrough")
}

func TestGenerateDomainXML_ValidXML(t *testing.T) {
	cfg := &DomainConfig{
		Name:         "myproject",
		MemoryMB:     8192,
		VCPU:         8,
		DiskPath:     "/tmp/disk.qcow2",
		CloudInitISO: "/tmp/cloud-init.iso",
		NetworkName:  "default",
	}

	xml, err := GenerateDomainXML(cfg)
	assert.NoError(t, err)

	// Should be valid XML structure
	assert.True(t, strings.HasPrefix(xml, "<domain"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(xml), "</domain>"))
}
