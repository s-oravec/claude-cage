package libvirt

import (
	"bytes"
	"text/template"
)

// DomainConfig holds configuration for a libvirt domain
type DomainConfig struct {
	Name         string
	MemoryMB     int
	VCPU         int
	DiskPath     string
	CloudInitISO string
	NetworkName  string
}

const domainXMLTemplate = `<domain type='kvm'>
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
    <interface type='network'>
      <source network='{{.NetworkName}}'/>
      <model type='virtio'/>
    </interface>

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
