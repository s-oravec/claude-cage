package network

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IsolationConfig holds configuration for network isolation
type IsolationConfig struct {
	CageName       string
	SocketPath     string   // Path for passt socket
	BlockedSubnets []string // Subnets to block (RFC 1918, etc.)
}

// IsolatedNetwork represents a running isolated network namespace with passt
type IsolatedNetwork struct {
	Namespace    string // Network namespace name
	SocketPath   string // Path to passt socket
	PasstPID     int    // PID of passt process
	VethHost     string // Host side of veth pair
	VethNS       string // Namespace side of veth pair
	NamespaceIP  string // IP address in namespace
	HostIP       string // IP address on host side
	OutInterface string // Outbound interface for NAT
}

// GetDefaultInterface returns the interface with the default route
func GetDefaultInterface() string {
	cmd := exec.Command("ip", "route", "show", "default")
	out, _ := cmd.Output()
	parts := strings.Fields(string(out))
	for i, p := range parts {
		if p == "dev" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "eth0"
}

// SetupIsolatedNetwork creates a network namespace with passt for isolated networking
func SetupIsolatedNetwork(cfg *IsolationConfig) (*IsolatedNetwork, error) {
	if cfg.BlockedSubnets == nil {
		cfg.BlockedSubnets = DefaultBlockedSubnets()
	}

	nsName := fmt.Sprintf("cage-%s", cfg.CageName)
	vethHost := fmt.Sprintf("cv-%s-h", shortName(cfg.CageName, 10))
	vethNS := fmt.Sprintf("cv-%s-n", shortName(cfg.CageName, 10))
	outIface := GetDefaultInterface()
	
	// Use a unique /30 subnet for this cage
	hostIP := "192.168.250.1"
	nsIP := "192.168.250.2"

	// Create network namespace
	if err := createNetworkNamespace(nsName); err != nil {
		return nil, fmt.Errorf("failed to create network namespace: %w", err)
	}

	// Create veth pair
	if err := createVethPair(vethHost, vethNS, nsName); err != nil {
		deleteNetworkNamespace(nsName)
		return nil, fmt.Errorf("failed to create veth pair: %w", err)
	}

	// Configure IP addresses
	if err := configureVeth(vethHost, vethNS, nsName, hostIP, nsIP); err != nil {
		deleteVethPair(vethHost)
		deleteNetworkNamespace(nsName)
		return nil, fmt.Errorf("failed to configure veth: %w", err)
	}

	// Set up NAT on host for namespace traffic
	if err := setupNAT(vethHost, nsIP, outIface); err != nil {
		deleteVethPair(vethHost)
		deleteNetworkNamespace(nsName)
		return nil, fmt.Errorf("failed to setup NAT: %w", err)
	}

	// Add default route in namespace via host
	if err := addDefaultRoute(nsName, hostIP); err != nil {
		cleanupNAT(vethHost, nsIP, outIface)
		deleteVethPair(vethHost)
		deleteNetworkNamespace(nsName)
		return nil, fmt.Errorf("failed to add default route: %w", err)
	}

	// Add blackhole routes for blocked subnets in the namespace
	// These override the default route for private IP ranges
	for _, subnet := range cfg.BlockedSubnets {
		if err := addBlackholeRoute(nsName, subnet); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to add blackhole route for %s: %v\n", subnet, err)
		}
	}

	// Start passt in the namespace
	socketPath := cfg.SocketPath
	if socketPath == "" {
		socketPath = filepath.Join(os.TempDir(), fmt.Sprintf("cage-%s-passt.socket", cfg.CageName))
	}

	pid, err := startPasstInNamespace(nsName, socketPath)
	if err != nil {
		cleanupNAT(vethHost, nsIP, outIface)
		deleteVethPair(vethHost)
		deleteNetworkNamespace(nsName)
		return nil, fmt.Errorf("failed to start passt: %w", err)
	}

	// Wait for socket to be ready
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return &IsolatedNetwork{
		Namespace:    nsName,
		SocketPath:   socketPath,
		PasstPID:     pid,
		VethHost:     vethHost,
		VethNS:       vethNS,
		NamespaceIP:  nsIP,
		HostIP:       hostIP,
		OutInterface: outIface,
	}, nil
}

