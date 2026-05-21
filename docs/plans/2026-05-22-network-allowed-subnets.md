# Configurable network isolation (allowed_subnets + isolation toggle) - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let a project opt out of, or poke holes in, the default SLIRP network isolation via `.cage.yml`: `network.allowed_subnets: [CIDR...]` (keep isolation, allow specific subnets) and `network.isolation: false` (disable entirely).

**Architecture:** Today auto/SLIRP cages always block RFC1918 + link-local via cloud-init `ip route add unreachable` (hardcoded in `internal/cloudinit/generate.go`; switched on unconditionally in `internal/cmd/start.go:309` with allowed subnets hardcoded to the SLIRP net). We thread two new `.cage.yml` `network` fields through `ProjectNetwork` -> `ResolvedConfig` -> `start.go` -> the cloud-init config, and fix the generator so allowed subnets become real routes via the SLIRP gateway (the current code only emits comments for them). Host-side bridge/root isolation is unchanged.

**Tech Stack:** Go, yaml.v3, cobra.

**Design decisions (validated with the user):**
- `isolation` defaults to ON (`*bool` nil => true); only `network.isolation: false` disables.
- `allowed_subnets` are extra subnets reachable while isolation stays on; they only apply to the auto/SLIRP path (bridge/root mode uses the host firewall and `network.blocked_subnets`, untouched here).
- The SLIRP network `10.0.2.0/24` is directly connected, so it gets NO via-gateway route; only user subnets get `ip route add <subnet> via 10.0.2.2`.

**Conventions for the executor:**
- After each task: `go build ./...` and `go test ./internal/...` MUST stay green. (`test/e2e` needs real VMs/images and fails in this env - ignore it.)
- TDD: failing test first, watch it fail, implement, watch it pass, commit.
- Every commit message ends with: `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>`
- ASCII only in comments/docs (no em-dash, arrow glyphs, section sign).
- Leave the untracked `.claude/` alone.

---

## Task 1: SLIRP constants in cloudinit

**Files:** `internal/cloudinit/generate.go`, `internal/cloudinit/generate_test.go`

**Step 1:** Add exported constants near the top of generate.go:
```go
// SLIRP user-mode networking fixed addresses (QEMU defaults).
const (
	SLIRPNetwork = "10.0.2.0/24" // directly-connected guest subnet
	SLIRPGateway = "10.0.2.2"    // NAT gateway; egress and allowed-subnet routes go via here
)
```

**Step 2:** TDD - write a trivial test asserting the constant values (`generate_test.go`), run it (it passes once constants exist; this task is mostly groundwork). Build.

**Step 3:** Commit: `feat(cloudinit): export SLIRP network/gateway constants`

---

## Task 2: cloud-init installs real routes for allowed subnets

**Files:** `internal/cloudinit/generate.go` (`generateNetworkIsolationRuncmd`), `internal/cloudinit/generate_test.go`

Currently `generateNetworkIsolationRuncmd(allowedSubnets)` only emits a COMMENT for each allowed subnet (lines ~48-51) and `unreachable` routes for the hardcoded blocked list. Fix it so each allowed subnet that is NOT the connected SLIRP network gets a real, more-specific route via the SLIRP gateway, and persist those alongside the unreachable routes.

**Step 1: Failing test** in generate_test.go:
- `TestNetworkIsolationRuncmd_AllowedSubnetGetsGatewayRoute`: call `generateNetworkIsolationRuncmd([]string{SLIRPNetwork, "192.168.1.0/24"})`. Assert the output:
  - contains `ip route add unreachable 10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16`
  - contains `ip route add 192.168.1.0/24 via 10.0.2.2`
  - does NOT contain `ip route add 10.0.2.0/24 via 10.0.2.2` (the connected SLIRP net must not get a gateway route)
  - the allowed gateway route is ALSO present in the two persistence blocks (if-up.d and networkd-dispatcher) - i.e. the via-gateway route appears in the persisted scripts too, not only the live runcmd.
- `TestNetworkIsolationRuncmd_NoExtraAllowed`: call with `[]string{SLIRPNetwork}` only; assert the unreachable routes are present and there is no stray `via 10.0.2.2` route line.

Run it -> FAIL (current code emits only comments).

**Step 2: Implement.** Rework `generateNetworkIsolationRuncmd` so:
- it still emits the four `unreachable` blocked routes (live + both persistence blocks);
- for each allowed subnet `!= SLIRPNetwork`, it emits `ip route add <subnet> via <SLIRPGateway>` in the live runcmd AND in both persistence scripts (so allowed routes survive reboot just like the blocked ones). Order: the via-gateway allowed routes can be added after the unreachable routes - `ip` longest-prefix match makes the more-specific allowed /24 win over the /16 unreachable regardless of insertion order, but add a brief comment explaining the precedence.
- keep the `2>/dev/null || true` resilience and the `if-up.d` + `networkd-dispatcher` dual persistence already present.

Consider extracting a small helper that, given (blocked, allowed, gateway), returns the ordered list of `ip route ...` command strings, and reuse it for the live block and both persistence blocks (DRY) so the three places cannot drift.

**Step 3:** Tests pass; `go build ./...`, `go test ./internal/cloudinit/`.

**Step 4:** Commit: `feat(cloudinit): install gateway routes for allowed subnets, not just comments`

---

## Task 3: config - network.isolation + network.allowed_subnets in .cage.yml

**Files:** `internal/config/config.go`, `internal/config/config_test.go`

