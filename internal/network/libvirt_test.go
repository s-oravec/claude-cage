package network

import (
	"strings"
	"testing"
)

func TestGenerateNetworkXML(t *testing.T) {
	cfg := &NetworkConfig{
		CageName:    "test",
		BridgeName:  "cage-test",
		IPAddress:   "192.168.100.1",
		Netmask:     "255.255.255.0",
		DHCPStart:   "192.168.100.2",
		DHCPEnd:     "192.168.100.254",
	}

	xml := GenerateNetworkXML(cfg)

	// Check network name
	if !strings.Contains(xml, "<name>cage-test</name>") {
		t.Error("XML should contain network name")
	}

	// Check NAT forward
	if !strings.Contains(xml, "<forward mode='nat'>") {
		t.Error("XML should contain NAT forward")
	}

	// Check bridge name
	if !strings.Contains(xml, "name='cage-test'") {
		t.Error("XML should contain bridge name")
	}

	// Check IP configuration
	if !strings.Contains(xml, "address='192.168.100.1'") {
		t.Error("XML should contain IP address")
	}

	// Check DHCP range
	if !strings.Contains(xml, "start='192.168.100.2'") {
		t.Error("XML should contain DHCP start")
	}
	if !strings.Contains(xml, "end='192.168.100.254'") {
		t.Error("XML should contain DHCP end")
	}
}

func TestNewNetworkConfig(t *testing.T) {
	cfg := NewNetworkConfig("myproject")

	if cfg.CageName != "myproject" {
		t.Errorf("CageName = %q, want %q", cfg.CageName, "myproject")
	}

	if cfg.BridgeName == "" {
		t.Error("BridgeName should not be empty")
	}

	// Bridge name should be max 15 chars
	if len(cfg.BridgeName) > 15 {
		t.Errorf("BridgeName length %d > 15", len(cfg.BridgeName))
	}

	if cfg.IPAddress == "" {
		t.Error("IPAddress should have default")
	}

	if cfg.Netmask == "" {
		t.Error("Netmask should have default")
	}
}

func TestNetworkConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *NetworkConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &NetworkConfig{
				CageName:   "test",
				BridgeName: "cage-test",
				IPAddress:  "192.168.100.1",
				Netmask:    "255.255.255.0",
				DHCPStart:  "192.168.100.2",
				DHCPEnd:    "192.168.100.254",
			},
			wantErr: false,
		},
		{
			name: "empty cage name",
			cfg: &NetworkConfig{
				CageName:   "",
				BridgeName: "cage-test",
				IPAddress:  "192.168.100.1",
			},
			wantErr: true,
		},
		{
			name: "empty bridge name",
			cfg: &NetworkConfig{
				CageName:   "test",
				BridgeName: "",
				IPAddress:  "192.168.100.1",
			},
			wantErr: true,
		},
		{
			name: "bridge name too long",
			cfg: &NetworkConfig{
				CageName:   "test",
				BridgeName: "this-is-way-too-long-for-bridge",
				IPAddress:  "192.168.100.1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