// Cleanup removes the isolated network
func (n *IsolatedNetwork) Cleanup() error {
	// Kill passt process
	if n.PasstPID > 0 {
		syscall.Kill(n.PasstPID, syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		syscall.Kill(n.PasstPID, syscall.SIGKILL)
	}

	// Remove socket
	os.Remove(n.SocketPath)

	// Cleanup NAT
	cleanupNAT(n.VethHost, n.NamespaceIP, n.OutInterface)

	// Delete veth (automatically removes both ends)
	deleteVethPair(n.VethHost)

	// Delete namespace
	return deleteNetworkNamespace(n.Namespace)
}

// shortName truncates a name to max length
func shortName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	return name[:max]
}

// createNetworkNamespace creates a new network namespace
func createNetworkNamespace(name string) error {
	cmd := exec.Command("ip", "netns", "add", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "File exists") {
			return nil
		}
		return fmt.Errorf("ip netns add failed: %s", string(out))
	}

	// Bring up loopback in namespace
	exec.Command("ip", "netns", "exec", name, "ip", "link", "set", "lo", "up").Run()

	return nil
}

// deleteNetworkNamespace removes a network namespace
func deleteNetworkNamespace(name string) error {
	exec.Command("ip", "netns", "delete", name).Run()
	return nil
}

// createVethPair creates a veth pair and moves one end to namespace
func createVethPair(hostEnd, nsEnd, nsName string) error {
	// Delete existing if any
	exec.Command("ip", "link", "delete", hostEnd).Run()

	// Create veth pair
	cmd := exec.Command("ip", "link", "add", hostEnd, "type", "veth", "peer", "name", nsEnd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create veth: %s", string(out))
	}

	// Move nsEnd to namespace
	cmd = exec.Command("ip", "link", "set", nsEnd, "netns", nsName)
	if out, err := cmd.CombinedOutput(); err != nil {
		exec.Command("ip", "link", "delete", hostEnd).Run()
		return fmt.Errorf("failed to move veth to namespace: %s", string(out))
	}

	return nil
}

// deleteVethPair removes a veth pair
func deleteVethPair(hostEnd string) {
	exec.Command("ip", "link", "delete", hostEnd).Run()
}

// configureVeth sets up IP addresses on veth interfaces
func configureVeth(hostEnd, nsEnd, nsName, hostIP, nsIP string) error {
	// Configure host side
	cmds := [][]string{
		{"ip", "addr", "add", hostIP + "/30", "dev", hostEnd},
		{"ip", "link", "set", hostEnd, "up"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			// Ignore "File exists" for addr add
			if !strings.Contains(string(out), "File exists") {
				return fmt.Errorf("command %v failed: %s", args, string(out))
			}
		}
	}

	// Configure namespace side
	nsCmds := [][]string{
		{"ip", "addr", "add", nsIP + "/30", "dev", nsEnd},
		{"ip", "link", "set", nsEnd, "up"},
	}
	for _, args := range nsCmds {
		fullArgs := append([]string{"netns", "exec", nsName}, args...)
		if out, err := exec.Command("ip", fullArgs...).CombinedOutput(); err != nil {
			if !strings.Contains(string(out), "File exists") {
				return fmt.Errorf("ns command %v failed: %s", args, string(out))
			}
		}
	}

	return nil
}

// setupNAT configures NAT for traffic from namespace
func setupNAT(vethHost, nsIP, outIface string) error {
	// Enable IP forwarding
	exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	// Add MASQUERADE rule (INSERT at beginning to take precedence)
	cmd := exec.Command("iptables", "-t", "nat", "-I", "POSTROUTING", "1",
		"-s", nsIP+"/32", "-o", outIface, "-j", "MASQUERADE")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add NAT rule: %s", string(out))
	}

	// Allow forwarding (INSERT at beginning)
	exec.Command("iptables", "-I", "FORWARD", "1", "-i", vethHost, "-o", outIface, "-j", "ACCEPT").Run()
	exec.Command("iptables", "-I", "FORWARD", "2", "-i", outIface, "-o", vethHost, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run()

	return nil
}

