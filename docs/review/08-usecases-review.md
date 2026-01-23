# Claude Cage CLI/UX Review

**Autor:** CLI/UX Designer Review
**Datum:** 2026-01-23

---

## Zhrnutie

Claude Cage CLI je dobre navrhnuty nastroj s intuitivnou hierarchiou prikazov a konzistentnym designom. Use cases pokryvaju vacsinu potrebnych scenariov pre pravu s VM sandboxom. Existuju vsak niektore nekonzistentnosti a chybajuce funkcie, ktore by mali byt adresovane pred uvolnenim do produkcie.

**Celkove hodnotenie:** 7.5/10

| Aspekt | Hodnotenie |
|--------|------------|
| Intuitivnost | 8/10 |
| Konzistentnost | 7/10 |
| Kompletnost | 7/10 |
| Error handling | 8/10 |
| Dokumentacia | 8/10 |
| Vystupny format | 8/10 |

---

## Silne stranky

### 1. Konzistentna struktura prikazov

Prikazy nasleduju logicku hierarchiu:
```
cage <resource> <action> [args] [flags]
```

Priklady:
- `cage image list` / `cage image save` / `cage image delete`
- `cage port add` / `cage port remove` / `cage port list`
- `cage config show` / `cage config edit` / `cage config init`

### 2. Dobre navrhnuty --json flag

Vsetky read prikazy podporuju `--json` flag, co je excelentne pre scripting a integraciu:
- `cage list --json`
- `cage status --json`
- `cage version --json`

### 3. Intuitivne defaulty

- `cage start` bez argumentov pouzije nazov aktualneho adresara
- `cage start` automaticky zdielaj aktualny adresar do `/workspace`
- Profile system (default/heavy/light) eliminuje potrebu pamatat si cisla

### 4. Dobre pokryte chybove stavy

Kazdy use case ma tabulku chyb s pricinami a rieseniami:
```
| Chyba | Pricina | Riesenie |
|-------|---------|----------|
| cage not found | Cage neexistuje | Skontrolovat cage list |
```

### 5. Uzitocny vystupny format

Human-readable vystup s emoji indikatormi (`checkmark`) a strukturovanou informaciou:
```
checkmark Cage "backend" started
  SSH:    cage ssh backend
  Docker: export DOCKER_HOST=tcp://localhost:2376
```

### 6. Watch mode pre monitoring

`cage status --watch` poskytuje real-time monitoring podobny `top`, co je uzitocne pre debugging.

---

## Chybajuce prikazy

### 1. **cage restart** - KRITICKE

**Problem:** Neexistuje prikaz na restart cage bez straty stavu.

**Navrh:**
```bash
cage restart <name> [flags]
cage restart backend
cage restart backend --force
```

### 2. **cage pause/resume** - STREDNE

**Problem:** Nemoznost pozastavit VM bez jej ukoncenia.

**Navrh:**
```bash
cage pause <name>      # suspenduje VM (QEMU save state)
cage resume <name>     # obnovi suspendu VM
```

### 3. **cage exec** - STREDNE

**Problem:** `cage ssh` je vhodne pre shell, ale pre jednorazove prikazy je verbose.

**Navrh:**
```bash
cage exec backend docker ps       # bez uvodzoviek
cage exec -it backend /bin/bash   # interaktivny
```

Porovnanie:
```bash
# Aktualne
cage ssh backend "docker ps"

# Navrhnuty
cage exec backend docker ps
```

### 4. **cage cp** - STREDNE

**Problem:** Kopirovat subory mimo workspace je komplikovane.

**Navrh:**
```bash
cage cp backend:/var/log/app.log ./          # z VM na host
cage cp ./config.yaml backend:/etc/app/      # z host do VM
```

### 5. **cage attach** - NIZKE

**Problem:** Pristup k VM konzole pre debugging boot problemov.

**Navrh:**
```bash
cage attach backend    # priame pripojenie na serial konzolu
```

### 6. **cage snapshot** - NIZKE

