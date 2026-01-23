# Review: Claude Cage Scenarios

**Dokument:** `/docs/scenarios/*.md` (8 suborov)
**Reviewer:** Technical Writer / DevOps Praktik
**Datum:** 2026-01-23

---

## Zhrnutie

Scenare pokryvaju 8 praktickych use cases od zakladneho Claude Code sandboxu po custom images. Celkova kvalita je **dobra**, dokumentacia je citatelna a workflow diagramy su uzitocne. Identifikovanych je viacero nekonzistentnosti s dizajnovym dokumentom a chybajuce scenare pre produkcne use cases.

| Aspekt | Hodnotenie |
|--------|------------|
| Realizmus scenárov | 8/10 |
| Pokrytie use cases | 7/10 |
| Správnosť kódu | 6/10 |
| Jasnosť workflow | 9/10 |
| Konzistencia s dizajnom | 6/10 |

---

## Silné stránky

### 1. Jasná štruktúra a čitateľnosť
- Každý scenár má konzistentnú štruktúru: Problém -> Riešenie -> Workflow -> Výhody
- ASCII diagramy efektívne vizualizujú architektúru
- Praktické príklady sú okamžite použiteľné

### 2. Realistické problémy
- Scenáre riešia skutočné bolesti vývojárov (VPN izolácia, čisté prostredia, CI testovanie)
- Bezpečnostné matice jasne komunikujú čo je povolené a čo blokované

### 3. DevOps-friendly príklady
- CI/CD scripty s trap pre cleanup (05-ci-local-testing.md)
- Matrix testing pre viacero verzií
- Docker Compose workflow

### 4. Kompletné workflow príklady
- Od spustenia po cleanup
- Reálne session príklady (ráno začať, večer skončiť)

---

## Chýbajúce scenáre

### Kriticky chýbajúce

| Scenár | Priorita | Zdôvodnenie |
|--------|----------|-------------|
| **Disaster recovery** | Vysoká | Čo robiť keď cage zamrzne alebo sa nedá zastaviť |
| **Network debugging** | Vysoká | Ako diagnostikovať keď sieť nefunguje |
| **Resource exhaustion** | Stredná | Čo keď cage vyčerpá pamäť/disk |
| **Secrets management** | Vysoká | Ako bezpečne predať API kľúče do cage |

### Odporúčané doplniť

| Scenár | Popis |
|--------|-------|
| **E2E testing s Playwright/Cypress** | Headless browser testing v izolácii |
| **GPU workloads** | ML training s GPU passthrough (ak podporované) |
| **Persistent development** | Keď potrebujem zachovať stav medzi reštartmi |
| **Team onboarding** | Krok-za-krokom pre nového člena tímu |
| **VS Code Remote SSH** | Integrácia s populárnym IDE |

---

## Problémy v existujúcich scenároch

### 01-claude-code-sandbox.md

**Problém 1: Nekonzistentnosť s dizajnom**
```bash
# Scenár uvádza:
cage start --name claude-sandbox --profile heavy --port 3000:3000

# Dizajn start.md uvádza aj:
cage start --name X --base Y   # base image parameter
```
Chýba vysvetlenie, aký base image sa použije keď nie je špecifikovaný.

**Problém 2: Workflow bez handling chýb**
```bash
cage ssh claude-sandbox
claude   # Čo keď Claude nie je nainštalované?
```
Chýba informácia, či je Claude Code predinstalovaný v base image.

### 02-docker-development.md

**Problém 3: Chýba `--profile` v druhom príklade**
```bash
# Prvý príklad:
cage start --name myapp --profile heavy --port 3000:3000 --port 5432:5432

# Typický session (nekonzistentný):
cage start --name myapp --port 3000:3000
```
Chýba `--profile heavy` a port 5432.

### 03-ai-agent-testing.md

**Problém 4: Neexistujúci príkaz**
```bash
cage status agent-sandbox --watch
cage logs agent-sandbox -f
```
Podľa `logs.md` je správny flag `--follow` alebo `-f`, ale `status.md` uvádza `--watch`. Dokumentácia je tu správna, ale v scenári by bolo dobré pridať poznámku o ekvivalentnosti.

**Problém 5: Chýba `--force` dokumentácia**
```bash
cage stop agent-sandbox --force
```
Tento flag nie je zdokumentovaný v `stop.md` use case.

### 04-untrusted-code.md

**Problém 6: Nekonzistentná syntax**
```bash
# Scenár:
cage docker sandbox run -v /workspace:/app -w /app node:20 npm install

# Ale dizajn neuvádza `cage docker` príkaz
```
Príkaz `cage docker` nie je v dizajnovom dokumente. Toto je potenciálne budúca feature alebo chyba.

**Problém 7: Bezpečnostná medzera**
```bash
cp -r ~/downloads/suspicious-package ~/projects/sandbox/
```
Kód sa kopíruje na host filesystem PRED analýzou. Ak malware exploituje filesystem vulnerability, host je vystavený riziku ešte pred cage.

### 05-ci-local-testing.md

