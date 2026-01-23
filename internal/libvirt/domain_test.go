package libvirt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainConfig_Structure(t *testing.T) {
	cfg := DomainConfig{
		Name:           "test",
		MemoryMB:       4096,
		VCPU:           4,
		DiskPath:       "/path/to/disk.qcow2",
		CloudInitISO:   "/path/to/cloud-init.iso",
		NetworkName:    "cage-test",
		VirtiofsSocket: "/run/virtiofs.sock",
	}

	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, 4096, cfg.MemoryMB)
	assert.Equal(t, 4, cfg.VCPU)
	assert.Equal(t, "/path/to/disk.qcow2", cfg.DiskPath)
	assert.Equal(t, "/path/to/cloud-init.iso", cfg.CloudInitISO)
	assert.Equal(t, "cage-test", cfg.NetworkName)
	assert.Equal(t, "/run/virtiofs.sock", cfg.VirtiofsSocket)
}

func TestGenerateDomainXML_BridgeNetwork(t *testing.T) {
	cfg := &DomainConfig{
		Name:         "test",
		MemoryMB:     4096,
		VCPU:         4,
		DiskPath:     "/home/user/.claude-cage/cages/test/disk.qcow2",
		CloudInitISO: "/home/user/.claude-cage/cages/test/cloud-init.iso",
		NetworkName:  "cage-test",
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// Check required elements
	assert.Contains(t, xml, "<name>cage-test</name>")
	assert.Contains(t, xml, "<memory unit='MiB'>4096</memory>")
	assert.Contains(t, xml, "<vcpu>4</vcpu>")
	assert.Contains(t, xml, cfg.DiskPath)
	assert.Contains(t, xml, cfg.CloudInitISO)
	assert.Contains(t, xml, "type='kvm'")
	assert.Contains(t, xml, "host-passthrough")

	// Bridge network should use network interface
	assert.Contains(t, xml, "<interface type='network'>")
	assert.Contains(t, xml, "<source network='cage-test'/>")
	assert.NotContains(t, xml, "<interface type='user'>")
}

func TestGenerateDomainXML_UserNetwork(t *testing.T) {
	cfg := &DomainConfig{
		Name:         "test",
		MemoryMB:     2048,
		VCPU:         2,
		DiskPath:     "/tmp/disk.qcow2",
		CloudInitISO: "/tmp/cloud-init.iso",
		NetworkName:  "", // Empty = user-mode networking
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// User-mode networking should use user interface
	assert.Contains(t, xml, "<interface type='user'>")
	assert.NotContains(t, xml, "<interface type='network'>")
	assert.NotContains(t, xml, "<source network=")
}

func TestGenerateDomainXML_WithVirtiofs(t *testing.T) {
	cfg := &DomainConfig{
		Name:           "test",
		MemoryMB:       4096,
		VCPU:           4,
		DiskPath:       "/tmp/disk.qcow2",
		CloudInitISO:   "/tmp/cloud-init.iso",
		NetworkName:    "default",
		VirtiofsSocket: "/run/cage/test/virtiofs.sock",
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// Should have virtiofs filesystem
	assert.Contains(t, xml, "<filesystem type='mount'")
	assert.Contains(t, xml, "type='virtiofs'")
	assert.Contains(t, xml, cfg.VirtiofsSocket)
	assert.Contains(t, xml, "<target dir='workspace'/>")

	// Should have shared memory backing for virtiofs
	assert.Contains(t, xml, "<memoryBacking>")
	assert.Contains(t, xml, "<source type='memfd'/>")
	assert.Contains(t, xml, "<access mode='shared'/>")
}

func TestGenerateDomainXML_WithoutVirtiofs(t *testing.T) {
	cfg := &DomainConfig{
		Name:           "test",
		MemoryMB:       4096,
		VCPU:           4,
		DiskPath:       "/tmp/disk.qcow2",
		CloudInitISO:   "/tmp/cloud-init.iso",
		NetworkName:    "default",
		VirtiofsSocket: "", // No virtiofs
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// Should NOT have virtiofs elements
	assert.NotContains(t, xml, "<filesystem type='mount'")
	assert.NotContains(t, xml, "type='virtiofs'")
	assert.NotContains(t, xml, "<memoryBacking>")
}

func TestGenerateDomainXML_ValidXMLStructure(t *testing.T) {
	cfg := &DomainConfig{
		Name:         "myproject",
		MemoryMB:     8192,
		VCPU:         8,
		DiskPath:     "/tmp/disk.qcow2",
		CloudInitISO: "/tmp/cloud-init.iso",
		NetworkName:  "default",
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// Should be valid XML structure
	assert.True(t, strings.HasPrefix(xml, "<domain"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(xml), "</domain>"))
}

func TestGenerateDomainXML_ContainsRequiredElements(t *testing.T) {
	cfg := &DomainConfig{
		Name:         "fulltest",
		MemoryMB:     4096,
		VCPU:         4,
		DiskPath:     "/var/lib/cage/disk.qcow2",
		CloudInitISO: "/var/lib/cage/cloud-init.iso",
		NetworkName:  "cage-net",
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// OS configuration
	assert.Contains(t, xml, "<os>")
	assert.Contains(t, xml, "<type arch='x86_64'>hvm</type>")
	assert.Contains(t, xml, "<boot dev='hd'/>")

	// Features
	assert.Contains(t, xml, "<features>")
	assert.Contains(t, xml, "<acpi/>")
	assert.Contains(t, xml, "<apic/>")

	// CPU
	assert.Contains(t, xml, "<cpu mode='host-passthrough'/>")

	// Disk configuration
	assert.Contains(t, xml, "<disk type='file' device='disk'>")
	assert.Contains(t, xml, "type='qcow2'")
	assert.Contains(t, xml, "<target dev='vda' bus='virtio'/>")

	// Cloud-init CDROM
	assert.Contains(t, xml, "<disk type='file' device='cdrom'>")
	assert.Contains(t, xml, "<target dev='sda' bus='sata'/>")
	assert.Contains(t, xml, "<readonly/>")

	// Console
	assert.Contains(t, xml, "<serial type='pty'>")
	assert.Contains(t, xml, "<console type='pty'>")

	// RNG
	assert.Contains(t, xml, "<rng model='virtio'>")
	assert.Contains(t, xml, "/dev/urandom")
}

func TestGenerateDomainXML_DifferentMemorySizes(t *testing.T) {
	testCases := []struct {
		name     string
		memoryMB int
	}{
		{"light", 2048},
		{"default", 4096},
		{"heavy", 8192},
		{"large", 16384},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &DomainConfig{
				Name:         tc.name,
				MemoryMB:     tc.memoryMB,
				VCPU:         4,
				DiskPath:     "/tmp/disk.qcow2",
				CloudInitISO: "/tmp/cloud-init.iso",
			}

			xml, err := GenerateDomainXML(cfg)
			require.NoError(t, err)
			assert.Contains(t, xml, "<memory unit='MiB'>")
		})
	}
}

func TestGenerateDomainXML_DifferentVCPUs(t *testing.T) {
	testCases := []int{1, 2, 4, 8, 16}

	for _, vcpu := range testCases {
		cfg := &DomainConfig{
			Name:         "test",
			MemoryMB:     4096,
			VCPU:         vcpu,
			DiskPath:     "/tmp/disk.qcow2",
			CloudInitISO: "/tmp/cloud-init.iso",
		}

		xml, err := GenerateDomainXML(cfg)
		require.NoError(t, err)
		assert.Contains(t, xml, "<vcpu>")
	}
}

func TestGenerateDomainXML_SpecialCharactersInPaths(t *testing.T) {
	cfg := &DomainConfig{
		Name:         "test-project",
		MemoryMB:     4096,
		VCPU:         4,
		DiskPath:     "/home/user/my projects/cage/disk.qcow2",
		CloudInitISO: "/home/user/my projects/cage/cloud-init.iso",
		NetworkName:  "cage-test-project",
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	assert.Contains(t, xml, cfg.DiskPath)
	assert.Contains(t, xml, cfg.CloudInitISO)
}

func TestGenerateDomainXML_CombinedUserNetworkAndVirtiofs(t *testing.T) {
	cfg := &DomainConfig{
		Name:           "test",
		MemoryMB:       4096,
		VCPU:           4,
		DiskPath:       "/tmp/disk.qcow2",
		CloudInitISO:   "/tmp/cloud-init.iso",
		NetworkName:    "", // User-mode networking
		VirtiofsSocket: "/run/virtiofs.sock",
	}

	xml, err := GenerateDomainXML(cfg)
	require.NoError(t, err)

	// Should have user network
	assert.Contains(t, xml, "<interface type='user'>")

	// Should have virtiofs
	assert.Contains(t, xml, "<filesystem type='mount'")
	assert.Contains(t, xml, "<memoryBacking>")
}
