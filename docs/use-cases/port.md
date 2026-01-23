# cage port

Spravuje port forwarding pre cage.

## Použitie

```bash
cage port <name> <command> [args...]
```

## Príkazy

| Príkaz | Popis |
|--------|-------|
| `list` | Zobrazí mapované porty |
| `add` | Pridá port mapping |
| `remove` | Odstráni port mapping |

## cage port list

```bash
cage port backend list
```

Výstup:
```
HOST    GUEST   PROTOCOL
8080    80      tcp
5432    5432    tcp
```

## cage port add

Pridá port mapping za behu.

```bash
cage port <name> add <host>:<guest> [flags]
```

Flags:
| Flag | Popis |
|------|-------|
| `--protocol` | tcp/udp (default: tcp) |

Príklady:
```bash
# Pridať port
cage port backend add 3000:3000

# UDP port
cage port backend add 53:53 --protocol udp

# Iný host port
cage port backend add 9090:8080
```

## cage port remove

Odstráni port mapping.

```bash
cage port <name> remove <host>
```

Príklady:
```bash
# Odstrániť port
cage port backend remove 3000

# Odstrániť podľa host portu
cage port backend remove 9090
```

## Príklady workflow

```bash
# Spustiť cage bez portov
cage start --name dev

# Neskôr pridať porty podľa potreby
cage port dev add 3000:3000   # frontend
cage port dev add 5000:5000   # API
cage port dev add 5432:5432   # database

# Zobraziť čo je mapované
cage port dev list

# Odstrániť nepotrebný
cage port dev remove 5000
```

## Obmedzenia

- Port musí byť voľný na hoste
- Zmena portu vyžaduje remove + add
- Maximálne 50 portov na cage

## Chyby

| Chyba | Príčina | Riešenie |
|-------|---------|----------|
| `port already mapped` | Port už existuje | Najprv `remove` |
| `port in use` | Host port obsadený | Použiť iný host port |
| `invalid port range` | Port mimo 1-65535 | Opraviť číslo portu |
