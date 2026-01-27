# Security Model

Claude Cage is designed to isolate potentially untrusted code execution from the host system and local network. This document describes the security architecture and threat mitigation strategies.

## Threat Model

### Primary Threats

1. **Host System Access** - Malicious code attempting to access or modify host files
2. **Local Network Access** - Code attempting to reach local services (databases, internal APIs)
3. **VPN Tunnels** - Code attempting to access corporate resources via VPN
4. **Cloud Metadata** - Code attempting to access cloud provider metadata services
5. **Resource Exhaustion** - Code consuming excessive CPU, memory, or disk

### Out of Scope

- Kernel-level VM escapes (relies on QEMU/KVM security)
- Supply chain attacks on base images
- Side-channel attacks

## Security Layers

```
┌──────────────────────────────────────────────────────────────────┐
│                     Host System                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                   KVM Hypervisor                            │  │
│  │  ┌──────────────────────────────────────────────────────┐  │  │
│  │  │                    Guest VM                           │  │  │
│  │  │  ┌────────────────────────────────────────────────┐  │  │  │
│  │  │  │              User Process                       │  │  │  │
│  │  │  │              (Claude Code)                      │  │  │  │
│  │  │  └────────────────────────────────────────────────┘  │  │  │
│  │  └──────────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### Layer 1: VM Isolation (KVM)

**Mechanism:** Hardware-assisted virtualization via KVM

**Protections:**
- Separate kernel and address space
- CPU privilege ring isolation
- Memory isolation via IOMMU
- Disk isolation via qcow2 overlay

**Configuration:** CPU mode is `host-passthrough` for performance while maintaining isolation.

### Layer 2: Network Isolation

**Mechanism:** iptables firewall rules + restricted DNS

**Protections:**
- Block RFC 1918 private subnets (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Block link-local addresses (169.254.0.0/16)
- Block VPN interfaces (tun+, wg+, tailscale+)
- Enforce DNS through specific servers (prevents DNS-based exfiltration to local resolvers)

See [Network Isolation](security-network.md) for details.

### Layer 3: File System Isolation

**Mechanism:** Copy-on-write disks + optional virtiofs

**Protections:**
- Base images are read-only (qcow2 backing file)
- Changes written to overlay (don't affect base)
- File shares are opt-in and explicitly configured
- virtiofsd runs with sandbox when root

### Layer 4: Resource Limits

**Mechanism:** libvirt resource constraints

**Protections:**
- CPU: Limited vCPUs per profile
- Memory: Hard memory limits
- Disk: Disk size limits on overlay
- Process limits (max_processes)
- I/O weight for fair scheduling

## Network Modes

### Auto Mode (Default)

**No root required.** Uses SLIRP user-mode networking.

```
Guest VM ─── SLIRP ─── Internet
                │
                ⚠️ LAN (reachable via router)
```

- Uses QEMU's built-in SLIRP networking
- SSH access via port forwarding: `localhost:<port> → VM:22`
- **⚠️ Note**: Does NOT fully isolate from LAN - traffic can reach local network via router

### Bridge Mode (Recommended for Full Isolation)

**Requires root.** Uses libvirt NAT bridge with iptables.

```
Guest VM ─── NAT Bridge ─── iptables ─── Internet
                                │
                                X─── Local Network (blocked by rules)
```

**Firewall rules applied:**
1. Allow established/related connections
2. Block VPN interfaces
3. Block private subnets
4. Allow only configured DNS servers
5. Allow HTTP/HTTPS to internet
6. Accept remaining (public internet)

## Authentication

### SSH Keys

- Ed25519 keys generated per cage
- Private key stored in `~/.claude-cage/keys/<cage>/`
- Public key injected via cloud-init
- No passphrase (convenience for automation)

### Console Access

- User: `cage`
- Password: `cage` (for emergency console access)
- SSH password auth disabled (keys only)

## File Sharing Security

### virtiofs

When file sharing is configured:

```yaml
shares:
  - host: ~/projects
    guest: /workspace
    mode: rw
```

**Protections:**
- virtiofsd runs with `--sandbox=chroot` (when root)
- Limited to explicitly shared directories
- Can be configured as read-only (`mode: ro`)

**Non-root mode:**
- `--sandbox=none` (can't chroot without root)
- Relies on VM isolation as primary barrier

### Recommendations

1. Share only necessary directories
2. Use read-only mode when possible
3. Avoid sharing home directory root
4. Don't share directories with secrets (`.ssh`, `.aws`)

## Environment Variables

Environment variables are injected via cloud-init:

```yaml
env:
  API_KEY: secret
```

**Security Notes:**
- Written to `/etc/profile.d/cage-env.sh` (readable by cage user)
- Not visible from host unless explicitly shared
- Consider using vault/secrets manager for production

## Verification

### cage verify

Run network isolation tests:

```bash
cage verify myvm
```

**Tests performed:**
1. Internet access (should work)
2. DNS resolution (should work)
3. 192.168.0.0/16 blocked (should fail to connect)
4. 10.0.0.0/8 blocked (should fail to connect)
5. 172.16.0.0/12 blocked (should fail to connect)
6. 169.254.0.0/16 blocked (should fail to connect)

### Manual Verification

From inside the cage:

```bash
# Should work
curl https://google.com

# Should fail (blocked)
ping 192.168.1.1
ping 10.0.0.1
curl http://169.254.169.254  # Cloud metadata
```

## Security Configuration

### Default Security Settings

```yaml
security:
  max_cages: 10              # Prevent resource exhaustion
  virtiofsd_sandbox: true    # Use chroot sandbox (root only)
```

### Network Security Settings

```yaml
network:
  blocked_interfaces:
    - tun+        # Block OpenVPN tunnels
    - tailscale+  # Block Tailscale
    - wg+         # Block WireGuard
  blocked_subnets:
    - 10.0.0.0/8       # Block private Class A
    - 172.16.0.0/12    # Block private Class B
    - 192.168.0.0/16   # Block private Class C
    - 169.254.0.0/16   # Block link-local/metadata
  dns:
    - 1.1.1.1    # Only allow specific DNS
    - 8.8.8.8
  port_bind: "127.0.0.1"  # Bind ports to localhost only
```

## Known Limitations

1. **VM Escape Vulnerabilities** - Relies on KVM/QEMU security. Keep host updated.

2. **Shared Memory** - virtiofs requires shared memory between host and guest. This is isolated by the hypervisor but increases attack surface.

3. **Clock/Side Channels** - VM shares host clock. Timing attacks may be possible.

4. **USB/Hardware** - No hardware passthrough by default. Don't add it unless necessary.

## Recommendations

### For Development

- Use `auto` network mode (no root, simpler security model)
- Share only project directories
- Use default resource profiles
- Run `cage verify` after creation

### For Sensitive Workloads

- Use `bridge` mode with iptables rules
- Add custom blocked subnets if needed
- Set `virtiofsd_sandbox: true` and run as root
- Use read-only file shares
- Monitor cage processes

### For Production/CI

- Use ephemeral cages (create, use, remove)
- Don't persist sensitive data in cages
- Use separate cages for different trust levels
- Implement cage resource quotas

## See Also

- [Network Isolation](security-network.md) - Detailed firewall rules
- [Configuration Models](models-config.md) - Security settings
