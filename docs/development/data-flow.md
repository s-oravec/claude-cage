# Data Flow

This document describes how data flows through Claude Cage during key operations.

## Cage Creation Flow

```
cage create -n myvm --ssh auto
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Validation                                                  │
│    - Check cage doesn't exist                                  │
│    - Validate network mode                                     │
│    - Parse SSH port (auto → find free port)                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Load Configuration                                          │
│    - Load ~/.claude-cage/config.yaml                          │
│    - Merge .claude-cage.yml if present                        │
│    - Resolve profile (default, heavy, light)                  │
│    - Resolve image alias (alpine → alpine-3.21)               │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Create Cage Directory                                       │
│    ~/.claude-cage/cages/<name>/                               │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 4. Network Setup (bridge mode only)                            │
│    - Create libvirt NAT network (virsh net-define/start)      │
│    - Setup iptables firewall rules                            │
│    - Setup DNS DNAT                                           │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 5. Create Disk Overlay                                         │
│    qemu-img create -f qcow2 -b <base> disk.qcow2 <size>G     │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 6. Generate SSH Keys                                           │
│    ssh-keygen -t ed25519 → ~/.claude-cage/keys/<name>/        │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 7. Generate Cloud-init ISO                                     │
│    - Create user-data (user, SSH key, packages, env vars)     │
│    - Create meta-data (hostname)                              │
│    - Generate ISO (cloud-localds or genisoimage)              │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 8. Define libvirt Domain                                       │
│    - Generate domain XML (cpu, memory, disk, network, etc.)   │
│    - virsh define <xml>                                       │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 9. Save State                                                  │
│    Write state.json with status="stopped"                     │
└───────────────────────────────────────────────────────────────┘
```

## Cage Start Flow

```
cage start myvm
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Load State                                                  │
│    Read ~/.claude-cage/cages/<name>/state.json                │
│    Verify status is "stopped"                                 │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Start virtiofsd (if file sharing configured)               │
│    virtiofsd --socket-path=<sock> --shared-dir=<path>         │
│    Save PID for cleanup                                       │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Start Domain                                                │
│    virsh start cage-<name>                                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 4. Setup Port Forwarding (auto mode only)                      │
│    Start forwarding processes for requested ports             │
│    Save PIDs for cleanup                                      │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 5. Update State                                                │
│    status="running", startedAt=now()                          │
│    Save PIDs: virtiofsd, forwarder, passt                     │
└───────────────────────────────────────────────────────────────┘
```

## SSH Connection Flow

```
cage ssh myvm
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Load State                                                  │
│    Get SSH port and network mode                              │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Determine Connection Target                                 │
│    auto mode: localhost:<ssh_port>                            │
│    bridge mode: <vm_ip>:22                                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Execute SSH                                                 │
│    ssh -i <key> -o StrictHostKeyChecking=no cage@<target>     │
└───────────────────────────────────────────────────────────────┘
```

## Configuration Merge Flow

```
cage create -n myvm
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Load Global Config                                          │
│    ~/.claude-cage/config.yaml                                  │
│    ┌─────────────────────────────────────┐                    │
│    │ images:                              │                    │
│    │   default: alpine                    │                    │
│    │ profiles:                            │                    │
│    │   default: {vcpu: 4, memory: 4096}   │                    │
│    │ network:                             │                    │
│    │   dns: [1.1.1.1, 8.8.8.8]           │                    │
│    └─────────────────────────────────────┘                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Check for Project Config                                    │
│    ./.claude-cage.yml (current directory)                     │
│    ┌─────────────────────────────────────┐                    │
│    │ profiles:                            │                    │
│    │   default: {vcpu: 8, memory: 8192}   │  ← overrides      │
│    │ env:                                 │                    │
│    │   NODE_ENV: development              │  ← adds           │
│    └─────────────────────────────────────┘                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Merge Configs                                               │
│    - Scalars: project wins                                    │
│    - Maps: merge, project wins on conflict                    │
│    - Arrays: project replaces                                 │
│    ┌─────────────────────────────────────┐                    │
│    │ images:                              │                    │
│    │   default: alpine                    │ (global)          │
│    │ profiles:                            │                    │
│    │   default: {vcpu: 8, memory: 8192}   │ (project)         │
│    │ network:                             │                    │
│    │   dns: [1.1.1.1, 8.8.8.8]           │ (global)          │
│    │ env:                                 │                    │
│    │   NODE_ENV: development              │ (project)         │
│    └─────────────────────────────────────┘                    │
└───────────────────────────────────────────────────────────────┘
```

## Cleanup Flow (cage remove)

```
cage remove myvm
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Load State                                                  │
│    Check if running (force required if so)                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Stop if Running                                             │
│    - Kill virtiofsd process                                   │
│    - Kill port forwarder processes                            │
│    - virsh destroy cage-<name>                                │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Remove libvirt Domain                                       │
│    virsh undefine cage-<name>                                 │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 4. Cleanup Network (bridge mode)                               │
│    - Remove iptables rules                                    │
│    - virsh net-destroy/net-undefine                           │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 5. Delete Files                                                │
│    - Remove ~/.claude-cage/cages/<name>/                      │
│    - Remove ~/.claude-cage/keys/<name>/                       │
│    - Remove virtiofs socket directory                         │
└───────────────────────────────────────────────────────────────┘
```

## See Also

- [Architecture Overview](architecture.md) - System design
- [Modules Overview](modules.md) - Package details
