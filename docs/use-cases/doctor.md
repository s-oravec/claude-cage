# cage doctor

Diagnostický nástroj - skontroluje požiadavky a konfiguráciu.

## Použitie

```bash
cage doctor [flags]
```

## Flags

| Flag | Popis |
|------|-------|
| `--fix` | Pokúsi sa automaticky opraviť problémy |
| `--json` | JSON výstup |

## Príklady

```bash
# Základná diagnostika
cage doctor

# S automatickým fixom
cage doctor --fix
```

## Výstup

```
Claude Cage Doctor
==================

System Requirements:
  ✓ KVM available (/dev/kvm)
  ✓ User in kvm group
  ✓ User in libvirt group
  ✓ libvirtd running
  ✓ virtiofsd installed (v1.8.0)
  ✓ qemu-system-x86_64 installed (v8.0.0)

Configuration:
  ✓ Config file exists (~/.claude-cage/config.yaml)
  ✓ Config is valid YAML
  ✓ Default image exists (ubuntu-24.04)

Network:
  ✓ iptables available
  ✓ Bridge module loaded
  ⚠ IPv6 enabled (consider disabling for security)

Storage:
  ✓ Images directory exists (~/.claude-cage/images/)
  ✓ Sufficient disk space (45 GB free)

Security:
  ✓ SELinux/AppArmor active
  ✓ Virtiofsd sandbox enabled

Summary: 14 passed, 1 warning, 0 errors

Run 'cage doctor --fix' to attempt automatic fixes.
```

## Kontrolované položky

| Kategória | Kontrola |
|-----------|----------|
| System | KVM, libvirt, QEMU, virtiofsd |
| Permissions | kvm group, libvirt group |
| Config | YAML syntax, required fields |
| Network | iptables, bridge module |
| Storage | disk space, image integrity |
| Security | SELinux/AppArmor, sandbox config |

## Automatický fix

```bash
cage doctor --fix
```

Môže opraviť:
- Pridať usera do skupín (vyžaduje sudo)
- Vytvoriť chýbajúce adresáre
- Spustiť libvirtd
- Načítať kernel moduly

## Kedy použiť

- Po inštalácii (verifikácia)
- Keď cage nefunguje
- Pred reportovaním bugu
