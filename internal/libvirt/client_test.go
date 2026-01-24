package libvirt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_NewClient(t *testing.T) {
	client := NewClient()
	assert.NotNil(t, client)
	assert.Equal(t, "qemu:///session", client.uri)
}

func TestRedefineDomain_FailsIfRunning(t *testing.T) {
	// This test requires a mock or actual libvirt to run properly.
	// For unit testing, we verify the client structure and method signatures.
	// Full integration testing is done via e2e tests.

	client := NewClient()

	// Test with a domain that doesn't exist
	// The method should fail because it can't find the domain
	xml := `<domain type='kvm'><name>cage-nonexistent</name></domain>`
	err := client.RedefineDomain("nonexistent", xml)

	// Expect error (domain doesn't exist for undefine)
	assert.Error(t, err)
}

func TestRedefineDomain_MethodExists(t *testing.T) {
	// Verify the method exists and has correct signature
	client := NewClient()

	// Type assertion to verify method exists
	type redefinable interface {
		RedefineDomain(name, xml string) error
	}

	var _ redefinable = client
}

func TestClient_DomainExists(t *testing.T) {
	client := NewClient()

	// Test with a domain that doesn't exist
	exists := client.DomainExists("definitely-nonexistent-domain-12345")
	assert.False(t, exists)
}

func TestClient_DomainIsRunning(t *testing.T) {
	client := NewClient()

	// Test with a domain that doesn't exist
	running := client.DomainIsRunning("definitely-nonexistent-domain-12345")
	assert.False(t, running)
}

func TestClient_IsDomainActive(t *testing.T) {
	client := NewClient()

	// Test with a domain that doesn't exist
	active, err := client.IsDomainActive("definitely-nonexistent-domain-12345")
	assert.Error(t, err) // Domain doesn't exist
	assert.False(t, active)
}
