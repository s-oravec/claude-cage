package libvirt

import (
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps libvirt operations using virsh CLI
// Using CLI instead of CGO bindings for simpler deployment
type Client struct {
	uri string
}

// NewClient creates a new libvirt client
func NewClient() *Client {
	return &Client{
		uri: "qemu:///session",
	}
}

// virsh runs a virsh command and returns output
func (c *Client) virsh(args ...string) (string, error) {
	fullArgs := append([]string{"-c", c.uri}, args...)
	cmd := exec.Command("virsh", fullArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// DefineDomain defines a domain from XML
func (c *Client) DefineDomain(xml string) error {
	// Write XML to temp file
	tmpFile := "/tmp/cage-domain.xml"
	if err := exec.Command("bash", "-c", fmt.Sprintf("cat > %s << 'EOFXML'\n%s\nEOFXML", tmpFile, xml)).Run(); err != nil {
		return fmt.Errorf("failed to write domain XML: %w", err)
	}

	out, err := c.virsh("define", tmpFile)
	if err != nil {
		return fmt.Errorf("failed to define domain: %s", out)
	}
	return nil
}

// IsDomainActive checks if a domain is currently running
func (c *Client) IsDomainActive(name string) (bool, error) {
	out, err := c.virsh("domstate", "cage-"+name)
	if err != nil {
		// Domain might not exist
		return false, err
	}
	state := strings.TrimSpace(out)
	return state == "running", nil
}

// StartDomain starts a defined domain
func (c *Client) StartDomain(name string) error {
	out, err := c.virsh("start", "cage-"+name)
	if err != nil {
		return fmt.Errorf("failed to start domain: %s", out)
	}
	return nil
}

// StopDomain stops a domain (graceful shutdown)
func (c *Client) StopDomain(name string) error {
	out, err := c.virsh("shutdown", "cage-"+name)
	if err != nil {
		return fmt.Errorf("failed to shutdown domain: %s", out)
	}
	return nil
}

// DestroyDomain forcefully stops a domain
func (c *Client) DestroyDomain(name string) error {
	out, err := c.virsh("destroy", "cage-"+name)
	if err != nil {
		return fmt.Errorf("failed to destroy domain: %s", out)
	}
	return nil
}

// UndefineDomain removes a domain definition
func (c *Client) UndefineDomain(name string) error {
	out, err := c.virsh("undefine", "cage-"+name)
	if err != nil {
		return fmt.Errorf("failed to undefine domain: %s", out)
	}
	return nil
}

// DomainExists checks if a domain exists
func (c *Client) DomainExists(name string) bool {
	_, err := c.virsh("dominfo", "cage-"+name)
	return err == nil
}

// DomainIsRunning checks if a domain is running
func (c *Client) DomainIsRunning(name string) bool {
	out, err := c.virsh("domstate", "cage-"+name)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "running"
}

// GetDomainIP gets the IP address of a running domain
func (c *Client) GetDomainIP(name string) (string, error) {
	out, err := c.virsh("domifaddr", "cage-"+name)
	if err != nil {
		return "", err
	}

	// Parse output to find IP
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 && strings.Contains(fields[3], "/") {
			ip := strings.Split(fields[3], "/")[0]
			return ip, nil
		}
	}

	return "", fmt.Errorf("no IP found for domain")
}

// ListRunningDomains returns names of running cage domains
func (c *Client) ListRunningDomains() ([]string, error) {
	out, err := c.virsh("list", "--name")
	if err != nil {
		return nil, err
	}

	var cages []string
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if strings.HasPrefix(name, "cage-") {
			cages = append(cages, strings.TrimPrefix(name, "cage-"))
		}
	}
	return cages, nil
}

// RedefineDomain undefines and redefines a domain with new XML
// The domain must be stopped (inactive) for this to work.
func (c *Client) RedefineDomain(name, xml string) error {
	// Check if domain is running - can't redefine a running domain
	if c.DomainIsRunning(name) {
		return fmt.Errorf("domain cage-%s is running, cannot redefine", name)
	}

	// Undefine the old domain
	if err := c.UndefineDomain(name); err != nil {
		return fmt.Errorf("failed to undefine domain: %w", err)
	}

	// Define the new domain
	if err := c.DefineDomain(xml); err != nil {
		return fmt.Errorf("failed to define new domain: %w", err)
	}

	return nil
}
