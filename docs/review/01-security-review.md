# Claude Cage - Security Review

**Dokument:** Bezpecnostny audit dizajnu
**Datum:** 2026-01-23
**Reviewer:** Security Analysis (Claude)
**Verzia dizajnu:** 2026-01-23-claude-cage-design.md

---

## Zhrnutie

Claude Cage je sandbox riesenie pre izolaciu Claude Code experimentov vyuzivajuce QEMU/KVM virtualizaciu cez libvirt. Dizajn poskytuje **solidnu zakladnu uroven izolacie** vdaka hardverovej virtualizacii (KVM), avsak obsahuje niekolko oblasti, ktore vyzaduju pozornost - predovsetkym v oblasti virtio-fs konfiguracie a sietovej izolacie.

**Celkove hodnotenie: 7/10** - Vhodne pre experimentalne prostredie, potrebuje doplnenia pre produkcne nasadenie.

---

## Silne stranky

### 1. Hardverova virtualizacia (KVM)

- **Plna HW izolacia** - KVM poskytuje skutocnu hardverovu virtualizaciu s Intel VT-x/AMD-V
- **Odddeleny adresny priestor** - VM ma vlastny kernel, memory space, a zariadenia
- **Nepriepustna hranica** - Utocnik vo VM nema priamy pristup k host pamate
- **Overena technologia** - KVM je pouzivany v produkcii velkych cloud providerov (GCP, AWS)

### 2. Ephemeral disky

- **Cista VM pri kazdom starte** - Akakolvek kompromitacia prezije len do restartu
- **qcow2 copy-on-write** - Zmeny sa neukladaju do base image
- **Rychla obnova** - `cage stop && cage start` obnovi cisty stav

### 3. Libvirt bezpecnostne funkcie

- **sVirt (SELinux/AppArmor)** - Kazda VM ma vlastny bezpecnostny kontext
- **cgroups limity** - Ochrana pred resource exhaustion
- **seccomp profil** - Obmedzene syscally pre QEMU proces
- **Neprivilegovany QEMU** - QEMU bezia ako non-root user

### 4. Sietova segmentacia

- **Blokovanie VPN interfacov** - tun0/tailscale0 nie su dostupne z VM
- **Verejny DNS** - Pouzitie 1.1.1.1/8.8.8.8 zabrannuje DNS leaku do firemnej siete
- **Vlastny bridge** - Izolacia od host networking stacku

### 5. Minimalny attack surface na hoste

- **Len SSH komunikacia** - Jediny vstupny bod je SSH
- **Ziaden pristup k host filesystem** - Okrem explicitne zdielaneneho /workspace

---

## Slabe stranky / Rizika

### VYSOKE RIZIKO

#### R1: Virtio-fs bez read-only moznosti

**Problem:** Dizajn nespecifikuje moznost read-only mountov pre citlive adresare.

```
shares:
  - host: ~/projects
    guest: /workspace
    # CHYBA: mode: ro/rw nie je definovany
```

**Dopad:** Kompromitovany Claude Code moze:
- Modifikovat vsetky subory v ~/projects
- Vkladat backdoory do zdrojoveho kodu
- Mazat subory (ransomware scenar)

**Odporucanie:** Implementovat `mode: ro | rw` parameter. Default by mal byt `ro`.

#### R2: Nedefinovane iptables pravidla

**Problem:** Dizajn uvadza "iptables/nftables na hoste" bez konkretnej specifikacie.

**Otazky bez odpovede:**
- Je default policy DROP alebo ACCEPT?
- Su pravidla stavove (conntrack)?
- Je zablokovane ICMP (potencialne covert channels)?
- Je limitovany egress rate (data exfiltration)?

**Dopad:** Bez explicitnych pravidiel moze dojst k:
- Pokusom o lateral movement
- Data exfiltracia cez DNS tunneling
- Skeningu internej siete

**Odporucanie:** Definovat kompletny iptables ruleset s default DROP policy.

