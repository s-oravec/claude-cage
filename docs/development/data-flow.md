# Data Flow

This document describes how data flows through Claude Cage during key operations.

## Cage Start Flow (with Auto-Creation)

The `cage start` command handles both cage creation and starting. When run in a directory
with `.claude-cage.yml`, it will create the cage if it doesn't exist.

```
cage start (in project directory)
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Resolve Cage Name                                           │
│    - If name provided as argument: use it                      │
│    - If no argument: load from .claude-cage.yml                │
│    - Cage name defaults to directory name if not in config     │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Load Configuration                                          │
│    - Load ~/.claude-cage/config.yaml (global)                 │
│    - Load .claude-cage.yml (project)                          │
│    - Resolve config (merge profile + overrides)               │
│    - Resolve image alias (alpine → alpine-3.21)               │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Check If Cage Exists                                        │
│    Does ~/.claude-cage/cages/<name>/ exist?                   │
│    ├── NO  → Create cage (see below)                          │
│    └── YES → Validate image, reconfigure if stopped           │
└───────────────────────────────────────────────────────────────┘
        │
        ▼ (if cage doesn't exist)
┌───────────────────────────────────────────────────────────────┐
│ 4. Create Cage                                                 │
│    a. Create cage directory                                   │
│    b. Create disk overlay (qemu-img)                          │
│    c. Generate SSH keys                                       │
│    d. Create runtime directory for env injection              │
│    e. Generate cloud-init ISO (with UseRuntimeEnv=true)       │
│    f. Generate domain XML (with RuntimeDir set)               │
│    g. Define libvirt domain (virsh define)                    │
│    h. Save state with status="stopped"                        │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 5. Write Runtime Environment                                   │
│    - Write env vars to <cage>/runtime/env.sh                  │
│    - This is mounted via virtiofs at /cage/runtime            │
│    - Sourced from /etc/profile.d/cage-runtime-env.sh          │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 6. Start VM                                                    │
│    - Start virtiofsd if configured (bridge mode)              │
│    - virsh start cage-<name>                                  │
│    - Setup port forwarding                                    │
│    - Update state to status="running"                         │
└───────────────────────────────────────────────────────────────┘
```

## Cage Init Flow

```
cage init --image ubuntu-24.04
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Validate                                                    │
│    - --image is required                                      │
│    - Check if .claude-cage.yml already exists                 │
│    - If exists and no --force: error                          │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Build ProjectConfig                                         │
│    - Set image from --image                                   │
│    - Set cage name from --cage or leave empty (dir name)      │
│    - Set memory/vcpu/disk if provided                         │
│    - Set network.ssh (default: auto)                          │
│    - Add default share: . → /workspace                        │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Write .claude-cage.yml                                      │
│    - Add header comment                                       │
│    - Marshal config to YAML                                   │
│    - Write to file                                            │
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

## Configuration Resolution Flow

```
cage start (with .claude-cage.yml)
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
│    │   heavy: {vcpu: 8, memory: 8192}     │                    │
│    │ network:                             │                    │
│    │   dns: [1.1.1.1, 8.8.8.8]           │                    │
│    └─────────────────────────────────────┘                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Load Project Config                                         │
│    ./.claude-cage.yml                                         │
│    ┌─────────────────────────────────────┐                    │
│    │ image: ubuntu-24.04                  │ (required)        │
│    │ profile: default                     │ (reference)       │
│    │ memory: 8G                           │ (override)        │
│    │ env:                                 │                    │
│    │   NODE_ENV: development              │                    │
│    └─────────────────────────────────────┘                    │
└───────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Resolve Config                                              │
│    ResolveProjectConfig(global, project, projectDir)          │
│    ┌─────────────────────────────────────┐                    │
│    │ CageName: "myproject"                │ (from dir name)   │
│    │ Image: "ubuntu-24.04"                │ (from project)    │
│    │ VCPU: 4                              │ (from profile)    │
│    │ MemoryMB: 8192                       │ (override: 8G)    │
│    │ DiskGB: 20                           │ (from profile)    │
│    │ Env: {NODE_ENV: development}         │ (from project)    │
│    │ Shares: [{./src → /workspace}]       │ (resolved paths)  │
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
