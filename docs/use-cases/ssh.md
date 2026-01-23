# cage ssh

SSH pripojenie do cage VM.

## Použitie

```bash
cage ssh <name> [command]
```

## Argumenty

| Argument | Popis | Povinný |
|----------|-------|---------|
| `name` | Názov cage | áno |
| `command` | Príkaz na spustenie (voliteľné) | nie |

## Flags

| Flag | Popis |
|------|-------|
| `--user` | SSH user (default: `cage`) |

## Príklady

```bash
# Interaktívny shell
cage ssh backend

# Spustiť príkaz
cage ssh backend "docker ps"

# Spustiť viac príkazov
cage ssh backend "cd /workspace && ls -la"

# Ako root
cage ssh backend --user root

# Pipeline
cage ssh backend "cat /etc/os-release" | grep VERSION
```

## Správanie

1. Nájde IP adresu cage
2. Použije SSH kľúč z `~/.claude-cage/keys/`
3. Pripojí sa na port 22 vo VM

## Workspace

Po SSH sa nachádzaš v `/workspace`, čo je zdieľaný adresár s hostom:

```bash
cage ssh backend
pwd
# /workspace
ls
# (súbory z host adresára)
```

## Tips

```bash
# Sledovať logy
cage ssh backend "tail -f /var/log/docker.log"

# Interaktívny docker
cage ssh backend
docker run -it ubuntu bash

# Skopírovať súbor (cez zdieľaný adresár)
# Súbory v workspace sú automaticky zdieľané
```

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage not found` | Cage nebeží | `cage start` |
| `connection refused` | SSH ešte nenaštartoval | Počkať pár sekúnd |
