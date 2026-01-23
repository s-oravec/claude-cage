# Fáza 07: Network Security

Sieťová izolácia - blokáda VPN a privátnych sietí.

## Cieľ

- CAGE-FILTER iptables chain
- Blokáda VPN interfaces (tun+, tailscale+, wg+)
- Blokáda RFC 1918 subnets
- DNS enforcement
- Povolenie verejného internetu

## Závisí na

- Fáza 06 (virtiofs)

## Kritické pre bezpečnosť

Toto je **kritická fáza** pre security. Po jej dokončení je systém bezpečný pre yolo mode.

## Implementácia

### 1. Network types

```go
// internal/network/types.go
type NetworkConfig struct {
    BlockedInterfaces []string // tun+, tailscale+, wg+
    BlockedSubnets    []string // 10.0.0.0/8, ...
    DNS               []string // 1.1.1.1, 8.8.8.8
    PortBind          string   // 127.0.0.1
}
```

### 2. Libvirt network

```go
// internal/network/libvirt.go
func CreateNetwork(cageName string) error {
    // Vytvorí NAT network pre cage
    xml := fmt.Sprintf(`
<network>
  <name>cage-%s</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='cage-%s' stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>`, cageName, cageName[:8]) // bridge name max 15 chars

    client := libvirt.NewClient()
    network, err := client.NetworkDefineXML(xml)
    if err != nil {
        return err
    }
    return network.Create()
}
```

### 3. CAGE-FILTER iptables chain

```go
// internal/network/firewall.go
func SetupFirewall(cageName string, cfg NetworkConfig) error {
    bridgeName := fmt.Sprintf("cage-%s", cageName[:8])

    rules := [][]string{
        // Create chain if not exists
        {"-N", "CAGE-FILTER"},

        // Allow established connections
        {"-A", "CAGE-FILTER", "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
    }

    // Block VPN interfaces
    for _, iface := range cfg.BlockedInterfaces {
        rules = append(rules, []string{
            "-A", "CAGE-FILTER", "-o", iface, "-j", "DROP",
        })
    }

    // Block private subnets
    for _, subnet := range cfg.BlockedSubnets {
        rules = append(rules, []string{
            "-A", "CAGE-FILTER", "-d", subnet, "-j", "DROP",
        })
    }

    // Allow DNS only to configured servers
    for _, dns := range cfg.DNS {
        rules = append(rules, []string{
            "-A", "CAGE-FILTER", "-p", "udp", "--dport", "53", "-d", dns, "-j", "ACCEPT",
        })
    }
    rules = append(rules, []string{
        "-A", "CAGE-FILTER", "-p", "udp", "--dport", "53", "-j", "DROP",
    })

    // Allow HTTP/HTTPS
    rules = append(rules, []string{
        "-A", "CAGE-FILTER", "-p", "tcp", "--dport", "80", "-j", "ACCEPT",
    })
    rules = append(rules, []string{
        "-A", "CAGE-FILTER", "-p", "tcp", "--dport", "443", "-j", "ACCEPT",
    })

    // Allow other traffic (after blocking private)
    rules = append(rules, []string{
        "-A", "CAGE-FILTER", "-j", "ACCEPT",
    })

    // Apply chain to bridge
    rules = append(rules, []string{
        "-A", "FORWARD", "-i", bridgeName, "-j", "CAGE-FILTER",
    })

    // Execute rules
    for _, rule := range rules {
        cmd := exec.Command("iptables", rule...)
        if err := cmd.Run(); err != nil {
            // Ignore "chain already exists" errors
            if !strings.Contains(err.Error(), "Chain already exists") {
                return fmt.Errorf("iptables rule failed: %v", err)
            }
        }
    }

    return nil
}
```

### 4. DNS enforcement (DNAT)

```go
func SetupDNAT(cageName string, dnsServer string) error {
    bridgeName := fmt.Sprintf("cage-%s", cageName[:8])

    // Redirect all DNS to configured server
    cmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
        "-i", bridgeName,
        "-p", "udp", "--dport", "53",
        "-j", "DNAT", "--to-destination", dnsServer+":53")

    return cmd.Run()
}
```

