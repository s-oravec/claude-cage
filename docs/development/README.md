# Developer Documentation

This documentation covers the architecture, design, and implementation details of Cage.

## Overview

Cage is a CLI tool that creates isolated QEMU/KVM virtual machines for running arbitrary workloads in a secure sandbox. It provides network isolation, file sharing, and snapshot management while being easy to use for developers.

## Table of Contents

### Architecture
- [Architecture Overview](architecture.md) - System design, layers, and component interactions
- [Data Flow](data-flow.md) - Request lifecycle and data transformations

### Modules
- [Modules Overview](modules.md) - Package structure and responsibilities
- [CLI Commands](modules-cmd.md) - Command implementation details

### Data Models
- [Configuration Models](models-config.md) - Configuration structures and YAML schema
- [Runtime Models](models-runtime.md) - State management and cage lifecycle

### Security
- [Security Model](security.md) - Isolation mechanisms and threat mitigation
- [Network Isolation](security-network.md) - Firewall rules and subnet blocking

### Development
- [Getting Started](getting-started.md) - Setting up development environment
- [Testing Guide](testing.md) - Running and writing tests