**Problem:** QEMU podporuje snapshots, ale CLI to nevystavuje.

**Navrh:**
```bash
cage snapshot create backend --name "before-update"
cage snapshot list backend
cage snapshot restore backend --name "before-update"
```

### 7. **cage doctor** - NIZKE

**Problem:** Diagnostika problemov s prostredim.

**Navrh:**
```bash
cage doctor
# Skontroluje: KVM, libvirt, virtiofsd, permissions, network
```

---

## Problemy v existujucich use cases

### 1. **Nekonzistencia: version.md vs design.md**

**Problem:** `version.md` zobrazuje Firecracker komponenty, ale `design.md` pouziva QEMU/KVM.

**version.md:**
```
Components:
  Firecracker: 1.5.0  # ZLE - design pouziva QEMU
```

**Riesenie:** Aktualizovat `version.md` na:
```
Components:
  QEMU:        8.2.0
  libvirt:     9.0.0
  virtiofsd:   1.8.0
```

### 2. **start.md: Nekonzistentne flags pre resources**

**Problem:** `--cpus` vs design.md pouziva `vcpu`.

**Aktualne:**
```bash
--cpus 2
```

**Odporucanie:** Zjednotit na `--vcpu` alebo aktualizovat config aby pouzival `cpus`.

### 3. **stop.md: TLS certifikaty zmienene bez kontextu**

**Problem:** Zmienuje "TLS certifikaty zostanu" ale nikde nie je vysvetlene na co sa pouzivaju, ked Docker bezia vo VM.

**Riesenie:** Bud odstranit zmienku o TLS certifikatoch (nepotrebne pre novy design), alebo vysvetlit use case.

### 4. **logs.md: Chybajuca info o zlucovani logov**

**Problem:** Logy z roznych zdrojov (boot, docker, ssh) nie su jasne formatovane.

**Odporucanie:** Pridat flag pre filtrovanie:
```bash
cage logs backend --filter docker
cage logs backend --filter boot
```

### 5. **setup.md: Chyba --force flag dokumentacia**

**Problem:** Zmieneny `--force` flag pre prepis existujuceho image, ale nie je v tabulke flagov.

**Riesenie:** Pridat do tabulky:
```
| --force | Prepise existujuci image | false |
```

### 6. **port.md: Chyba validacia pre privilegovane porty**

**Problem:** Nepokryta chyba pri pokuse o binding na port < 1024 bez root.

**Riesenie:** Pridat do tabulky chyb:
```
| permission denied | Host port < 1024 | Pouzit vysi port alebo sudo |
```

### 7. **config.md: Chyba set/get subcommands**

**Problem:** Neexistuje priamy sposob ako zmenit jednu hodnotu bez editora.

**Odporucanie:**
```bash
cage config get profiles.default.memory_mb
cage config set profiles.default.memory_mb 8192
```

### 8. **list.md: --all flag zmatok**

**Problem:** `--all` zobrazuje "aj zastavene (ak existuju certs)" - ale preco certs? Je to pozostatok z Docker TLS designu.

**Riesenie:** Preformulovat alebo zmenit logiku na "aj nedavno zastavene".

### 9. **ssh.md: Chyba -L/-R port forwarding**

**Problem:** SSH prikaz nepodporuje dynamicky port forwarding.

**Odporucanie:**
```bash
cage ssh backend -L 8080:localhost:80    # local forward
cage ssh backend -R 3000:localhost:3000  # remote forward
```

### 10. **image.md: Chyba export/import**

**Problem:** Zmienene `cp` pre zdielanie images, ale neexistuje proper export/import.

**Odporucanie:**
```bash
cage image export data-science --output ./data-science.tar.gz
cage image import ./data-science.tar.gz --name data-science
```

---

## Odporucania

### Vysoka priorita

1. **Opravit Firecracker -> QEMU v `version.md`**
   - Aktualna dokumentacia je mylna

2. **Pridat `cage restart`**
   - Zakladny prikaz ocakavany uzivatelmi

