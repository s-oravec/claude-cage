package libvirt

import (
	"bytes"
	"text/template"
)

// DomainConfig holds configuration for a libvirt domain
type DomainConfig struct {
	Name           string
	MemoryMB       int
	VCPU           int
	DiskPath       string
	CloudInitISO   string
	NetworkName    string // if empty, uses user-mode networking (SLIRP)
	VirtiofsSocket string // optional: path to virtiofsd socket for workspace sharing
	RuntimeDir     string // optional: host path to runtime directory for env vars
	SSHPort        int    // optional: host port for SSH forwarding (user-mode only)
	PasstSocket    string // optional: path to passt socket for isolated networking
}

const domainXMLTemplate = `<domain type='kvm'{{if or (and (not .NetworkName) (gt .SSHPort 0)) .PasstSocket}} xmlns:qemu='http://libvirt.org/schemas/domain/qemu/1.0'{{end}}>
  <name>cage-{{.Name}}</name>
  <memory unit='MiB'>{{.MemoryMB}}</memory>
  <vcpu>{{.VCPU}}</vcpu>

  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='hd'/>
  </os>

  <features>
    <acpi/>
    <apic/>
  </features>

  <cpu mode='host-passthrough'/>
{{if or .VirtiofsSocket .RuntimeDir}}
  <memoryBacking>
    <source type='memfd'/>
    <access mode='shared'/>
  </memoryBacking>
{{end}}
  <devices>
    <!-- Disk (qcow2 overlay) -->
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='{{.DiskPath}}'/>
      <target dev='vda' bus='virtio'/>
    </disk>

    <!-- Cloud-init ISO: Using IDE bus instead of SATA to avoid PCI slot
         conflicts. SATA creates an AHCI controller that takes PCI slot 0x2,
         which can conflict with other devices (virtio-net-pci). IDE is
         emulated as part of PIIX chipset and doesn't occupy a separate PCI slot. -->
    <disk type='file' device='cdrom'>
      <source file='{{.CloudInitISO}}'/>
      <target dev='hdc' bus='ide'/>
      <readonly/>
    </disk>

    <!-- Network -->
{{if .NetworkName}}
    <interface type='network'>
      <source network='{{.NetworkName}}'/>
      <model type='virtio'/>
    </interface>
{{else}}{{if gt .SSHPort 0}}
    <!-- User-mode network with port forwarding handled via qemu:commandline -->
{{else}}
    <interface type='user'>
      <model type='virtio'/>
    </interface>
{{end}}{{end}}
{{if .VirtiofsSocket}}
    <!-- Virtio-fs shared directory (workspace) -->
    <filesystem type='mount' accessmode='passthrough'>
      <driver type='virtiofs' queue='1024'/>
      <source socket='{{.VirtiofsSocket}}'/>
      <target dir='workspace'/>
    </filesystem>
{{end}}
{{if .RuntimeDir}}
    <!-- Virtio-fs runtime directory (env vars) -->
    <filesystem type='mount' accessmode='passthrough'>
      <driver type='virtiofs'/>
      <source dir='{{.RuntimeDir}}'/>
      <target dir='cage-runtime'/>
    </filesystem>
{{end}}
    <!-- Console -->
    <serial type='pty'>
      <target port='0'/>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>

    <!-- Random number generator -->
    <rng model='virtio'>
      <backend model='random'>/dev/urandom</backend>
    </rng>
  </devices>
{{if .PasstSocket}}
  <!-- Isolated networking via passt in network namespace.
       Provides host-level network isolation with blackhole routes for private IPs.
       The passt socket is in an isolated namespace that blocks RFC 1918 ranges. -->
  <qemu:commandline>
    <qemu:arg value='-netdev'/>
    <qemu:arg value='stream,id=net0,server=off,addr.type=unix,addr.path={{.PasstSocket}}'/>
    <qemu:arg value='-device'/>
    <qemu:arg value='virtio-net-pci,netdev=net0,bus=pci.0,addr=0x10'/>
  </qemu:commandline>
{{else if and (not .NetworkName) (gt .SSHPort 0)}}
  <!-- User-mode networking with SSH port forwarding via QEMU command line.
       We must specify explicit PCI address (0x10) because qemu:commandline
       bypasses libvirt's PCI address management. Without it, QEMU would
       auto-assign a low slot (e.g., 0x2) that conflicts with libvirt-managed
       devices like virtio-blk. Slot 0x10 is high enough to avoid conflicts
       with libvirt's auto-assignment which starts from low numbers. -->
  <qemu:commandline>
    <qemu:arg value='-netdev'/>
    <qemu:arg value='user,id=net0,hostfwd=tcp:127.0.0.1:{{.SSHPort}}-:22'/>
    <qemu:arg value='-device'/>
    <qemu:arg value='virtio-net-pci,netdev=net0,bus=pci.0,addr=0x10'/>
  </qemu:commandline>
{{end}}
</domain>`

// GenerateDomainXML generates libvirt domain XML from config
func GenerateDomainXML(cfg *DomainConfig) (string, error) {
	tmpl, err := template.New("domain").Parse(domainXMLTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", err
	}

	return buf.String(), nil
}