#### R3: SSH key management nie je specifikovany

**Problem:** Dizajn uvadza "SSH key management" avsak bez detailov.

**Otazky:**
- Kto generuje kluce?
- Kde sa ukladaju privatne kluce?
- Su kluce per-cage alebo globalne?
- Aka je rotacia klucov?

**Dopad:** Zle spravovane SSH kluce mozu umoznit:
- Pristup k inym cage instanciam
- Persistentny pristup po kompromitacii

### STREDNE RIZIKO

#### R4: Docker-in-VM s privileged containers

**Problem:** Dizajn explicitne povoluje `privileged containers` vo VM.

```
│   │         - privileged containers ok           │
```

**Dopad:** Ak utocnik ziska pristup k Docker socketu vo VM:
- Moze spustat privileged containers
- Potencialny VM escape (container -> VM je lahsi nez VM -> host)
- Pristup k vsetkym VM resources

**Odporucanie:** Zvazit blokovanie `--privileged` flag defaultne, povolit explicitne.

#### R5: Nedefinovane host-only porty

**Problem:** Port forwarding (`--port 8080:80`) nie je obmedzeny na localhost.

**Otazky:**
- Su forwarded porty viazane len na 127.0.0.1?
- Moze externa siet pristupovat k VM sluzbam?

**Dopad:** Ak porty nie su viazane na localhost:
- Externe utoky na sluzby bezace vo VM
- Pivot point do internej siete

**Odporucanie:** Default binding na `127.0.0.1`, explicitne `0.0.0.0` len so -varovani.

#### R6: Nespecifikovane cgroup limity

**Problem:** Dokument uvadza "cgroups limity" bez konkretnych hodnot.

**Otazky:**
- Ake su CPU/memory limity?
- Je limitovany disk I/O?
- Je limitovany network bandwidth?

**Dopad:** Resource exhaustion utoky mozu ovplyvnit host.

### NIZKE RIZIKO

#### R7: DNS hardening

**Problem:** Pouzitie verejnych DNS serverov (1.1.1.1, 8.8.8.8) je dobre, avsak:
- Nie je specifikovane DNS-over-HTTPS/TLS
- DNS queries su viditelne v plaintext

**Dopad:** Potencialny DNS-based tracking/fingerprinting.

#### R8: Logging a audit

**Problem:** Nedostatocne specifikovany logging.

**Otazky:**
- Su SSH sessions logovane?
- Je network traffic logovany?
- Existuje audit trail pre cage operacie?

---

## Attack Vectors Analyza

### AV1: Claude Code Compromise

**Scenar:** Claude Code vo VM je kompromitovany (prompt injection, supply chain attack).

**Co moze utocnik:**
1. Citanie/zapis do /workspace (vsetky projektove subory)
2. Spustanie arbitrary kodu vo VM
3. Sietova komunikacia na internet (data exfiltration)
4. Spustanie Docker containers

**Co NEMOZE utocnik (ak je dizajn spravne implementovany):**
1. Pristup k host filesystem mimo /workspace
2. Pristup k VPN/Tailscale sieti
3. Prezitie kompromitacie cez restart VM
4. Escape z VM do hostu (velmi tazke cez KVM)

### AV2: VM Escape

**Scenar:** Zero-day vulnerability v QEMU/KVM umozni escape.

**Mitigacie v dizajne:**
- sVirt (SELinux/AppArmor) obmedzuje escaped proces
- seccomp profil limituje syscalls
- Neprivilegovany QEMU user

**Pravdepodobnost:** Velmi nizka - KVM escapes su vzacne a rychlo patchovane.

### AV3: Virtio-fs Attack

**Scenar:** Utocnik exploituje virtio-fs pre pristup k host filesystem.

**Riziko:** Stredne - virtiofsd ma historiu CVE (CVE-2023-1018, CVE-2022-0358).

**Mitigacie:**
- Pravidelne aktualizacie virtiofsd
- Minimalizovat zdielane adresare
- Read-only mounting kde je mozne

