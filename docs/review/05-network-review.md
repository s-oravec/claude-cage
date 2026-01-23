# Claude Cage - Network Security Review

**Dátum:** 2026-01-23
**Revízor:** Network Security Expert
**Verzia dokumentácie:** Design Document v1

---

## Zhrnutie

Claude Cage implementuje sieťovú izoláciu pre VM bežiace v QEMU/KVM prostredia s cieľom:
- Povoliť prístup na verejný internet
- Blokovať prístup k VPN interfaces (tun0, tailscale0)
- Umožniť port forwarding z VM na host

Celkové hodnotenie: **Koncept je správny, ale implementačné detaily chýbajú.** Dokumentácia popisuje zámer, ale neobsahuje konkrétne iptables pravidlá ani validáciu bezpečnostných predpokladov.

---

## Silné stránky

### 1. Správny architektonický prístup
- VM-level izolácia poskytuje silnú bezpečnostnú hranicu
- QEMU/KVM + libvirt je overená kombinácia pre sandboxing
- Ephemeral VM dizajn eliminuje perzistenciu kompromitovaného stavu

### 2. Explicitný blocklist pre VPN interfaces
```yaml
network:
  blocked_interfaces:
    - tun0
    - tailscale0
```
- Konfigurácia jasne definuje blokované interfaces
- Používateľ môže pridať ďalšie interfaces podľa potreby

### 3. Verejné DNS servery
```yaml
dns:
  - 1.1.1.1
  - 8.8.8.8
```
- Zabraňuje DNS únikom cez firemné/VPN DNS servery
- Cloudflare a Google DNS sú spoľahlivé a dobre známe

### 4. Multi-layer security
- Libvirt sandboxing (sVirt, SELinux/AppArmor)
- Seccomp profily pre QEMU proces
- Cgroups limity pre resources
- Filesystem izolácia (len /workspace zdieľaný)

### 5. Port forwarding flexibilita
- Dynamické pridávanie/odoberanie portov za behu
- Explicitný limit 50 portov na cage
- Podpora TCP aj UDP protokolov

---

## Slabé stránky / Riziká

### 1. KRITICKÉ: Chýba implementácia iptables pravidiel

**Problém:** Dokumentácia hovorí o "iptables/nftables na hoste", ale neobsahuje konkrétne pravidlá.

**Riziko:** Bez správnej implementácie VM môže mať plný prístup k VPN.

**Očakávané pravidlá (chýbajú):**
```bash
# Blokovať traffic z VM bridge na VPN interfaces
iptables -I FORWARD -i virbr-cage -o tun0 -j DROP
iptables -I FORWARD -i virbr-cage -o tailscale0 -j DROP

# Blokovať VPN subnety (ak interface neexistuje ale routing áno)
iptables -I FORWARD -i virbr-cage -d 10.0.0.0/8 -j DROP
iptables -I FORWARD -i virbr-cage -d 172.16.0.0/12 -j DROP
iptables -I FORWARD -i virbr-cage -d 192.168.0.0/16 -j DROP

# Povoliť internet
iptables -A FORWARD -i virbr-cage -o eth0 -j ACCEPT
```

### 2. VYSOKÉ: Blokovanie interfaces nestačí

**Problém:** Blokovanie len podľa interface name (`tun0`, `tailscale0`) nie je dostatočné.

**Scenáre obídenia:**
1. **Interface renaming:** VPN môže použiť iný interface name (napr. `tun1`, `wg0`)
2. **Static routes:** VPN traffic môže ísť cez default gateway ak interface neexistuje
3. **Split tunneling:** Niektoré VPN konfigurácie routujú len určitý traffic cez VPN

**Odporúčanie:** Okrem interface blokovania treba blokovať aj privátne IP rozsahy.

### 3. VYSOKÉ: Chýba validácia DNS konfigurácie

**Problém:** DNS servery sú nastavené v config.yaml, ale nie je jasné:
- Kde sa DNS aplikuje (v `/etc/resolv.conf` vo VM?)
- Či je DNS traffic vynútený cez tieto servery
- Či VM nemôže použiť iný DNS server

