# cage config

Zobrazí alebo upraví konfiguráciu.

## Použitie

```bash
cage config [command] [flags]
```

## Príkazy

| Príkaz | Popis |
|--------|-------|
| `show` | Zobrazí aktuálnu konfiguráciu |
| `edit` | Otvorí config v editore |
| `path` | Zobrazí cestu ku config súboru |
| `init` | Vytvorí default config |

## cage config show

```bash
cage config show
```

Výstup:
```yaml
profiles:
  default:
    vcpu: 4
    memory_mb: 4096
  heavy:
    vcpu: 8
    memory_mb: 8192
  light:
    vcpu: 2
    memory_mb: 2048

network:
  blocked_interfaces:
    - tun+
    - tailscale+
    - wg+
  blocked_subnets:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
    - 169.254.0.0/16
  dns:
    - 1.1.1.1
    - 8.8.8.8

shares:
  - host: ~/projects
    guest: /workspace
```

## cage config edit

```bash
cage config edit
```

Otvorí `~/.claude-cage/config.yaml` v `$EDITOR` (alebo vim/nano).

## cage config path

```bash
cage config path
# /home/user/.claude-cage/config.yaml
```

## cage config init

```bash
# Vytvorí default config ak neexistuje
cage config init

# Prepísať existujúci
cage config init --force
```

## Validácia

Config je validovaný pri:
- `cage start`
- `cage config show`

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `config not found` | Config súbor neexistuje | `cage config init` |
| `invalid YAML` | Syntaktická chyba | Opraviť YAML syntax |
| `invalid config` | Neplatná hodnota | Pozrieť chybovú hlášku pre detail |

Príklad chybovej hlášky:
```
Error: invalid config at line 5
  vcpu: must be positive integer
```
