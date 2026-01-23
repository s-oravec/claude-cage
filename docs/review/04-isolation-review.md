# Claude Cage - Security Isolation Review

**Reviewer:** Virtualization Security Expert
**Date:** 2026-01-23
**Scope:** Host isolation analysis - VM escape vectors, resource exhaustion, persistence

---

## Zhrnutie

Claude Cage predstavuje solídny bezpečnostný sandbox založený na QEMU/KVM virtualizácii. Architektúra správne oddeľuje nedôveryhodný kód od host systému pomocou viacvrstvovej izolácie. Dizajn je pre daný use case (Claude Code v "yolo mode") vhodný, no existuje niekoľko oblastí vyžadujúcich pozornosť pre dosiahnutie produkčnej úrovne bezpečnosti.

**Celkové hodnotenie: 7.5/10** - Dobrá základná architektúra s priestorom na hardening.

---

## Silné stránky

### 1. QEMU/KVM ako základ virtualizácie

- **Hardware-level izolácia:** KVM využíva Intel VT-x/AMD-V pre skutočnú hardware virtualizáciu, nie kontajnerovú izoláciu
- **Vlastný kernel:** VM beží s vlastným Linux kernelom, čo eliminuje kernel syscall attack surface
- **Overená technológia:** QEMU/KVM je battle-tested v produkčných prostrediach (AWS, GCP, OpenStack)
- **Memory isolation:** Každá VM má vlastný adresný priestor chránený hardvérovo (EPT/NPT)

### 2. Viacvrstvová bezpečnosť

```
┌─────────────────────────────────────────┐
│ VM Guest (nedôveryhodný kód)            │
├─────────────────────────────────────────┤
│ virtio drivers (paravirtualizácia)      │
├─────────────────────────────────────────┤
│ QEMU userspace (seccomp sandbox)        │
├─────────────────────────────────────────┤
│ KVM kernel module                       │
├─────────────────────────────────────────┤
│ SELinux/AppArmor (sVirt labeling)       │
├─────────────────────────────────────────┤
│ Host kernel                             │
└─────────────────────────────────────────┘
```

### 3. Sieťová izolácia

- Explicitné blokovanie VPN interfaces (`tun0`, `tailscale0`)
- Oddelenie od internej siete
- Verejné DNS servery (`1.1.1.1`, `8.8.8.8`)

### 4. Ephemeral design

- Copy-on-write qcow2 images
- Čistý štart pri každom spustení
- Žiadna perzistencia mimo `/workspace`

### 5. Libvirt security features

- QEMU beží ako neprivilegovaný user
- sVirt labeling (SELinux/AppArmor)
- cgroups resource limits
- seccomp filtering

---

## Potenciálne úniky / Riziká

### 1. QEMU/KVM VM Escape - KRITICKÉ

**Riziko: STREDNÉ-VYSOKÉ**

| Vektor | Popis | Mitigácia v dizajne |
|--------|-------|---------------------|
| QEMU CVE | QEMU má historicky CVE umožňujúce escape | Čiastočná (seccomp, sVirt) |
| virtio driver bugs | virtio-blk, virtio-net, virtio-fs | Nie je explicitne riešené |
| VFIO/passthrough | Ak by sa použilo | Neaplikuje sa |

**Známe historické VM escape CVE:**
- CVE-2020-14364 (QEMU USB)
- CVE-2019-6778 (QEMU networking)
- CVE-2017-2615 (Cirrus VGA)

**Odporúčanie:**
```yaml
# Minimalizovať QEMU attack surface
qemu_hardening:
  disable_devices:
    - usb
    - floppy
    - parallel
    - serial (okrem konzoly)
  use_microvm: true  # minimálny machine type
  disable_legacy: true
```

### 2. virtio-fs bezpečnosť - VYSOKÉ RIZIKO

**Riziko: VYSOKÉ**

