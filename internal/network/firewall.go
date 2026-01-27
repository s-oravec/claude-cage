package network

import (
	"fmt"
	"os/exec"
)

// FirewallConfig holds configuration for iptables rules
type FirewallConfig struct {
	BridgeName        string
	BlockedInterfaces []string
	BlockedSubnets    []string
	AllowedDNS        []string
}

// BridgeName generates a bridge name for a cage (max 15 chars for Linux)
func BridgeName(cageName string) string {
	prefix := "cage-"
	maxNameLen := 15 - len(prefix) // 10 chars for cage name
	if len(cageName) > maxNameLen {
		cageName = cageName[:maxNameLen]
	}
	return prefix + cageName
}

// DefaultBlockedInterfaces returns the default VPN interfaces to block
func DefaultBlockedInterfaces() []string {
	return []string{
		"tun+",       // OpenVPN, generic TUN
		"tailscale+", // Tailscale
		"wg+",        // WireGuard
	}
}

// DefaultBlockedSubnets returns the default RFC 1918 and link-local subnets to block
func DefaultBlockedSubnets() []string {
	return []string{
		"10.0.0.0/8",     // RFC 1918 Class A
		"172.16.0.0/12",  // RFC 1918 Class B
		"192.168.0.0/16", // RFC 1918 Class C
		"169.254.0.0/16", // Link-local
	}
}

// DefaultAllowedDNS returns the default DNS servers to allow
func DefaultAllowedDNS() []string {
	return []string{
		"1.1.1.1", // Cloudflare
		"8.8.8.8", // Google
	}
}

// GenerateFirewallRules generates iptables rules for a cage
func GenerateFirewallRules(cfg *FirewallConfig) [][]string {
	var rules [][]string

	// Create chain if not exists
	rules = append(rules, []string{"-N", "CAGE-FILTER"})

	// Allow established connections
	rules = append(rules, []string{"-A", "CAGE-FILTER", "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"})

	// Block VPN interfaces
	for _, iface := range cfg.BlockedInterfaces {
		rules = append(rules, []string{"-A", "CAGE-FILTER", "-o", iface, "-j", "DROP"})
	}

	// Block private subnets
	for _, subnet := range cfg.BlockedSubnets {
		rules = append(rules, []string{"-A", "CAGE-FILTER", "-d", subnet, "-j", "DROP"})
	}

	// Allow DNS only to configured servers
	for _, dns := range cfg.AllowedDNS {
		rules = append(rules, []string{"-A", "CAGE-FILTER", "-p", "udp", "--dport", "53", "-d", dns, "-j", "ACCEPT"})
	}
	// Drop other DNS
	rules = append(rules, []string{"-A", "CAGE-FILTER", "-p", "udp", "--dport", "53", "-j", "DROP"})

	// Allow HTTP/HTTPS
	rules = append(rules, []string{"-A", "CAGE-FILTER", "-p", "tcp", "--dport", "80", "-j", "ACCEPT"})
	rules = append(rules, []string{"-A", "CAGE-FILTER", "-p", "tcp", "--dport", "443", "-j", "ACCEPT"})

	// Allow other traffic (after blocking private subnets)
	rules = append(rules, []string{"-A", "CAGE-FILTER", "-j", "ACCEPT"})

	// Apply chain to bridge
	rules = append(rules, []string{"-A", "FORWARD", "-i", cfg.BridgeName, "-j", "CAGE-FILTER"})

	return rules
}

// GenerateDNATRule generates a DNAT rule for DNS enforcement
func GenerateDNATRule(bridgeName, dnsServer string) []string {
	return []string{
		"-t", "nat", "-A", "PREROUTING",
		"-i", bridgeName,
		"-p", "udp", "--dport", "53",
		"-j", "DNAT", "--to-destination", dnsServer + ":53",
	}
}

// GenerateCleanupRules generates rules to remove firewall configuration
func GenerateCleanupRules(bridgeName, dnsServer string) [][]string {
	return [][]string{
		// Remove FORWARD rule
		{"-D", "FORWARD", "-i", bridgeName, "-j", "CAGE-FILTER"},
		// Remove DNAT rule
		{"-t", "nat", "-D", "PREROUTING", "-i", bridgeName, "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", dnsServer + ":53"},
	}
}

// SetupFirewall applies firewall rules for a cage
func SetupFirewall(cageName string, cfg *FirewallConfig) error {
	if cfg == nil {
		cfg = &FirewallConfig{
			BridgeName:        BridgeName(cageName),
			BlockedInterfaces: DefaultBlockedInterfaces(),
			BlockedSubnets:    DefaultBlockedSubnets(),
			AllowedDNS:        DefaultAllowedDNS(),
		}
	}

	rules := GenerateFirewallRules(cfg)

	for _, rule := range rules {
		cmd := exec.Command("iptables", rule...)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Ignore "chain already exists" error
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == 1 && len(rule) > 0 && rule[0] == "-N" {
					// Chain already exists, continue
					continue
				}
			}
			return fmt.Errorf("iptables rule %v failed: %s", rule, string(output))
		}
	}

	return nil
}

// SetupDNAT sets up DNS DNAT for a cage
func SetupDNAT(cageName, dnsServer string) error {
	bridgeName := BridgeName(cageName)
	rule := GenerateDNATRule(bridgeName, dnsServer)

	cmd := exec.Command("iptables", rule...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables DNAT rule failed: %s", string(output))
	}

	return nil
}

// CleanupFirewall removes firewall rules for a cage
func CleanupFirewall(cageName, dnsServer string) error {
	bridgeName := BridgeName(cageName)
	rules := GenerateCleanupRules(bridgeName, dnsServer)

	for _, rule := range rules {
		cmd := exec.Command("iptables", rule...)
		// Ignore errors on cleanup (rules may not exist)
		cmd.Run()
	}

	return nil
}