### AV4: Network Covert Channels

**Scenar:** Data exfiltration cez povolene sietove spojenia.

**Metody:**
- DNS tunneling
- HTTPS na legitimne domeny (GitHub, npm)
- Steganografia v legitimnej traffic

**Mitigacia:** Tazko zabranit uplne pri potrebe internetoveho pristupu.

---

## Odporucania

### Priorita 1 (NUTNE)

1. **Implementovat read-only mount option pre virtio-fs**
   ```yaml
   shares:
     - host: ~/projects
       guest: /workspace
       mode: rw  # explicit, default by mal byt ro
   ```

2. **Definovat kompletny firewall ruleset**
   ```bash
   # Priklad zakladneho rulesetu
   iptables -P FORWARD DROP
   iptables -A FORWARD -i virbr+ -o eth0 -j ACCEPT  # VM -> internet
   iptables -A FORWARD -i virbr+ -o tun0 -j DROP     # VM -> VPN block
   iptables -A FORWARD -m conntrack --ctstate ESTABLISHED -j ACCEPT
   ```

3. **Specifikovat SSH key management**
   - Generovat per-cage keypair
   - Ukladat v ~/.claude-cage/keys/{cage-name}/
   - Mazat pri `cage stop`

### Priorita 2 (DOPORUCENE)

4. **Pridat network egress logging**
   - Logovat outbound connections
   - Alert na podozrive patterns (vela DNS queries, nezvycajne porty)

5. **Implementovat resource limity explicitne**
   ```yaml
   profiles:
     default:
       vcpu: 4
       memory_mb: 4096
       disk_io_limit: 100MB/s
       network_limit: 10MB/s
   ```

6. **Pridat integrity monitoring pre /workspace**
   - Volitelny git-based tracking zmien
   - Alert na zmeny v binarnych suboroch

### Priorita 3 (NICE-TO-HAVE)

7. **DNS-over-HTTPS vo VM**
8. **Audit logging pre cage operacie**
9. **Podpora pre user namespaces vo VM**

---

## Porovnanie s alternativami

| Feature | Claude Cage | Docker | gVisor | Firecracker |
|---------|-------------|--------|--------|-------------|
| Izolacia | HW (KVM) | Kernel ns | User-space kernel | HW (KVM) |
| Docker kompatibilita | 100% | 100% | ~95% | ~90% |
| Escape difficulty | Velmi vysoka | Stredna | Vysoka | Velmi vysoka |
| Performance overhead | ~5% | ~1% | ~30% | ~3% |
| Privileged containers | Ano | Ano | Nie | Nie |

Claude Cage ponuka dobry kompromis medzi bezpecnostou a funkcionalitou.

---

## Zaver

**Hodnotenie: POTREBUJE UPRAVY (minor)**

Claude Cage dizajn je **fundamentalne spravny** - pouzitie QEMU/KVM poskytuje silnu izolacnu hranicu, ktora je vhodna pre sandbox prostredie kde Claude Code bezi v "yolo mode".

**Hlavne pozitiva:**
- Hardverova virtualizacia je spravna volba
- Ephemeral VMs su vyborny bezpecnostny pattern
- Blokovanie VPN pristupu chrani firemnu infrastrukturu

**Hlavne nedostatky na opravu:**
- Virtio-fs potrebuje read-only opciu
- Firewall pravidla musia byt explicitne definovane
- SSH key management vyzaduje specifikaciu

Po implementacii odporucani Priority 1 bude dizajn vhodny pre:
- Vyvoj a experimentovanie s AI agentmi
- Testovanie nedoveryhodneho kodu
- Sandbox pre Claude Code s plnym Docker

**NIE JE vhodny pre:**
- Spracovanie vysoko citlivych dat (credentials, production secrets)
- Multi-tenant hosting
- Compliance-kriticke prostredia (bez dalsich auditov)

---

*Report generovany ako sucas bezpecnostneho auditu Claude Cage projektu.*
