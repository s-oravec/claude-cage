# cage version

Zobrazí verziu cage CLI a súvisiacich komponentov.

## Použitie

```bash
cage version [flags]
```

## Flags

| Flag | Popis |
|------|-------|
| `--short` | Len číslo verzie |
| `--json` | JSON výstup |

## Príklady

```bash
cage version
```

Výstup:
```
cage version 1.0.0

Components:
  QEMU:        8.2.0
  libvirt:     9.0.0
  virtiofsd:   1.8.0

System:
  KVM:         /dev/kvm (accessible)
  libvirtd:    running
  OS:          Ubuntu 24.04
  Arch:        x86_64
```

## Short

```bash
cage version --short
# 1.0.0
```

## JSON

```bash
cage version --json
```

```json
{
  "cage": "1.0.0",
  "components": {
    "qemu": "8.2.0",
    "libvirt": "9.0.0",
    "virtiofsd": "1.8.0"
  },
  "system": {
    "kvm_available": true,
    "libvirtd_running": true,
    "os": "Ubuntu 24.04",
    "arch": "x86_64"
  }
}
```
