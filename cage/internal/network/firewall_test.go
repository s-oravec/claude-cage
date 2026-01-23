package network

import (
	"strings"
	"testing"
)

func TestGenerateFirewallRules(t *testing.T) {
	cfg := &FirewallConfig{
		BridgeName: "cage-test",
		BlockedInterfaces: []string{"tun+", "tailscale+", "wg+"},
		BlockedSubnets: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"169.254.0.0/16",
		},
		AllowedDNS: []string{"1.1.1.1", "8.8.8.8"},
	}

	rules := GenerateFirewallRules(cfg)

	// Should create chain
	assertContainsRule(t, rules, []string{"-N", "CAGE-FILTER"})

	// Should allow established connections
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"})

	// Should block VPN interfaces
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-o", "tun+", "-j", "DROP"})
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-o", "tailscale+", "-j", "DROP"})
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-o", "wg+", "-j", "DROP"})

	// Should block RFC 1918 subnets
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-d", "10.0.0.0/8", "-j", "DROP"})
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-d", "172.16.0.0/12", "-j", "DROP"})
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-d", "192.168.0.0/16", "-j", "DROP"})
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-d", "169.254.0.0/16", "-j", "DROP"})

	// Should allow DNS only to configured servers
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-p", "udp", "--dport", "53", "-d", "1.1.1.1", "-j", "ACCEPT"})
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-p", "udp", "--dport", "53", "-d", "8.8.8.8", "-j", "ACCEPT"})

	// Should drop other DNS
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-p", "udp", "--dport", "53", "-j", "DROP"})

	// Should allow HTTP/HTTPS
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-p", "tcp", "--dport", "80", "-j", "ACCEPT"})
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-p", "tcp", "--dport", "443", "-j", "ACCEPT"})

	// Should allow other traffic at the end
	assertContainsRule(t, rules, []string{"-A", "CAGE-FILTER", "-j", "ACCEPT"})

	// Should apply chain to bridge
	assertContainsRule(t, rules, []string{"-A", "FORWARD", "-i", "cage-test", "-j", "CAGE-FILTER"})
}

func TestGenerateDNATRule(t *testing.T) {
	rule := GenerateDNATRule("cage-test", "1.1.1.1")

	expected := []string{"-t", "nat", "-A", "PREROUTING", "-i", "cage-test", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "1.1.1.1:53"}
	if !rulesEqual(rule, expected) {
		t.Errorf("expected DNAT rule %v, got %v", expected, rule)
	}
}

func TestGenerateCleanupRules(t *testing.T) {
	rules := GenerateCleanupRules("cage-test", "1.1.1.1")

	// Should remove FORWARD rule
	assertContainsRule(t, rules, []string{"-D", "FORWARD", "-i", "cage-test", "-j", "CAGE-FILTER"})

	// Should remove DNAT rule
	assertContainsRule(t, rules, []string{"-t", "nat", "-D", "PREROUTING", "-i", "cage-test", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "1.1.1.1:53"})
}

func TestBridgeName(t *testing.T) {
	tests := []struct {
		cageName string
		expected string
	}{
		{"test", "cage-test"},
		{"myproject", "cage-myproject"}, // 14 chars, fits
		{"verylongcagename", "cage-verylongca"}, // truncated to 15 chars total
		{"a", "cage-a"},
	}

	for _, tt := range tests {
		t.Run(tt.cageName, func(t *testing.T) {
			result := BridgeName(tt.cageName)
			if result != tt.expected {
				t.Errorf("BridgeName(%q) = %q, want %q", tt.cageName, result, tt.expected)
			}
			if len(result) > 15 {
				t.Errorf("BridgeName(%q) = %q, length %d > 15", tt.cageName, result, len(result))
			}
		})
	}
}

func assertContainsRule(t *testing.T, rules [][]string, expected []string) {
	t.Helper()
	for _, rule := range rules {
		if rulesEqual(rule, expected) {
			return
		}
	}
	t.Errorf("rules do not contain expected rule: %v", expected)
}

func rulesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDefaultBlockedInterfaces(t *testing.T) {
	interfaces := DefaultBlockedInterfaces()

	if len(interfaces) < 3 {
		t.Error("should have at least 3 blocked interfaces")
	}

	// Check VPN interfaces
	found := make(map[string]bool)
	for _, iface := range interfaces {
		if strings.HasPrefix(iface, "tun") || strings.HasPrefix(iface, "tailscale") || strings.HasPrefix(iface, "wg") {
			found[iface] = true
		}
	}

	if !found["tun+"] {
		t.Error("should block tun+")
	}
	if !found["tailscale+"] {
		t.Error("should block tailscale+")
	}
	if !found["wg+"] {
		t.Error("should block wg+")
	}
}

func TestDefaultBlockedSubnets(t *testing.T) {
	subnets := DefaultBlockedSubnets()

	if len(subnets) < 4 {
		t.Error("should have at least 4 blocked subnets")
	}

	// Check RFC 1918 and link-local
	required := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
	}

	for _, req := range required {
		found := false
		for _, subnet := range subnets {
			if subnet == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("should block subnet %s", req)
		}
	}
}