**Riziko:** VM môže použiť firemný DNS a resolvovať interné hostname.

**Odporúčanie:**
```bash
# Vynútiť DNS cez povolené servery (na hoste)
iptables -t nat -I PREROUTING -i virbr-cage -p udp --dport 53 -j DNAT --to 1.1.1.1:53
iptables -t nat -I PREROUTING -i virbr-cage -p tcp --dport 53 -j DNAT --to 1.1.1.1:53
```

### 4. STREDNÉ: Port forwarding security

**Problém:** Port forwarding je flexibilný, ale:
- Nie je dokumentované, ktoré porty sú blokované (privilegované < 1024?)
- Nie je jasné, či forwarding počúva na 0.0.0.0 alebo 127.0.0.1
- Chýba rate limiting a connection limiting

**Riziko:**
- Ak port forwarding počúva na všetkých interfaces, VM servisy sú dostupné z internetu
- DoS útoky cez port forwarding

**Odporúčanie:**
- Default binding na 127.0.0.1
- Explicitná konfigurácia pre 0.0.0.0 binding
- Connection limiting cez iptables

### 5. STREDNÉ: Chýba blokovanie metadata služieb

**Problém:** Cloud providers používajú link-local adresy pre metadata služby:
- AWS: `169.254.169.254`
- GCP: `metadata.google.internal`
- Azure: `169.254.169.254`

**Riziko:** Ak host beží v cloude, VM môže získať credentials cez metadata endpoint.

**Odporúčanie:**
```bash
iptables -I FORWARD -i virbr-cage -d 169.254.169.254 -j DROP
```

### 6. NÍZKE: Dynamická detekcia VPN interfaces

**Problém:** Config obsahuje statický zoznam blokovaných interfaces. Ak používateľ pridá novú VPN, musí manuálne upraviť config.

**Odporúčanie:**
- Automatická detekcia VPN interfaces pri štarte (typ `tun`, `tap`, `wg`)
- Blokovanie interfaces podľa typu, nie len podľa mena

---

## Chýbajúca konfigurácia

### 1. Konfigurácia blokovaných subnetov
```yaml
# CHÝBA v config.yaml
network:
  blocked_subnets:
    - 10.0.0.0/8        # Private Class A
    - 172.16.0.0/12     # Private Class B
    - 192.168.0.0/16    # Private Class C
    - 169.254.0.0/16    # Link-local (metadata)
    - fc00::/7          # IPv6 ULA
```

### 2. Konfigurácia port forwarding security
```yaml
# CHÝBA v config.yaml
network:
  port_forwarding:
    bind_address: 127.0.0.1  # default, nie 0.0.0.0
    allow_privileged: false   # blokovať < 1024
    max_connections: 100      # per port
```

### 3. Konfigurácia DNS enforcement
```yaml
# CHÝBA v config.yaml
network:
  dns:
    servers:
      - 1.1.1.1
      - 8.8.8.8
    force: true  # redirect all DNS queries
```

### 4. Egress filtering
```yaml
# CHÝBA v config.yaml
network:
  egress:
    allow_http: true
    allow_https: true
    allow_ssh: false      # blokovať outbound SSH
    allow_all: false      # default deny
```

### 5. IPv6 konfigurácia
```yaml
# CHÝBA v config.yaml
network:
  ipv6:
    enabled: false  # bezpečnejšie vypnúť ak nie je potrebné
```

---

## Odporúčania

### Priorita 1 (Kritické)

1. **Implementovať a zdokumentovať iptables pravidlá**
   - Vytvoriť dedikovaný chain pre cage traffic
   - Explicitne logovať blokovaný traffic
   - Pridať pravidlá do dokumentácie

2. **Blokovať privátne IP rozsahy okrem interface blokovania**
   - RFC 1918 ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
   - Link-local (169.254.0.0/16)

3. **Vynútiť DNS konfiguráciu**
   - DNAT všetky DNS requesty na povolené servery
   - Blokovať DoH/DoT na nepovolené servery

