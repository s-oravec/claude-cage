package network

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"text/template"

	"github.com/s-oravec/cage/internal/mode"
)

// virsh runs a virsh command against the libvirt URI for the current mode.
func virsh(args ...string) *exec.Cmd {
	fullArgs := append([]string{"-c", mode.Current().URI()}, args...)
	return exec.Command("virsh", fullArgs...)
}

// NetworkConfig holds configuration for a libvirt network
type NetworkConfig struct {
	CageName   string
	BridgeName string
	IPAddress  string
	Netmask    string
	DHCPStart  string
	DHCPEnd    string
}

const networkXMLTemplate = `<network>
  <name>{{.BridgeName}}</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='{{.BridgeName}}' stp='on' delay='0'/>
  <ip address='{{.IPAddress}}' netmask='{{.Netmask}}'>
    <dhcp>
      <range start='{{.DHCPStart}}' end='{{.DHCPEnd}}'/>
    </dhcp>
  </ip>
</network>`

// NewNetworkConfig creates a new NetworkConfig with defaults
func NewNetworkConfig(cageName string) *NetworkConfig {
	return &NetworkConfig{
		CageName:   cageName,
		BridgeName: BridgeName(cageName),
		IPAddress:  "192.168.100.1",
		Netmask:    "255.255.255.0",
		DHCPStart:  "192.168.100.2",
		DHCPEnd:    "192.168.100.254",
	}
}

// Validate checks if the network configuration is valid
func (c *NetworkConfig) Validate() error {
	if c.CageName == "" {
		return errors.New("cage name is required")
	}
	if c.BridgeName == "" {
		return errors.New("bridge name is required")
	}
	if len(c.BridgeName) > 15 {
		return fmt.Errorf("bridge name too long: %d > 15 chars", len(c.BridgeName))
	}
	return nil
}

// GenerateNetworkXML generates libvirt network XML
func GenerateNetworkXML(cfg *NetworkConfig) string {
	tmpl, err := template.New("network").Parse(networkXMLTemplate)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return ""
	}

	return buf.String()
}

// CreateNetwork creates a libvirt network for a cage
func CreateNetwork(cageName string) error {
	cfg := NewNetworkConfig(cageName)
	if err := cfg.Validate(); err != nil {
		return err
	}

	xml := GenerateNetworkXML(cfg)

	// Define network
	cmd := virsh("net-define", "/dev/stdin")
	cmd.Stdin = bytes.NewBufferString(xml)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to define network: %s", string(output))
	}

	// Start network
	cmd = virsh("net-start", cfg.BridgeName)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Cleanup on failure
		virsh("net-undefine", cfg.BridgeName).Run()
		return fmt.Errorf("failed to start network: %s", string(output))
	}

	return nil
}

// DestroyNetwork removes a libvirt network for a cage
func DestroyNetwork(cageName string) error {
	bridgeName := BridgeName(cageName)

	// Destroy (stop) network
	virsh("net-destroy", bridgeName).Run() // Ignore errors (may not be running)

	// Undefine network
	virsh("net-undefine", bridgeName).Run() // Ignore errors (may not exist)

	return nil
}

// NetworkExists checks if a network exists
func NetworkExists(cageName string) bool {
	bridgeName := BridgeName(cageName)
	return virsh("net-info", bridgeName).Run() == nil
}
