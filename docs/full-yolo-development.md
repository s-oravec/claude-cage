# Scenario: Full Yolo Agentic Development

An untrusted workload (for example an AI coding agent) runs in yolo mode inside an isolated cage VM with full Docker access.

## Problem

I want to run a workload in yolo mode (automatic command approval, no guardrails) but:
- It must not have access to VPN (corporate network, tailscale, wireguard)
- It must not see the home network (192.168.x.x)
- It must not have access to sensitive files (~/.ssh, ~/.aws, ~/.config)
- I need full Docker functionality (not limited Docker-in-Docker)

## Solution

```bash
# 1. Initialize cage configuration in project directory
cd ~/projects/myapp
cage init --image ubuntu-24.04 --memory 8G --vcpu 8

# 2. Start cage with port forwarding
cage start --port 3000:3000 --port 5432:5432

# 3. SSH into cage
cage ssh

# 4. Run your workload in yolo mode (example: an AI coding agent
#    with all permission prompts disabled)
your-workload --yolo

# The workload can now:
# - run any commands
# - use Docker/docker-compose
# - install packages
# - all in a secure sandbox
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              HOST                                        │
│                                                                          │
│  Terminal                                                                │
│  └── cage ssh dev                                                        │
│            │                                                             │
│            ▼                                                             │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                         CAGE VM (QEMU/KVM)                         │  │
│  │                                                                    │  │
│  │  ┌─────────────────────────────────────────────────────────────┐  │  │
│  │  │                  Untrusted workload (yolo mode)              │  │  │
│  │  │                                                              │  │  │
│  │  │   ✓ Full shell access                                       │  │  │
│  │  │   ✓ Full Docker access (native daemon)                      │  │  │
│  │  │   ✓ File editing in /workspace                              │  │  │
│  │  │   ✓ Package installation (apt, npm, pip)                    │  │  │
│  │  │   ✓ Public internet access                                  │  │  │
│  │  └─────────────────────────────────────────────────────────────┘  │  │
│  │                              │                                     │  │
│  │                              ▼                                     │  │
│  │  ┌─────────────────────────────────────────────────────────────┐  │  │
│  │  │              Docker daemon (native in VM)                    │  │  │
│  │  │                                                              │  │  │
│  │  │   - Full functionality (privileged, volumes, networks)      │  │  │
│  │  │   - docker-compose, docker build                            │  │  │
│  │  │   - No limitations like with Docker-in-Docker               │  │  │
│  │  └─────────────────────────────────────────────────────────────┘  │  │
│  │                                                                    │  │
│  │  /workspace ←───── virtio-fs ─────→ ~/projects/myapp              │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  BLOCKED (iptables CAGE-FILTER chain):                                  │
│  ├── tun+ (OpenVPN)                                                     │
│  ├── tailscale+ (Tailscale)                                             │
│  ├── wg+ (WireGuard)                                                    │
│  ├── 10.0.0.0/8 (RFC 1918)                                              │
│  ├── 172.16.0.0/12 (RFC 1918)                                           │
│  ├── 192.168.0.0/16 (RFC 1918)                                          │
│  └── 169.254.0.0/16 (link-local)                                        │
│                                                                          │
│  ALLOWED:                                                                │
│  └── Public internet (via host NAT)                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

## Security Layers

| Layer | Protection | Implementation |
|-------|------------|----------------|
| VM isolation | Complete separation from host | QEMU/KVM with own kernel |
| Network isolation | VPN and internal network blocking | iptables CAGE-FILTER chain |
| Filesystem isolation | Only /workspace is shared | virtiofsd with --sandbox chroot |
| Resource limits | CPU/RAM/IO control | cgroups v2 |
| Ephemeral environment | Changes only persist in /workspace | qcow2 copy-on-write |
| DNS enforcement | DNS query control | DNAT to 1.1.1.1/8.8.8.8 |

## What the workload CAN do (inside cage)

- Run any shell commands
- Use Docker (build, run, compose, exec, logs...)
- Run privileged containers
- Create Docker networks and volumes
- Install system packages (apt, dnf)
- Install dev dependencies (npm, pip, cargo)
- Access public internet (GitHub, npm, PyPI)
- Modify anything in /workspace

## What the workload CANNOT do

- Access VPN networks (corporate network)
- Access Tailscale/WireGuard networks
- See home network (192.168.x.x, 10.x.x.x)
- Read host filesystem (except /workspace)
- Access ~/.ssh, ~/.aws, ~/.config on host
- Modify host system
- Communicate with other VMs/containers on host
- Access link-local addresses

## Typical Workflow

```bash
# === FIRST TIME SETUP ===
cd ~/projects/myapp
cage init --image ubuntu-24.04 --memory 8G --vcpu 8

# Add port forwards to .cage.yml if needed regularly:
# network:
#   ssh: auto
#   ports:
#     - "3000:3000"
#     - "5432:5432"

# === MORNING - Start work ===
cd ~/projects/myapp
cage start

# SSH into cage
cage ssh

# Start development stack
cd /workspace
docker compose up -d

# Run your workload in yolo mode (example: an AI coding agent)
your-workload --yolo

# === WORK ===
# The workload can:
# - edit code
# - run tests
# - restart containers
# - install dependencies
# - debug
# - all automatically

# === EVENING - End work ===
exit                    # exit the workload
docker compose down     # stop containers
exit                    # leave cage
cage stop               # stop VM

# State:
# - Changes in ~/projects/myapp (host) are preserved
# - Everything else in VM is lost
# - No residue, containers, volumes
```

## Docker Compose Example

```bash
# On host: ~/projects/myapp/docker-compose.yaml
cage ssh dev
cd /workspace
docker compose up -d

# Application:
# - localhost:3000 → web app (port forwarded from cage)
# - localhost:5432 → postgres (port forwarded from cage)
```

## Monitoring and Control

```bash
# In another terminal on host:

# Watch resource usage
cage status dev --watch

# Watch VM logs
cage logs dev -f

# Run command without SSH session
cage exec dev -- docker ps
cage exec dev -- ps aux

# If something suspicious - immediate stop
cage stop dev --force
```

## Snapshots for Experiments

```bash
# Before risky experiment
cage snapshot create dev --name before-experiment

# The workload runs the experiment...
# Something went wrong!

# Restore state
cage stop dev
cage snapshot restore dev --name before-experiment
cage start dev

# Back to original state
```

## Fail-safe

If the workload does something suspicious:

```bash
# Immediate stop (from host)
cage stop dev --force

# VM is immediately destroyed
# - the workload loses all state
# - No residue
# - /workspace on host remains (you can check changes)

# If needed - git reset changes
cd ~/projects/myapp
git diff                 # check what changed
git checkout .           # discard changes if needed
```

## Why QEMU/KVM and not Containers

| Docker container | Cage VM |
|------------------|---------|
| Shared kernel with host | Own kernel |
| Container escape possible | VM escape extremely difficult |
| Privileged = full host access | Privileged = only in VM |
| Network isolation complicated | Network isolation native |
| Docker-in-Docker limited | Docker native, full functionality |
