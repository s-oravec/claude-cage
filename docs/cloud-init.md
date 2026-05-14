# Cloud-init

Cage bootstraps every cage VM via [cloud-init](https://cloud-init.io/),
the de-facto standard "first-boot configuration" agent shipped on every
official cloud image (Ubuntu, Debian, Alpine, Rocky, Fedora, openSUSE,
…). This page documents exactly what cage's cloud-init ISO does at first
boot and where to look when something goes wrong.

## Why cloud-init

Base images cage pulls are vanilla cloud distributions. They don't know
what user to create, what SSH key to trust, what env vars to expose, or
what to mount from the host. cloud-init solves this generically: at
first boot the VM looks for a config source (a small ISO image attached
as a second drive), reads `user-data` and `meta-data`, and applies them.

This is the same mechanism EC2, GCE, Azure, OpenStack, and libvirt's
`virt-install --cloud-init` use. Cage builds the ISO itself via
`cloud-localds` or a `genisoimage` fallback (see `internal/cloudinit/`).

## What the ISO contains

cage generates two files per cage:

```
<cage-dir>/cloudinit/user-data    # YAML #cloud-config — the recipe
<cage-dir>/cloudinit/meta-data    # YAML — instance-id, hostname
```

…and packs them into `<cage-dir>/cloud-init.iso` (the disk attached to
the VM at boot). On Debian/Ubuntu/etc. these are NoCloud-format files.

## What `user-data` configures

In order, on every first boot:

1. **Create the `cage` user**
   - UID/GID auto-assigned, shell `/bin/sh`, member of `wheel` and `docker`
   - Passwordless sudo: `cage ALL=(ALL) NOPASSWD:ALL`
   - Embedded password hash so `cage console` login also works (password
     `cage` — only reachable via local serial console, not network)
   - SSH key authorized: the per-cage public key cage generated on the host

2. **Resize the rootfs** to fill the qcow2 overlay
   - `growpart: { mode: auto, devices: ['/'] }`
   - `resize_rootfs: true`
   - Lets you grow disk size in `.cage.yml` without rebuilding the image

3. **Disable password SSH auth**: `ssh_pwauth: false`

4. **Mount virtiofs filesystems** (when applicable):
   - `workspace` → `/workspace` (the host directory from `shares:` in the
     cagefile; root mode only)
   - `cage-runtime` → `/cage/runtime` (read-only; used for env injection,
     populated only when `env:` is set in the cagefile)

5. **Re-apply the SSH key** (runcmd, defensive)
   - `mkdir -p /home/cage/.ssh && chmod 700 …`
   - Writes `authorized_keys` again. This is critical for **custom
     images** (built via `cage build`): cloud-init's user module sees
     the cage user already exists and skips it, so the key wouldn't be
     refreshed without an explicit runcmd.

6. **Install sudo/doas** for Alpine; configure sudoers/doas for the cage
   user (Alpine doesn't ship sudo).

7. **Generate the `en_US.UTF-8` locale** so SSH-forwarded `LANG`/`LC_*`
   from the user's host don't cause `perl/dpkg/locale: Setting locale
   failed` warnings on every command. Specifically:
   - Debian/Ubuntu: `apt-get install -y locales`, `locale-gen
     en_US.UTF-8`, `update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`
   - Alpine: `apk add musl-locales musl-locales-lang`
   - All guarded by `which …` so they're no-ops on the wrong distro.

8. **Enable & start Docker** if the image has it (systemd-based:
   `systemctl enable docker && systemctl start docker`; OpenRC-based
   like Alpine: `rc-update add docker default && rc-service docker
   start`).

9. **Install SSH server** if cage requested it (`UseSSH`). For SLIRP /
   user-mode networking, port forwarding goes through QEMU; for bridge
   mode the bridge gives the VM a real LAN IP.

10. **Inject environment variables**, two flavors:
    - **Static** (legacy `Env` field): each `KEY=value` written via
      `runcmd` to `/etc/profile.d/cage-env.sh`.
    - **Runtime virtiofs** (`UseRuntimeEnv`, the modern path): cloud-init
      writes a `/etc/profile.d/cage-runtime-env.sh` that sources
      `/cage/runtime/env.sh` from the virtiofs mount on every login.
      Cage rewrites that file outside the VM whenever the `env:` block
      in the cagefile changes, so updates apply without rebooting.

11. **VM-side network isolation** (auto / SLIRP mode only): runcmd
    installs blackhole routes for the RFC 1918 private subnets cage was
    told to block, on top of the cloud-init network configuration. This
    is the second of cage's two isolation layers — the first is
    host-side passt-in-netns (root mode), this one is guest-side and
    applies even in user mode.

## What `meta-data` configures

```yaml
instance-id: cage-<timestamp>
local-hostname: <cage-name>
```

The instance-id is **regenerated every time** cage starts the VM. That's
how cloud-init knows to re-run on first-boot logic (cloud-init dedupes
by instance-id; same id ⇒ skip). For cage this matters because we want
the SSH key and env file to be re-injected when a cage is recreated
with a new key.

## Where it lives in the code

| File | Role |
|---|---|
| `internal/cloudinit/generate.go` | Builds `user-data` and `meta-data` strings from a `CloudInitConfig` |
| `internal/cloudinit/generate.go:GenerateISOWithConfig` | Writes the YAML to disk, packs into `.iso` via `cloud-localds` (preferred) or `genisoimage`/`mkisofs` (fallback) |
| `internal/cmd/start.go` | Calls `cloudinit.GenerateISOWithConfig` during cage creation; passes `MountVirtiofs`, `UseRuntimeEnv`, `NetworkIsolation`, `AllowedSubnets` |

## Debugging

Cloud-init runs in the VM and writes verbose logs:

```bash
cage ssh <name>
# inside the VM:
sudo cat /var/log/cloud-init.log          # cloud-init's own log
sudo cat /var/log/cloud-init-output.log   # stdout/stderr of every runcmd
sudo cloud-init status --long             # current state + errors
sudo cloud-init analyze show              # per-module timing
```

To force a re-run (e.g. after editing the ISO outside cage):

```bash
sudo cloud-init clean --logs && sudo cloud-init init
```

To inspect what cage generated for a cage (before the VM ever booted):

```bash
ls ~/.claude-cage/cages/<name>/cloudinit/
cat ~/.claude-cage/cages/<name>/cloudinit/user-data
# Or in root mode:
sudo cat /var/lib/libvirt/images/cage/cages/<name>/cloudinit/user-data
```

## Common failure modes

- **`cage ssh` times out for ~30 s on first boot**: cloud-init still
  installing the SSH server (or `apt-get install -y locales` warming up
  the cache). Normal — `cage start` waits up to 2 minutes.

- **`Setting locale failed` warnings**: should not appear after the
  locale generation step lands; if they do, the host is forwarding
  a locale that's neither `C.UTF-8` nor `en_US.UTF-8`. Generate it
  explicitly in your `Cagefile` via `RUN locale-gen <name>.UTF-8`.

- **`debconf: unable to initialize frontend: Dialog`**: harmless. dpkg
  is running non-interactively under cloud-init's runcmd, no tty. We
  silence noisy installs with `DEBIAN_FRONTEND=noninteractive`, but
  some packages still log this once.

- **Workspace not mounted in root mode**: the virtiofs fstab entry has
  `nofail` so the VM boots even if virtiofsd never came up on the host.
  Check `dmesg | grep virtio` in the VM and `cage doctor --root` on
  the host.

## See also

- [docs/cagefile.md](cagefile.md) — Cagefile syntax (the build-time
  equivalent of cloud-init)
- [docs/modes.md](modes.md) — user vs. root mode (affects whether
  virtiofs share + env injection are available)
- [docs/development/architecture.md](development/architecture.md) —
  big-picture component map
- [cloud-init upstream docs](https://cloudinit.readthedocs.io/) —
  full reference for `#cloud-config` directives
