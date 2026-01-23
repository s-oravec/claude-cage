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
	VirtiofsSocket string // optional: path to virtiofsd socket
	SSHPort        int    // optional: host port for SSH forwarding (user-mode only)
}

const domainXMLTemplate = `<domain type='kvm'{{if and (not .NetworkName) (gt .SSHPort 0)}} xmlns:qemu='http://libvirt.org/schemas/domain/qemu/1.0'{{end}}>
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
{{if .VirtiofsSocket}}
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

    <!-- Cloud-init ISO -->
    <disk type='file' device='cdrom'>
      <source file='{{.CloudInitISO}}'/>
      <target dev='sda' bus='sata'/>
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
    <!-- Virtio-fs shared directory -->
    <filesystem type='mount' accessmode='passthrough'>
      <driver type='virtiofs' queue='1024'/>
      <source socket='{{.VirtiofsSocket}}'/>
      <target dir='workspace'/>
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
{{if and (not .NetworkName) (gt .SSHPort 0)}}
  <qemu:commandline>
    <qemu:arg value='-netdev'/>
    <qemu:arg value='user,id=net0,hostfwd=tcp:127.0.0.1:{{.SSHPort}}-:22'/>
    <qemu:arg value='-device'/>
    <qemu:arg value='virtio-net-pci,netdev=net0'/>
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
