# SSH Review - Claude Cage

**Dátum:** 2026-01-23
**Reviewer:** Linux sysadmin
**Scope:** SSH prístup, generovanie kľúčov, konfigurácia, výkon

---

## Zhrnutie

Dokumentácia popisuje SSH ako primárny spôsob prístupu do cage VM. Návrh je koncepčne správny - používateľ sa pripája z hosta do VM cez štandardný SSH protokol. Avšak **dokumentácia neobsahuje kritické implementačné detaily** o generovaní kľúčov, ich distribúcii do VM, ani SSH daemon konfigurácii.

---

## Silné stránky

### 1. Jednoduchý UX
```bash
cage ssh myproject                    # interaktívny shell
cage ssh myproject "docker ps"        # spustenie príkazu
cage ssh myproject --user root        # ako root
```
CLI je intuitívne a kopíruje štandardnú SSH konvenciu.

### 2. Jasná séparácia zodpovedností
- Host: cage CLI, SSH klient
- VM: SSH daemon, workspace
- Zdieľanie len cez virtio-fs (nie cez SSH)

### 3. Default user `cage`
Použitie dedikovaného usera namiesto `root` je správna bezpečnostná prax.

### 4. Workspace ako default CWD
Po SSH sa používateľ ocitne v `/workspace` - logické pre workflow.

---

## Problémy / Chýbajúce detaily

### KRITICKÉ: Generovanie SSH kľúčov

**Stav:** Dokumentácia neopisuje, ako a kedy sa generujú SSH kľúče.

**Otázky bez odpovede:**
- Kedy sa generujú? Pri `cage setup`? Pri `cage start`? Pri prvom `cage ssh`?
- Kde sa ukladajú? `~/.claude-cage/keys/` je zmienený, ale nie detaily.
- Aký algoritmus? RSA? Ed25519?
- Aká dĺžka kľúča?
- Sú kľúče per-cage alebo globálne?

**Odporúčanie:**
```bash
# Malo by byť zdokumentované:
~/.claude-cage/keys/
  id_ed25519           # privátny kľúč (len na hoste)
  id_ed25519.pub       # verejný kľúč (injektovaný do VM)
```

**Bezpečnostný štandard:**
- **Ed25519** - moderný, rýchly, bezpečný (256-bit)
- Alternatívne RSA-4096 pre legacy kompatibilitu

### KRITICKÉ: Distribúcia kľúčov do VM

**Stav:** Nie je zdokumentované, ako sa verejný kľúč dostane do VM.

**Pravdepodobné riešenie (na základe zmienky o cloud-init):**
1. Pri `cage start` sa vytvorí cloud-init ISO/NoCloud
2. Verejný kľúč sa vloží do user-data
3. VM pri boote spustí cloud-init
4. cloud-init nakonfiguruje `~/.ssh/authorized_keys`

**Chýba dokumentácia:**
- Formát cloud-init user-data
- Či sa používa meta-data (nastavenie hostname)
- Či je cloud-init skutočne použitý (vs. virt-customize)

**Príklad cloud-init (malo by byť zdokumentované):**
```yaml
#cloud-config
users:
  - name: cage
    groups: docker,sudo
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - ssh-ed25519 AAAAC3Nz... cage@host
```

### STREDNÉ: SSH daemon konfigurácia

**Stav:** Nie je zmienené, aká je SSH konfigurácia vo VM.

**Odporúčaná konfigurácia pre bezpečnosť:**
```bash
# /etc/ssh/sshd_config vo VM
PermitRootLogin prohibit-password    # alebo no
PasswordAuthentication no             # len kľúče
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys
MaxAuthTries 3
LoginGraceTime 30
```

### STREDNÉ: Vlastné kľúče používateľa

**Stav:** Nie je riešené.

**Use case:** Používateľ chce použiť existujúci SSH kľúč (napr. `~/.ssh/id_ed25519`).

**Navrhované riešenie:**
```bash
cage start --name myvm --ssh-key ~/.ssh/id_ed25519.pub
# alebo v configu:
ssh:
  key: ~/.ssh/id_ed25519
```

**Výhody:**
- Konzistentné s používateľovým workflow
- Môže použiť kľúč s hardware tokenom (YubiKey)
- Jednoduchšia správa (menej kľúčov)

### NÍZKE: Výkon SSH pripojenia

**Stav:** Nie sú metriky ani benchmark.

**Očakávaný výkon:**
- Prvé pripojenie: ~100-300ms (TCP handshake + SSH handshake)
- Následné pripojenia: ~50-100ms (ak je ControlMaster)
- Príkaz cez `cage ssh name "cmd"`: závisí od SSH overhead

