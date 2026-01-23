# Claude Cage - Implementation Overview

Implementácia rozdelená do 10 fáz. Každá fáza pridáva funkcionalitu na stabilizovanú verziu.

## Fázy

| Fáza | Príkazy | Závisí na | Test |
|------|---------|-----------|------|
| 01 | `version`, `doctor` | - | CLI funguje, prerequisites OK |
| 02 | `config` | 01 | Config loading/editing |
| 03 | `setup` | 02 | Base image stiahnutý |
| 04 | `start`, `stop`, `list` | 03 | VM bootuje, lifecycle |
| 05 | `ssh` | 04 | SSH do VM funguje |
| 06 | (virtiofs) | 05 | /workspace sync |
| 07 | (network) | 06 | VPN blokovaná, internet OK |
| 08 | `exec`, `status`, `logs` | 07 | Monitoring funguje |
| 09 | `port` | 08 | Port forwarding |
| 10 | `restart`, `snapshot` | 09 | Advanced lifecycle |
| 11 | `image` | 10 | Custom images |

## Minimálny viable produkt

**MVP = Fázy 01-07** - Po týchto fázach je systém použiteľný pre hlavný use case:
- Spustenie VM
- SSH do VM
- File sharing cez /workspace
- Network security (blokáda VPN)

## Poradie implementácie

```
01-cli-foundation
       │
       ▼
02-config
       │
       ▼
03-setup-images
       │
       ▼
04-vm-lifecycle ◄─── MVP milestone 1: VM boots
       │
       ▼
05-ssh-access ◄───── MVP milestone 2: Can connect
       │
       ▼
06-virtiofs ◄──────── MVP milestone 3: File sharing
       │
       ▼
07-network-security ◄ MVP COMPLETE: Secure sandbox
       │
       ▼
08-monitoring
       │
       ▼
09-port-forwarding
       │
       ▼
10-advanced-lifecycle
       │
       ▼
11-custom-images ◄─── Full feature set
```

## Testovacia stratégia

Každá fáza má acceptance test:

```bash
# Fáza 01
cage version && cage doctor

# Fáza 02
cage config show

# Fáza 03
cage setup --base ubuntu-24.04

# Fáza 04
cage start --name test && cage list && cage stop test

# Fáza 05
cage start --name test && cage ssh test "whoami" && cage stop test

# Fáza 06
echo "test" > ~/projects/test/file.txt
cage ssh test "cat /workspace/file.txt"  # should show "test"

# Fáza 07
cage ssh test "curl -s https://google.com"  # OK
cage ssh test "ping 192.168.1.1"  # BLOCKED

# Fáza 08
cage status test && cage exec test -- uname -a

# Fáza 09
cage start --name test --port 8080:80
curl localhost:8080  # should work

# Fáza 10
cage snapshot create test --name snap1
cage snapshot restore test --name snap1

# Fáza 11
cage image save test --name my-image
cage start --name test2 --image my-image
```