### 5. Cleanup on stop

```go
func CleanupFirewall(cageName string) error {
    bridgeName := fmt.Sprintf("cage-%s", cageName[:8])

    // Remove FORWARD rule
    exec.Command("iptables", "-D", "FORWARD", "-i", bridgeName, "-j", "CAGE-FILTER").Run()

    // Remove DNAT
    exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
        "-i", bridgeName, "-p", "udp", "--dport", "53",
        "-j", "DNAT", "--to-destination", "1.1.1.1:53").Run()

    // Note: CAGE-FILTER chain is shared, don't delete

    return nil
}
```

### 6. Network verification

```go
// internal/network/verify.go
func VerifyIsolation(cageName string) error {
    // Test 1: Can reach internet
    err := cage.Exec(cageName, "curl -s --max-time 5 https://google.com")
    if err != nil {
        return errors.New("internet access failed")
    }

    // Test 2: Cannot reach private subnet
    err = cage.Exec(cageName, "ping -c 1 -W 1 192.168.1.1")
    if err == nil {
        return errors.New("SECURITY: private subnet accessible!")
    }

    // Test 3: Cannot reach VPN
    // This depends on your VPN IPs

    return nil
}
```

### 7. Update start workflow

```go
func Start(name string, opts StartOptions) error {
    // ... existing code ...

    cfg := config.Load()

    // 7b. Create network
    network.CreateNetwork(name)

    // 7c. Setup firewall
    network.SetupFirewall(name, cfg.Network)
    network.SetupDNAT(name, cfg.Network.DNS[0])

    // ... create domain ...

    return nil
}

func Stop(name string, force bool) error {
    // ... existing code ...

    // Cleanup network
    network.CleanupFirewall(name)
    network.DestroyNetwork(name)

    // ... rest ...
}
```

## Acceptance test

```bash
# Start cage
./cage start --name test

# Test internet access
./cage ssh test "curl -s https://google.com | head -c 100"
# <!doctype html>...

# Test DNS works
./cage ssh test "host google.com"
# google.com has address ...

# Test private subnets BLOCKED
./cage ssh test "ping -c 1 -W 1 192.168.1.1"
# (should fail/timeout)

./cage ssh test "ping -c 1 -W 1 10.0.0.1"
# (should fail/timeout)

./cage ssh test "ping -c 1 -W 1 172.16.0.1"
# (should fail/timeout)

# Test VPN interfaces BLOCKED (if you have VPN)
# This depends on your VPN IP

# Cleanup
./cage stop test
```

## Security verification script

```bash
#!/bin/bash
# verify-isolation.sh

echo "=== Network Isolation Test ==="

echo -n "Internet access: "
cage ssh test "curl -s --max-time 5 https://google.com > /dev/null" && echo "OK" || echo "FAIL"

echo -n "192.168.0.0/16 blocked: "
cage ssh test "ping -c 1 -W 1 192.168.1.1 2>/dev/null" && echo "FAIL (SECURITY!)" || echo "OK"

echo -n "10.0.0.0/8 blocked: "
cage ssh test "ping -c 1 -W 1 10.0.0.1 2>/dev/null" && echo "FAIL (SECURITY!)" || echo "OK"

echo -n "172.16.0.0/12 blocked: "
cage ssh test "ping -c 1 -W 1 172.16.0.1 2>/dev/null" && echo "FAIL (SECURITY!)" || echo "OK"

echo -n "169.254.0.0/16 blocked: "
cage ssh test "ping -c 1 -W 1 169.254.169.254 2>/dev/null" && echo "FAIL (SECURITY!)" || echo "OK"
```

## Deliverables

- [x] Libvirt NAT network creation
- [x] CAGE-FILTER iptables chain
- [x] VPN interface blocking (tun+, tailscale+, wg+)
- [x] RFC 1918 subnet blocking
- [x] Link-local blocking (169.254.0.0/16)
- [x] DNS DNAT enforcement
- [x] Firewall cleanup on stop
- [x] Network verification tests
- [x] Security test script
