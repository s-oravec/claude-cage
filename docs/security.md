# Network Isolation

Cage provides network isolation to prevent the VM from accessing your local network (LAN) while still allowing internet access. This protects against a compromised or untrusted workload (for example an AI coding agent hit by a prompt injection attack) that might try to access local services.

## How It Works

### SLIRP (User-Mode Networking) - Default

When using SLIRP networking (the default), cage adds blackhole routes inside the VM via cloud-init:

```bash
# These routes block access to private IP ranges
ip route add unreachable 10.0.0.0/8
ip route add unreachable 172.16.0.0/12
ip route add unreachable 192.168.0.0/16
ip route add unreachable 169.254.0.0/16
```

The SLIRP network (10.0.2.0/24) is more specific and takes precedence, so the VM can still communicate with its gateway.

**Note**: These routes are inside the VM. A root user inside the VM could remove them. This provides defense-in-depth but is not a complete security boundary.

### Bridge Networking

When using bridge networking (`--network bridge`), cage applies iptables rules on the host:

- Blocks traffic to RFC 1918 private ranges
- Blocks VPN interfaces (tun+, tailscale+, wg+)
- Enforces DNS to specified servers only

This provides stronger isolation as the rules are enforced by the host.

## Verification

You can verify network isolation with:

```bash
cage verify <name>
```

This runs tests from inside the VM to confirm:
- ✓ Internet access works
- ✓ DNS resolution works
- ✓ 192.168.0.0/16 is blocked
- ✓ 10.0.0.0/8 is blocked
- ✓ 172.16.0.0/12 is blocked
- ✓ 169.254.0.0/16 is blocked

## Configuration

Network isolation is enabled by default. The blocked subnets can be configured in `~/.cage/config.yaml`:

```yaml
network:
  blocked_subnets:
    - "10.0.0.0/8"
    - "172.16.0.0/12"
    - "192.168.0.0/16"
    - "169.254.0.0/16"
  blocked_interfaces:
    - "tun+"
    - "tailscale+"
    - "wg+"
  dns:
    - "1.1.1.1"
    - "8.8.8.8"
```

## Limitations

1. **SLIRP mode**: Isolation routes are inside the VM - a root user could bypass them
2. **Bridge mode**: Requires running cage commands as root to apply iptables rules
3. **IPv6**: Currently only IPv4 private ranges are blocked

For maximum isolation, consider running the cage on a separate network segment or using a dedicated VM host.
