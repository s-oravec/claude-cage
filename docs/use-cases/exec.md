# cage exec

Spustí príkaz v cage bez alokácie TTY (rýchlejšie ako SSH).

## Použitie

```bash
cage exec <name> -- <command> [args...]
```

## Argumenty

| Argument | Popis | Povinný |
|----------|-------|---------|
| `name` | Názov cage | áno |
| `command` | Príkaz na spustenie | áno |

## Flags

| Flag | Skratka | Popis |
|------|---------|-------|
| `--tty` | `-t` | Alokuje TTY (pre interaktívne príkazy) |
| `--interactive` | `-i` | Interaktívny režim (stdin) |
| `--user` | `-u` | Spustiť ako iný user |

## Príklady

```bash
# Rýchly príkaz
cage exec backend -- docker ps

# Pipeline
cage exec backend -- docker logs app 2>&1 | grep error

# Viac príkazov
cage exec backend -- sh -c "cd /workspace && npm test"

# Interaktívny (ako SSH)
cage exec -it backend -- bash

# Ako root
cage exec -u root backend -- apt update
```

## Rozdiel oproti SSH

| Aspekt | `cage ssh` | `cage exec` |
|--------|------------|-------------|
| TTY | áno (default) | nie (default) |
| Latencia | vyššia | nižšia |
| Interaktívne | áno | voliteľne |
| Scripting | možné | lepšie |

## Kedy použiť

- Scripty a automatizácia
- Jednoduché príkazy kde nepotrebuješ shell
- Keď potrebuješ rýchlejšiu odozvu

## Implementácia

Používa `virsh console` alebo QEMU Guest Agent namiesto SSH.

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `cage not found` | Cage neexistuje | Skontrolovať `cage list` |
| `command failed` | Príkaz skončil s chybou | Skontrolovať exit code |
| `guest agent not responding` | QEMU GA nie je nainštalovaný | Použiť `cage ssh` namiesto exec |