virtio-fs je pomerne nová technológia s komplexnou implementáciou.

| Aspekt | Status | Problém |
|--------|--------|---------|
| Path traversal | Potrebuje audit | `../../../etc/passwd` |
| Symlink escape | Potrebuje audit | Symlink na host filesystem |
| Race conditions | Potrebuje audit | TOCTOU útoky |
| virtiofsd CVE | Potrebuje monitoring | Nový kód = nové bugy |

**Kritický problém:** Dizajn umožňuje `--share ~/data:/data` - viac zdieľaných adresárov zvyšuje attack surface.

**Odporúčanie:**
```yaml
virtiofs_hardening:
  # Striktný sandbox pre virtiofsd
  sandbox: chroot  # alebo namespace
  # Zakázať symlink following mimo workspace
  no_xattr: true
  # Len nutné capabilities
  capabilities: []
  # Chroot virtiofsd do workspace
  source_validation: strict
```

**Test na path traversal:**
```bash
# Vo VM otestovať:
ls -la /workspace/../../../etc/passwd
cat /workspace/../../../etc/shadow
```

### 3. Resource exhaustion - STREDNÉ RIZIKO

**Riziko: STREDNÉ**

Dizajn spomína cgroups, ale neuvádza konkrétne limity.

| Resource | Bez limitov | Dopad |
|----------|-------------|-------|
| CPU | fork bomb | Host freeze |
| Memory | memory exhaustion | OOM killer na hoste |
| Disk I/O | I/O storm | Host disk latency |
| Network | bandwidth flood | Host network degraded |
| Disk space | qcow2 grows | Host disk full |

**Chýbajúce špecifikácie:**
```yaml
# Explicitne definovať:
cgroups_limits:
  cpu_quota: 400000  # 4 cores max
  memory_limit_mb: 8192
  memory_swap_limit_mb: 0  # žiadny swap
  blkio_weight: 500
  blkio_read_bps: 100000000  # 100 MB/s
  blkio_write_bps: 100000000
  pids_max: 4096  # proti fork bomb
```

**Disk space limit:**
```bash
# qcow2 preallocation s limitom
qemu-img create -f qcow2 -o preallocation=metadata,size=50G disk.qcow2
```

### 4. Sieťová izolácia - medzery

**Riziko: STREDNÉ**

| Problém | Popis |
|---------|-------|
| Len interface blocking | Útočník môže stále komunikovať s host services |
| Host localhost | VM môže skúšať `192.168.x.1` (default gateway = host) |
| Metadata service | Ak by bežal na hoste |
| ARP spoofing | V shared bridge |

**Odporúčanie:**
```bash
# Explicitne blokovať prístup k host IP
iptables -I FORWARD -s $VM_NETWORK -d $HOST_IP -j DROP
iptables -I FORWARD -s $VM_NETWORK -d 127.0.0.0/8 -j DROP
iptables -I FORWARD -s $VM_NETWORK -d 169.254.0.0/16 -j DROP
```

### 5. Ephemeral režim - verifikácia

**Riziko: NÍZKE-STREDNÉ**

Ephemeral režim závisí od:

1. **qcow2 backing file:** Zmeny idú do overlay, nie base image - OK
2. **virtiofs:** Zmeny v `/workspace` PERZISTUJÚ na hoste - INTENDED
3. **VM state:** RAM, CPU state - zmizne pri stop - OK

**Potenciálny problém:**
```bash
# Ak VM modifikuje base image namiesto overlay:
# - Buď bug v implementácii
# - Alebo úmyselný útok cez QEMU vulnerability
```

**Verifikácia:**
```bash
# Po každom stop overiť:
sha256sum /path/to/base-image.qcow2
# Hash sa NESMIE zmeniť
```

### 6. Prežitie malware po reštarte

**Riziko: NÍZKE (v rámci VM), STREDNÉ (cez workspace)**

