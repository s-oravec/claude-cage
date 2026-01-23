# Claude Cage - Review Summary

Súhrn 8 nezávislých review od Opus agentov.

## Kritické zistenia (musia byť opravené)

### 1. Virtio-fs bezpečnosť (Security + Isolation + Pentest)
- **Problém:** Chýba `--no-symlinks` a chroot sandboxing pre virtiofsd
- **Riziko:** Path traversal, symlink escape mimo /workspace
- **Odporúčanie:** `virtiofsd --sandbox chroot --no-symlinks`

### 2. Sieťové pravidlá nie sú špecifikované (Security + Network)
- **Problém:** Chýba konkrétny iptables ruleset, len "blokovať tun0, tailscale0"
- **Riziko:** VPN bypass cez routing, DNS tunneling
- **Odporúčanie:** Explicitný firewall chain + blokovať RFC 1918 ranges

### 3. SSH key management nedefinovaný (SSH Review)
- **Problém:** Nie je jasné kedy/ako sa generujú SSH kľúče
- **Odporúčanie:** Dokumentovať: Ed25519, per-cage, cloud-init injection

### 4. Persistence v /workspace (Pentest)
- **Problém:** Malware môže infikovať git hooks, .bashrc, Makefiles
- **Riziko:** Prežije reštart VM cez zdieľané súbory
- **Odporúčanie:** Dokumentovať workspace hygiene, audit pred použitím na hoste

## Stredné zistenia

### 5. Chýbajúce CLI príkazy (Functionality + Use Cases)
- `cage restart` - základný príkaz
- `cage exec` - rýchlejšie ako SSH pre jednoduché príkazy
- `cage snapshot` - qcow2 podporuje, nevyužité
- `cage cp` - kopírovanie súborov

### 6. Cgroups limity nie sú explicitné (Isolation)
- **Problém:** Chýbajú konkrétne hodnoty pre CPU, RAM, I/O
- **Riziko:** Fork bomb, memory exhaustion

### 7. Port forwarding binding (Network)
- **Problém:** Nie je jasné či bind na localhost alebo 0.0.0.0
- **Odporúčanie:** Default 127.0.0.1, voliteľne 0.0.0.0

### 8. Nekonzistencia Firecracker vs QEMU (Scenarios + Use Cases)
- **Problém:** Niektoré dokumenty ešte spomínajú Firecracker
- **Odporúčanie:** Aktualizovať všetky zmienky

## Chýbajúce scenáre

1. Disaster recovery
2. Secrets management (API kľúče)
3. Network debugging
4. VS Code Remote integrácia

## Chýbajúca konfigurácia

1. `blocked_subnets` - blokovanie privátnych IP ranges
2. `readonly_shares` - read-only virtio-fs mounts
3. `max_cages` - limit počtu VM
4. Lifecycle hooks

## Celkové hodnotenie

| Review | Status |
|--------|--------|
| Security | ⚠️ Potrebuje úpravy (virtio-fs, firewall) |
| Functionality | ✅ OK (minor: chýba restart, exec) |
| SSH | ⚠️ Potrebuje dokumentáciu |
| Isolation | ✅ OK (QEMU/KVM je správna voľba) |
| Network | ⚠️ Potrebuje explicitné pravidlá |
| Pentest | ⚠️ Workspace je inherentný tradeoff |
| Scenarios | ✅ OK (minor opravy) |
| Use Cases | ✅ OK (7.5/10) |

## Záver

**Dizajn je fundamentálne správny.** QEMU/KVM poskytuje silnú izoláciu. Hlavné oblasti na zlepšenie:

1. **Kritické:** Virtiofsd hardening, explicitné firewall pravidlá
2. **Dôležité:** SSH dokumentácia, cgroups limity
3. **Nice-to-have:** Chýbajúce CLI príkazy, nové scenáre

Po implementácii kritických odporúčaní bude systém vhodný pre Claude Code v "yolo mode".