### Priorita 2 (Vysoké)

4. **Port forwarding security**
   - Default bind na localhost
   - Dokumentovať bezpečnostné implikácie 0.0.0.0 binding
   - Implementovať connection limiting

5. **Automatická detekcia VPN interfaces**
   - Blokovať všetky tun/tap/wg interfaces
   - Warning ak nový VPN interface vznikne počas behu cage

6. **Konfigurovateľné blokované subnety**
   - Pridať `blocked_subnets` do config.yaml
   - Default obsahuje RFC 1918 + link-local

### Priorita 3 (Stredné)

7. **IPv6 security**
   - Default disabled alebo rovnaké pravidlá ako IPv4
   - Blokovať IPv6 ULA ranges

8. **Egress filtering**
   - Voliteľné obmedzenie odchádzajúcich portov
   - Logging egress traffic pre audit

9. **Network monitoring**
   - Sledovanie blokovaných connection attempts
   - Alerting pri podozrivej aktivite

---

## Môže VM obísť sieťové obmedzenia?

### Potenciálne vektory útoku

| Vektor | Riziko | Mitigácia |
|--------|--------|-----------|
| DNS tunneling | Stredné | Force DNS + monitor veľkosť odpovedí |
| ICMP tunneling | Nízke | Blokovať ICMP alebo rate limit |
| HTTP tunneling cez port 443 | Vysoké | Egress filtering, deep packet inspection |
| IPv6 bypass | Vysoké | Disablovať IPv6 alebo rovnaké pravidlá |
| Metadata service access | Stredné | Blokovať 169.254.0.0/16 |
| Static routes v VM | Nízke | VM nemá prístup k host routing table |

### Efektívnosť aktuálneho návrhu

**Aktuálny návrh blokuje:** Priamy prístup k VPN interfaces

**Aktuálny návrh NEblokuje:**
- Privátne IP rozsahy ak routing existuje
- DNS úniky
- Metadata služby
- IPv6 traffic

**Verdict:** VM môže potenciálne obísť obmedzenia ak:
1. VPN routing je cez default gateway (nie dedikovaný interface)
2. Firemný DNS je dostupný cez internet
3. Host beží v cloude s metadata service

---

## Je port forwarding bezpečný?

### Aktuálny stav
- Flexibilné pridávanie/odoberanie portov
- TCP/UDP podpora
- Limit 50 portov

### Bezpečnostné obavy

1. **Bind address:** Ak port forwarding počúva na 0.0.0.0, služby sú dostupné z internetu

2. **Privilegované porty:** Ak VM môže forwardovať port 22, 80, 443, môže to kolidovať s host službami

3. **Reverse tunneling:** VM môže vytvoriť reverse tunnel cez povolený port

### Odporúčania
- Default bind na 127.0.0.1
- Explicitná konfigurácia pre externý prístup
- Logging všetkých port forwarding operácií

---

## Záver

Claude Cage má **správny bezpečnostný koncept** pre sieťovú izoláciu, ale **implementačné detaily sú nedostatočné**.

**Hlavné nedostatky:**
1. Chýbajú konkrétne iptables pravidlá
2. Blokovanie len interface names je nedostatočné
3. DNS konfigurácia nie je vynútená
4. Chýba blokovanie privátnych IP rozsahov

**Pred produkčným nasadením je potrebné:**
1. Implementovať kompletné iptables pravidlá
2. Pridať blokovanie privátnych subnetov do konfigurácie
3. Vynútiť DNS cez povolené servery
4. Zabezpečiť port forwarding (default localhost binding)
5. Disablovať alebo zabezpečiť IPv6

**Odhadované riziko po implementácii odporúčaní:** Nízke

**Aktuálne riziko (len podľa dokumentácie):** Stredné až Vysoké - závisí od implementácie

---

*Tento report je založený na analýze design dokumentácie. Pre kompletné hodnotenie je potrebná revízia implementácie (Go kód, iptables pravidlá, libvirt XML konfigurácie).*