3. **Zjednotit `--cpus` vs `vcpu`**
   - Jednoducha zmena, velky dopad na konzistentnost

4. **Odstranit/aktualizovat TLS referencie**
   - Stary design, myli uzivatelov

### Stredna priorita

5. **Pridat `cage exec`**
   - Lepsie UX pre jednorazove prikazy

6. **Pridat `cage cp`**
   - Uzitocne pre debugging

7. **Rozsirit `cage config` o get/set**
   - Scripting friendly

8. **Pridat `--filter` do `cage logs`**
   - Lepsie debugging

### Nizka priorita

9. **Pridat `cage doctor`**
   - Diagnostika pre nových uzivatelov

10. **Pridat `cage snapshot`**
    - Vyuzit QEMU snapshots

11. **Pridat `cage pause/resume`**
    - Power user feature

---

## Kontrolny zoznam UX principov

| Princip | Status | Poznamka |
|---------|--------|----------|
| Minimalny pocet krokov pre zakladne use cases | PASS | `cage start && cage ssh` |
| Konzistentna syntax | PARTIAL | Niektore nekonzistentnosti (cpus/vcpu) |
| Uzitocne chybove hlasky | PASS | Tabulky s riesieniami |
| JSON vystup pre automatizaciu | PASS | Vsetky read prikazy |
| Rozumne defaulty | PASS | Automaticke nazvy, profily |
| Discoverable help | ? | Nevidel som `--help` priklady |
| Dry-run moznost | FAIL | Chyba `--dry-run` flag |
| Verbose mode | FAIL | Chyba `--verbose/-v` flag |
| Quiet mode | PASS | `cage list -q` |
| Bezpecne defaulty | PASS | Ephemeral, blokovane VPN |

---

## Navrhnuty UX flow pre noveho uzivatela

```
1. cage setup                      # Interaktivny, stiahne image
2. cage start                      # Pouzije aktualne adresare, default profil
3. cage ssh                        # Vstup do VM (ak je len jeden cage)
4. [praca vo VM]
5. cage stop                       # Zastavi (ephemeral cleanup)
```

**Poznamka:** Krok 3 by mohol automaticky detekovat jediny bežiaci cage:
```bash
cage ssh       # Ak je len jeden cage, pripoji sa k nemu
cage ssh       # Ak je viac cage, zobrazi vyber
```

---

## Zaver

Claude Cage CLI ma solidny zaklad s dobrou intuitivnostou a konzistentnym designom. Hlavne problemy su:

1. **Nekonzistentnost v dokumentacii** (Firecracker vs QEMU)
2. **Chybajuce zakladne prikazy** (restart, exec)
3. **Pozostatky stareho designu** (TLS, docker_port)

Po adresovani vysokej priority issues bude CLI pripravene na beta testovanie. Stredna priorita by mala byt adresovana pred GA release.

**Odporucanie:** Vytvorit integration test suite, ktory pokryje vsetky dokumentovane use cases a overi konzistentnost medzi dokumentaciou a implementaciou.

---

## Appendix: Zhrnutie zmien

### Subory na aktualizaciu

| Subor | Zmena |
|-------|-------|
| `version.md` | Firecracker -> QEMU/libvirt |
| `start.md` | `--cpus` -> `--vcpu` (alebo opacne v config) |
| `stop.md` | Odstranit/vysvetlit TLS certifikaty |
| `setup.md` | Pridat `--force` do tabulky flagov |
| `port.md` | Pridat chybu pre privilegovane porty |
| `list.md` | Preformulovat `--all` popis |
| `status.md` | Odstranit docker_port z JSON outputu |

### Nove subory na vytvorenie

| Subor | Priorita |
|-------|----------|
| `restart.md` | Vysoka |
| `exec.md` | Stredna |
| `cp.md` | Stredna |
| `doctor.md` | Nizka |
| `snapshot.md` | Nizka |
| `pause.md` | Nizka |