// cleanupNAT removes NAT rules
func cleanupNAT(vethHost, nsIP, outIface string) {
	exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-s", nsIP+"/32", "-o", outIface, "-j", "MASQUERADE").Run()
	exec.Command("iptables", "-D", "FORWARD", "-i", vethHost, "-o", outIface, "-j", "ACCEPT").Run()
	exec.Command("iptables", "-D", "FORWARD", "-i", outIface, "-o", vethHost, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run()
}

// addDefaultRoute adds default route in namespace
func addDefaultRoute(nsName, gateway string) error {
	cmd := exec.Command("ip", "netns", "exec", nsName, "ip", "route", "add", "default", "via", gateway)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "File exists") {
			return nil
		}
		return fmt.Errorf("failed to add default route: %s", string(out))
	}
	return nil
}

// addBlackholeRoute adds an unreachable route in a namespace
func addBlackholeRoute(nsName, subnet string) error {
	cmd := exec.Command("ip", "netns", "exec", nsName, "ip", "route", "add", "blackhole", subnet)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "File exists") {
			return nil
		}
		return fmt.Errorf("failed to add blackhole route: %s", string(out))
	}
	return nil
}

// startPasstInNamespace starts passt inside a network namespace
func startPasstInNamespace(nsName, socketPath string) (int, error) {
	os.Remove(socketPath)

	// Start passt in the namespace
	cmd := exec.Command("ip", "netns", "exec", nsName,
		"passt",
		"--socket", socketPath,
		"--foreground",
	)

	// Redirect output to prevent blocking
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start passt: %w", err)
	}

	return cmd.Process.Pid, nil
}

// VerifyNamespaceIsolation verifies that the namespace cannot reach private IPs but can reach internet
func VerifyNamespaceIsolation(nsName string) ([]VerificationResult, error) {
	var results []VerificationResult

	// Test internet connectivity (use TCP since ICMP often blocked)
	result := VerificationResult{
		TestName: "Internet access (TCP)",
	}
	cmd := exec.Command("ip", "netns", "exec", nsName, "curl", "-s", "--connect-timeout", "5", "http://example.com")
	if _, err := cmd.CombinedOutput(); err == nil {
		result.Passed = true
		result.Message = "OK"
	} else {
		result.Passed = false
		result.Message = "FAILED (no internet access)"
	}
	results = append(results, result)

	// Test blocked subnets
	testIPs := []struct {
		IP   string
		Name string
	}{
		{"10.0.0.1", "10.0.0.0/8 blocked"},
		{"172.16.0.1", "172.16.0.0/12 blocked"},
		{"192.168.1.1", "192.168.0.0/16 blocked"},
		{"169.254.1.1", "169.254.0.0/16 blocked"},
	}

	for _, test := range testIPs {
		result := VerificationResult{
			TestName: test.Name,
		}
		
		// Try TCP connection (more reliable than ping in nested NAT)
		cmd := exec.Command("ip", "netns", "exec", nsName, "timeout", "2", "nc", "-zv", test.IP, "80")
		_, err := cmd.CombinedOutput()

		if err != nil {
			result.Passed = true
			result.Message = "OK (correctly blocked at host level)"
		} else {
			result.Passed = false
			result.Message = "SECURITY ISSUE: host namespace can reach this IP!"
		}
		results = append(results, result)
	}

	return results, nil
}

// GetPasstPID reads the PID of a running passt process
func GetPasstPID(socketPath string) (int, error) {
	pidFile := socketPath + ".pid"
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// IsNamespaceActive checks if a network namespace exists
func IsNamespaceActive(nsName string) bool {
	cmd := exec.Command("ip", "netns", "list")
	out, _ := cmd.Output()
	return strings.Contains(string(out), nsName)
}