| Lokácia | Prežije reštart? |
|---------|------------------|
| VM disk (/) | NIE |
| VM RAM | NIE |
| /workspace | ANO |
| Host (mimo workspace) | NIE (ak nie je exploit) |

**Problém:** Malware môže:
```bash
# Infikovať súbory v workspace
# Príklad: modifikovať .bashrc, .gitconfig v workspace
# Ak používateľ skopíruje tieto súbory na host...
```

**Odporúčanie:**
- Workspace by mal byť read-only pre .dotfiles
- Alebo explicitný "sanitization" krok pred kopírovaním na host

### 7. Docker-in-Docker security

**Riziko: STREDNÉ**

Docker vo VM je izolovaný od hosta, ale:

| Aspekt | Riziko |
|--------|--------|
| Privileged containers | Môžu modifikovať VM (nie host) |
| Host network v Dockeri | Vidí VM network |
| Docker socket exposure | Ak by bol exportovaný |

**Docker vo VM je OK** - útočník získa kontrolu nad VM, nie host.

---

## Odporúčania

### Kritické (implementovať okamžite)

1. **virtio-fs hardening**
   ```yaml
   virtiofs:
     sandbox: chroot
     no_symlink_follow: true  # alebo restrict to workspace
     seccomp: strict
   ```

2. **Explicitné cgroups limity**
   ```yaml
   resources:
     memory_max: 8G
     memory_swap: 0
     cpu_max: 400%
     pids_max: 4096
     io_max: "100M"
   ```

3. **Network hardening**
   ```bash
   # Blokovať prístup k host IP z VM
   # Blokovať link-local
   # Blokovať localhost routing
   ```

### Vysoká priorita

4. **QEMU minimalizácia**
   ```
   - Použiť microvm machine type
   - Disable všetky nepotrebné devices
   - Strict seccomp profile
   ```

5. **Monitoring a alerting**
   ```yaml
   monitoring:
     alert_on:
       - memory_usage > 90%
       - cpu_usage > 95% for 60s
       - unusual_network_traffic
       - qemu_process_crash
   ```

6. **Base image integrity**
   ```bash
   # Podpisovať base images
   # Verifikovať hash pred každým štartom
   ```

### Stredná priorita

7. **Audit logging**
   ```yaml
   audit:
     log_level: info
     events:
       - vm_start
       - vm_stop
       - port_forward_add
       - unusual_syscalls
   ```

8. **Timeouts**
   ```yaml
   limits:
     vm_max_runtime: 24h  # automatický stop
     idle_timeout: 2h
   ```

---

## Záver

Claude Cage dizajn poskytuje **solídnu základnú izoláciu** pre spúšťanie nedôveryhodného kódu. QEMU/KVM je správna voľba pre tento use case - poskytuje skutočnú hardware-level izoláciu na rozdiel od kontajnerov.

**Hlavné silné stránky:**
- Viacvrstvová bezpečnosť (KVM + seccomp + sVirt + cgroups)
- Ephemeral design eliminuje perzistenciu malware
- Sieťová izolácia od VPN

**Hlavné oblasti na zlepšenie:**
- virtio-fs vyžaduje dodatočný hardening
- Explicitné resource limity nie sú definované
- Chýba ochrana pred prístupom k host IP

**Celkové hodnotenie:**

| Aspekt | Skóre | Poznámka |
|--------|-------|----------|
| VM isolation | 8/10 | Štandard priemyslu |
| virtio-fs | 6/10 | Potrebuje hardening |
| Resource limits | 5/10 | Len náčrt, nie implementácia |
| Network isolation | 7/10 | Dobré, ale medzery |
| Ephemeral | 9/10 | Správny prístup |
| **Celkovo** | **7.5/10** | Dobrý základ |

Pre produkčné nasadenie odporúčam implementovať kritické odporúčania pred spustením s reálne nedôveryhodným kódom.

---

*Review dokončený: 2026-01-23*
