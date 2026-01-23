# cage setup

Inicializuje cage - stiahne base image a nakonfiguruje prostredie.

## Použitie

```bash
cage setup [flags]
```

## Flags

| Flag | Popis | Default |
|------|-------|---------|
| `--base` | Base image na stiahnutie | interaktívny výber |
| `--all` | Stiahnuť všetky dostupné base images | false |

## Interaktívny režim

```bash
cage setup
```

```
? Select base image:
  › Ubuntu 24.04 LTS (recommended)
    Ubuntu 22.04 LTS
    Debian 12 (Bookworm)
    Fedora 40
    Alpine 3.19

Downloading ubuntu-24.04...
  [████████████████████████] 100% (285 MB)

Installing Docker into image...
Installing cage-agent...
Configuring SSH...

✓ Base image ready: ubuntu-24.04
✓ Config created: ~/.claude-cage/config.yaml

Run 'cage start' to create your first cage.
```

## Non-interaktívny režim

```bash
# Konkrétny image
cage setup --base debian-12

# Všetky images
cage setup --all
```

## Dostupné base images

| Image | Veľkosť | Popis |
|-------|---------|-------|
| `ubuntu-24.04` | ~285 MB | Ubuntu 24.04 LTS, odporúčané |
| `ubuntu-22.04` | ~270 MB | Ubuntu 22.04 LTS |
| `debian-12` | ~250 MB | Debian Bookworm, stabilný |
| `fedora-40` | ~320 MB | Fedora 40, novšie balíky |
| `alpine-3.19` | ~50 MB | Minimálny, musl libc |

## Čo setup robí

1. Stiahne oficiálny cloud image (napr. z cloud-images.ubuntu.com)
2. Customizuje image:
   - Nainštaluje Docker
   - Pridá cage-agent
   - Nakonfiguruje SSH (cloud-init)
   - Nakonfiguruje systemd služby
3. Uloží do `~/.claude-cage/images/`
4. Vytvorí default config

## Opätovné spustenie

```bash
# Pridať ďalší base image
cage setup --base alpine-3.19

# Aktualizovať existujúci
cage setup --base ubuntu-24.04 --force
```

## Požiadavky

- KVM prístup (`/dev/kvm`)
- libvirt nainštalovaný a bežiaci
- ~1 GB voľného miesta na disk
- Internet pripojenie

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `KVM not available` | /dev/kvm nedostupné | `sudo usermod -aG kvm $USER` a relogin |
| `libvirtd not running` | libvirt služba nebeží | `sudo systemctl start libvirtd` |
| `download failed` | Sieťový problém | Skontrolovať internet pripojenie |
| `insufficient disk space` | Málo miesta | Uvoľniť miesto na disku |