**Odporúčanie - ControlMaster:**
```bash
# ~/.ssh/config (generované cage-om)
Host cage-*
    ControlMaster auto
    ControlPath ~/.claude-cage/sockets/%r@%h:%p
    ControlPersist 600
```
Toto by výrazne zrýchlilo opakované pripojenia a `cage ssh name "cmd"`.

### NÍZKE: Chybová hlásenia

**Stav:** Tabuľka chýb je minimalistická.

**Chýbajúce scenáre:**
| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `Permission denied (publickey)` | Kľúč nie je v authorized_keys | Reštart cage |
| `Host key verification failed` | VM má iný host key | `cage ssh --force` alebo vymazať known_hosts |
| `Connection timed out` | VM ešte nebootla / sieť | Počkať, skontrolovať `cage status` |
| `No route to host` | Bridge nefunguje | `cage restart` |

### NÍZKE: Known hosts management

**Stav:** Nie je riešené.

**Problém:** VM host key sa mení pri každom štarte (ephemeral). Štandardný SSH klient by hlásil "WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!"

**Riešenie (malo by byť implementované):**
```bash
# cage ssh wrapper by mal použiť:
ssh -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -o LogLevel=ERROR \
    cage@$VM_IP
```
Alebo lepšie - ukladať host key per-cage do `~/.claude-cage/known_hosts/`.

---

## Odporúčania

### Priorita 1 (Blocker pre implementáciu)

1. **Zdokumentovať generovanie kľúčov:**
   - Algoritmus: Ed25519 (default), RSA-4096 (fallback)
   - Kedy: pri `cage setup` (globálny) alebo `cage start` (per-cage)
   - Kde: `~/.claude-cage/keys/`
   - Práva: 0600 pre privátny, 0644 pre verejný

2. **Zdokumentovať cloud-init:**
   - Formát user-data YAML
   - Ako sa vytvára ISO/NoCloud
   - Aké služby sa konfigurujú (sshd, docker, ...)

3. **Zdokumentovať SSH config vo VM:**
   - Explicitne vypnúť password auth
   - Nastaviť primerané timeouty

### Priorita 2 (Dôležité pre UX)

4. **Podpora vlastných kľúčov:**
   ```bash
   cage start --ssh-key ~/.ssh/id_ed25519.pub
   ```

5. **ControlMaster pre rýchle pripojenia:**
   - Automaticky konfigurovať v SSH wrapper

6. **Vyriešiť known_hosts:**
   - Buď ignorovať (menej bezpečné)
   - Alebo per-cage known_hosts (bezpečnejšie)

### Priorita 3 (Nice to have)

7. **Diagnostický príkaz:**
   ```bash
   cage ssh-check myproject
   # → Overí SSH konektivitu, kľúče, ...
   ```

8. **Lepšie chybové hlásenia:**
   - Parsovať SSH stderr
   - Poskytnúť context-aware riešenia

---

## Záver

SSH návrh je **koncepčne správny**, ale **dokumentácia je nekompletná**. Chýbajú kritické detaily o:

1. **Generovaní kľúčov** - bez tohto nie je možná implementácia
2. **Cloud-init integrácii** - ako sa kľúče dostanú do VM
3. **SSH daemon konfigurácii** - bezpečnostné nastavenia

Pre implementáciu je potrebné doplniť tieto detaily. Odporúčam vytvoriť samostatný dokument `docs/design/ssh-keys.md` s technickými detailmi.

**Hodnotenie:** 6/10 (dobrý koncept, nekompletná špecifikácia)

---

## Appendix: Navrhovaný flow SSH kľúčov

```
cage setup
  │
  ├─→ Generuje ~/.claude-cage/keys/id_ed25519{,.pub}
  │   (ak neexistujú)
  │
  └─→ Uloží verejný kľúč do base image (virt-customize)
      ALEBO pripraví cloud-init template

cage start --name myvm
  │
  ├─→ Vytvorí cloud-init ISO s verejným kľúčom
  │   (ak sa používa cloud-init)
  │
  ├─→ Spustí VM s cloud-init ISO ako cdrom
  │
  └─→ cloud-init vo VM:
      1. Vytvorí user 'cage'
      2. Nastaví authorized_keys
      3. Reštartuje sshd

cage ssh myvm
  │
  ├─→ Získa IP adresu VM (libvirt API)
  │
  ├─→ Exec: ssh -i ~/.claude-cage/keys/id_ed25519 \
  │         -o StrictHostKeyChecking=no \
  │         cage@$VM_IP
  │
  └─→ Interaktívny shell v /workspace
```