**Problém 8: HEREDOC syntax**
```bash
cage ssh $CAGE_NAME << 'EOF'
cd /workspace
...
EOF
```
Keď sa používa `$CAGE_NAME` v príkaze, ale `'EOF'` blokuje expanziu vo vnútri, toto je správne. Ale chýba vysvetlenie pre menej skúsených používateľov.

**Problém 9: Matrix testing bez cleanup**
```bash
for version in 18 20 22; do
  cage start --name "node-$version"
  # ...
  cage stop "node-$version"
done
```
Ak zlyhá jeden z krokov, ostatné cage môžu ostať bežať. Chýba `trap` pre cleanup.

### 06-multi-service-dev.md

**Problém 10: Nesprávna syntax príkazu**
```bash
cage ssh backend -c "docker compose up -d"
```
Podľa `ssh.md` je syntax:
```bash
cage ssh backend "docker compose up -d"
```
Flag `-c` nie je dokumentovaný.

**Problém 11: Cross-cage komunikácia**
Scenár naznačuje, že frontend volá `localhost:8080` pre backend, ale nie je jasné, ako to funguje keď sú v rôznych cage. Port forwarding ide cez host, čo je správne, ale diagram to neukazuje jasne.

### 07-database-experimentation.md

**Problém 12: Chýba `psql` inštalácia**
```bash
psql -h localhost -U postgres
```
V base Ubuntu image nie je `psql` client. Treba:
```bash
docker exec -it postgres psql -U postgres
```

**Problém 13: Chýba password**
```bash
DATABASE_URL=postgres://postgres:test@localhost:5432/mydb npm run migrate
```
Správne, ale chýba vysvetlenie, že `test` je password nastavený v `docker run`.

### 08-custom-images.md

**Problém 14: Nekonzistentná flag syntax**
```bash
# Scenár uvádza:
cage start --name setup --base ubuntu-24.04
cage image save setup --name data-science --description "..."

# Ale design uvádza:
cage setup --base ubuntu-24.04   # pre base images
cage start --name temp           # bez --base
```
`--base` flag pri `cage start` vs `cage setup` je matúci.

**Problém 15: Chýba image path**
```bash
cp ~/.claude-cage/images/acme-backend-v1.img /shared/team/
```
Dizajn uvádza `.img` formát, ale `plans/2026-01-23-claude-cage-design.md` hovorí o qcow2. Nesúlad.

**Problém 16: Dátum v príklade**
```
# NAME              TYPE    BASE           SIZE     CREATED
# ubuntu-24.04      base    -              285 MB   2024-01-20
```
Rok 2024 vs aktuálny rok - kozmetický problém, ale mätúci.

---

## Konzistencia s dizajnom

### Zistené rozpory

| Scenár | Rozpor | Dizajn uvádza |
|--------|--------|---------------|
| 04-untrusted-code | `cage docker sandbox run ...` | Príkaz `cage docker` neexistuje |
| 06-multi-service | `cage ssh backend -c "cmd"` | `cage ssh backend "cmd"` |
| 08-custom-images | `.img` formát | qcow2 formát |
| Viacero | Firecracker zmienky v start.md | QEMU/KVM v hlavnom dizajne |

### Chýbajúce v scenároch (existuje v dizajne)

- `cage setup` - inicializácia (zmienená len v 08)
- `cage port add/remove` - dynamické porty
- `cage config` - konfigurácia
- `--share` flag pre viacero zdieľaných adresárov

### V scenároch ale nie v dizajne

- `cage docker` subcommand
- `cage ssh -c` flag
- `cage stop --force` flag

---

## Odporúčania

### Vysoká priorita (pred releasom)

1. **Opraviť `cage docker` príkaz** - buď odstrániť zo scenára 04, alebo pridať do dizajnu
2. **Zjednotiť SSH syntax** - odstrániť `-c` flag alebo ho zdokumentovať
3. **Pridať error handling** - trap v CI scriptoch, čo robiť pri zlyhaní
4. **Dokumentovať `--force` flag** pre `cage stop`

### Stredná priorita

5. **Pridať scenár pre secrets** - ako predať API kľúče bezpečne
6. **Vytvoriť troubleshooting scenár** - čo keď niečo nefunguje
7. **Aktualizovať dátumy** - 2024 -> 2026
8. **Zjednotiť image formát** - .img vs qcow2 v dokumentácii

### Nízka priorita (nice to have)

9. **Pridať VS Code Remote integráciu**
10. **GPU passthrough scenár** (ak podporované)
11. **Video/GIF tutoriály** (link na ne)

---

## Záver

Scenáre sú **solidným základom** pre používateľskú dokumentáciu. Pokrývajú hlavné use cases a poskytujú praktické príklady. Hlavným problémom je **nekonzistencia s dizajnovým dokumentom** - existujú príkazy a flagy, ktoré nie sú v dizajne, a naopak.

**Pred releasom odporúčam:**
1. Synchronizovať scenáre s aktuálnym CLI dizajnom
2. Pridať minimálne 2 scenáre (secrets management, troubleshooting)
3. Vytvoriť automated testing pre kódové príklady

**Celkové hodnotenie:** 7/10 - Dobrý základ, vyžaduje revíziu pred produkčným použitím.

---

*Review vytvorený: 2026-01-23*
