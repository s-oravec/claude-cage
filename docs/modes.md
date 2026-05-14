# Operating Modes

Claude Cage runs in one of two modes. The mode is chosen by whether the
`cage` process is started as a regular user or as root (`sudo cage`), and
together with the project cagefile determines which features are
available.

## User mode

**How:** `cage start` (no sudo)

**What it gives you:**
- Fully isolated VM with serial console and SSH
- SLIRP user-mode networking (NAT to the host, no LAN access)
- VM-side egress blocking installed via cloud-init iptables rules
- Snapshots, image pull, lifecycle (start/stop/remove)
- Zero host configuration: works out of the box on any KVM-capable Linux
  with the user in the `kvm` and `libvirt` groups

**What it does not give you:**
- Shared folders between host and VM
- Environment variables injected from the host (the `env:` cagefile field)
- libvirt-managed bridge networking
- Host-level network isolation via network namespace + passt

**libvirt backend:** `qemu:///session`. QEMU runs as your regular user; no
privileged operations are involved. State lives under `~/.claude-cage/`.

## Root mode

**How:** `sudo cage start` with a cagefile that uses shared folders, env
injection, or bridge networking

**What it adds over user mode:**
- Shared directories from host to VM via virtiofs (`shares:` in cagefile)
- Environment variable injection via virtiofs (`env:` in cagefile, exposed
  as a sourceable `/cage/runtime/env.sh` inside the VM)
- libvirt-managed bridge networking
- Host-level network isolation: each cage gets its own network namespace
  with passt providing user-mode-style egress, and host iptables rules
  blocking traffic to configured private subnets

**libvirt backend:** `qemu:///system`. QEMU runs under the libvirt-qemu
user with apparmor confinement. State lives under
`/var/lib/libvirt/images/cage/`, which is on the default virt-aa-helper
allow-list, so disk overlays and cloud-init ISOs are readable without
apparmor surgery.

## Choosing a mode

`cage init` generates a user-mode cagefile by default. Add `--root` to
generate one that requests root mode:

```bash
# user mode (default): just a sandbox VM, no host integration
cage init --image alpine

# root mode: workspace share + env injection prerequisites
cage init --image alpine --root
```

Cage enforces the mode at `cage start`: if your cagefile has any field
that requires root mode but you ran `cage start` without sudo, the
command exits with:

```
Error: this cage config requires root mode (shares/env/bridge):
run 'sudo cage start' instead, or remove those fields from .claude-cage.yml
```

You can also switch a user-mode cagefile to root mode by adding a
`shares:` or `env:` block manually:

```yaml
image: ubuntu-24.04
network:
  ssh: auto
shares:
  - host: .
    guest: /workspace
env:
  ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
```

## Doctor checks per mode

`cage doctor` validates user-mode prerequisites by default:

- KVM available
- libvirtd running
- User in `kvm` and `libvirt` groups
- `qemu-img` installed
- `cloud-localds` and `virt-customize` (optional)

`cage doctor --root` additionally validates root-mode prerequisites:

- libvirt system mode reachable (`virsh -c qemu:///system version` works)
- Your home directory is traversable by the QEMU user
  (`libvirt-qemu` on Debian/Ubuntu, `qemu` on Fedora/Arch). When this
  check fails, `cage doctor --root --fix` applies `setfacl -m u:<qemu>:x
  $HOME` automatically (no sudo required — you own your home).
- `virtiofsd` installed

## Why two modes (and not three or one)

Libvirt's session daemon does not support virtiofs filesystem definitions
at all, so a single "always system mode" design forces every user to
configure apparmor, ACLs, and Linux groups before they can launch a VM —
even a trivial sandbox that doesn't need shares. Conversely, a single
"always session mode" design cannot offer the features that motivated
cage in the first place (workspace sharing, env injection).

Splitting the surface lets the common case (run a sandbox) work with zero
host configuration, while the advanced case (host-VM integration) opts
into the trade-offs explicitly via `sudo`. The capability matrix in the
README maps directly to libvirt's own architectural split between session
and system connections.

## Existing user-mode cages and switching

A cage created in user mode is defined in `qemu:///session` with state at
`~/.claude-cage/cages/<name>/`. A cage created in root mode is in
`qemu:///system` with state at `/var/lib/libvirt/images/cage/cages/<name>/`.
They are separate populations: `cage list` as a user shows user-mode
cages, `sudo cage list` shows root-mode cages. To move a workload across
modes, run `cage remove <name>` in the originating mode and recreate it
in the new mode (state is small — disk overlays are recreated from the
base image).

## Troubleshooting root mode

If `sudo cage start` fails with `Permission denied` reading a disk file
or a cloud-init ISO, run `sudo cage doctor --root --fix`. The most common
issues:

- **virt-aa-helper denial in apparmor audit log:** the state path is not
  under `/var/lib/libvirt/images/`. This should not happen with default
  cage paths; if you overrode `CAGE_CONFIG_DIR`, point it back under that
  prefix or edit `/etc/apparmor.d/usr.lib.libvirt.virt-aa-helper` to
  allow your path.
- **`Cannot access storage file ... as uid:64055`:** the file exists but
  libvirt-qemu cannot traverse the path. Run
  `setfacl -m u:libvirt-qemu:x` on each parent directory back to `/`.

## Future work

- Daemon mode with a setuid helper that handles only the privileged
  operations (virtiofsd spawning, netns setup), removing the need to run
  the whole cage CLI as root.
- A `cage migrate <name> --to root` command to relocate a cage between
  modes without recreating the disk overlay.