**Step 1: Failing tests** in config_test.go:
- Parse a `.cage.yml` with
  ```yaml
  network:
    allowed_subnets: [192.168.1.0/24]
    isolation: false
  ```
  via `LoadProjectConfig` (write to a temp dir) and assert `project.Network.AllowedSubnets == ["192.168.1.0/24"]` and `*project.Network.Isolation == false`.
- `ResolveProjectConfig` with that project (and a minimal global with a "default" profile - copy the setup other config tests use) yields `resolved.NetworkIsolation == false` and `resolved.AllowedSubnets == ["192.168.1.0/24"]`.
- A project with NO `network.isolation` set yields `resolved.NetworkIsolation == true` (default on) and `resolved.AllowedSubnets == nil/empty`.

Run -> FAIL (fields/resolved values undefined).

**Step 2: Implement.**
- Extend `ProjectNetwork` (config.go:188):
  ```go
  Isolation      *bool    `yaml:"isolation,omitempty"`       // nil => default (on)
  AllowedSubnets []string `yaml:"allowed_subnets,omitempty"` // extra subnets reachable while isolation is on (auto/SLIRP only)
  ```
- Add to `ResolvedConfig`: `NetworkIsolation bool` and `AllowedSubnets []string`.
- In `ResolveProjectConfig`, after the SSH/ports handling:
  ```go
  resolved.NetworkIsolation = project.Network.Isolation == nil || *project.Network.Isolation
  resolved.AllowedSubnets = project.Network.AllowedSubnets
  ```

**Step 3:** Tests pass; build; `go test ./internal/config/`.

**Step 4:** Commit: `feat(config): network.isolation + network.allowed_subnets in .cage.yml`

---

## Task 4: wire start.go to the resolved config

**Files:** `internal/cmd/start.go`

**Step 1:** Replace the hardcoded block at start.go:307-314:
```go
	// Network isolation applies only to the auto/SLIRP path (bridge/root mode
	// is isolated host-side). Default on; .cage.yml network.isolation:false
	// opts out. allowed_subnets add reachable routes while isolation stays on.
	networkIsolation := networkMode == cage.NetworkAuto && resolved.NetworkIsolation
	allowedSubnets := append([]string{cloudinit.SLIRPNetwork}, resolved.AllowedSubnets...)

	if networkIsolation {
		if len(resolved.AllowedSubnets) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Enabling network isolation (LAN blocked except %s)...\n", strings.Join(resolved.AllowedSubnets, ", "))
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "  Enabling network isolation (blocking LAN access)...")
		}
	} else if networkMode == cage.NetworkAuto {
		fmt.Fprintln(cmd.OutOrStdout(), "  Network isolation disabled (cage can reach the LAN)...")
	}
```
(Confirm `strings` is imported in start.go; it is used elsewhere - verify. Confirm `cloudinit` is imported - it is, since GenerateISOWithConfig is called.) Keep passing `NetworkIsolation: networkIsolation, AllowedSubnets: allowedSubnets` into the CloudInitConfig (unchanged lines 327-328).

**Step 2:** `go build ./...`; `go test ./internal/...` (all green). If any existing start.go test asserts the old hardcoded "Enabling network isolation" string unconditionally, update it to the new behavior.

**Step 3:** Manual sanity (no VM needed): `go run ./cmd/cage start --help` still works. (Do not attempt a real start in this env.)

**Step 4:** Commit: `feat(start): honor network.isolation and allowed_subnets from .cage.yml`

---

## Task 5: docs + `cage init` template

**Files:** `internal/cmd/init.go` (generated `.cage.yml` template - add commented-out network examples), the doc that documents `.cage.yml` network settings (search: `grep -rn "allowed_subnets\|network:" docs/ README.md`; likely `docs/modes.md` and/or a config reference - update wherever `.cage.yml` `network.ssh`/`ports` is documented), and `docs/modes.md` (the user-mode bullet currently says "VM-side egress blocking installed via cloud-init iptables rules" - clarify it is route-based and now configurable via `network.isolation` / `network.allowed_subnets`).

Note: do NOT touch `docs/cagefile.md` - that documents the Dockerfile-style build recipe, a different file from `.cage.yml`.

**Step 1:** Add to the `cage init` generated template a commented block:
```yaml
# network:
#   isolation: true                  # block LAN/private ranges (default true)
#   allowed_subnets: [192.168.1.0/24] # extra subnets the cage may reach
```
(Match the template's existing comment style; verify how init.go currently renders the network section.)

**Step 2:** Update the prose docs as above (ASCII only).

**Step 3:** `go build ./...`; if init.go has a golden/template test, update it. `go test ./internal/cmd/ -run Init`.

**Step 4:** Commit: `docs(network): document network.isolation and allowed_subnets`

---

## Final verification
```bash
go build ./...
go test ./internal/... -race
go vet ./...
go run ./cmd/cage init --help && go run ./cmd/cage start --help
```
Expected: all green.

Manual smoke (optional, needs a real cage): a `.cage.yml` with `network.allowed_subnets: [<your-lan>/24]` lets the cage reach that subnet while the rest of RFC1918 stays blocked; `network.isolation: false` gives full LAN access.

## Done criteria
- `.cage.yml` `network.isolation: false` disables SLIRP route isolation; default (absent) keeps it on.
- `network.allowed_subnets` install real `ip route add <subnet> via 10.0.2.2` routes (live + persisted), reachable while the rest stays blocked; the connected `10.0.2.0/24` never gets a gateway route.
- Bridge/root host-side isolation unchanged.
- Docs + init template updated; `docs/cagefile.md` untouched.
- All `internal/...` tests pass.
