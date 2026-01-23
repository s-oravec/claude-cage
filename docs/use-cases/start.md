# cage start

Spustí novú cage VM.

## Použitie

```bash
cage start [flags]
```

## Flags

| Flag | Skratka | Popis | Default |
|------|---------|-------|---------|
| `--name` | `-n` | Názov cage | basename aktuálneho adresára |
| `--base` | `-b` | Base image (ubuntu-24.04, debian-12, ...) | z configu |
| `--image` | `-i` | Custom image (vytvorený cez `cage image save`) | - |
| `--profile` | `-p` | Resource profil (default/heavy/light) | `default` |
| `--memory` | `-m` | RAM v MB (override profilu) | z profilu |
| `--cpus` | `-c` | Počet vCPU (override profilu) | z profilu |
| `--port` | | Port mapping `host:guest` (opakovateľný) | žiadne |
| `--share` | `-s` | Zdieľaný adresár `host:guest` (opakovateľný) | aktuálny adresár:/workspace |

## Príklady

```bash
# Základné spustenie (názov = aktuálny adresár, default base image)
cage start

# Pomenovaný cage
cage start --name backend

# S konkrétnym base image
cage start --name backend --base debian-12

# S custom image (vytvorený cez cage image save)
cage start --name dev --image nodejs-python

# S profilom
cage start --name ml --profile heavy

# Override resources
cage start --name test --memory 1024 --cpus 2

# S portami
cage start --name api --port 8080:80 --port 5432:5432

# S viacerými zdieľanými adresármi
cage start --name dev --share ~/data:/data --share ~/config:/config

# Kombinácia
cage start -n webapp -p heavy -b ubuntu-24.04 --port 3000:3000
```

## Správanie

1. Vyberie image (--image > --base > default z configu)
2. Vytvorí kópiu image (ephemeral qcow2)
3. Vygeneruje SSH kľúče a cloud-init
4. Vytvorí bridge interface pre cage
5. Spustí virtiofsd pre zdieľané adresáre
6. Nakonfiguruje iptables (blokuje VPN a privátne subnety)
7. Spustí QEMU/KVM VM cez libvirt
8. Počká na boot VM (cloud-init)
9. Zobrazí info o pripojení

## Výstup

```
✓ Cage "backend" started
  SSH:    cage ssh backend
  Docker: cage ssh backend "docker ps"
  Ports:  8080 → 80, 5432 → 5432
```

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage already exists` | Cage s týmto menom už beží | Použiť iný názov alebo `cage stop` |
| `KVM not available` | /dev/kvm nedostupné | `sudo usermod -aG kvm $USER` |
| `port already in use` | Port obsadený | Zmeniť port mapping |
| `image not found` | Base/custom image neexistuje | `cage setup --base X` alebo `cage image list` |
| `no base image` | Cage nebol inicializovaný | Spustiť `cage setup` |
