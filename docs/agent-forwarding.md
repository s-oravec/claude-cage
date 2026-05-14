# SSH Agent Forwarding

Cage's `-A` / `--forward-agent` flag (on `cage ssh` and `cage build`)
plumbs your **host's** ssh-agent into the cage VM so that `git clone`,
`ssh`, and other agent-using tools inside the cage can authenticate to
private hosts (Gitea, GitHub, internal servers) — **without ever
copying your private keys into the cage image**.

## What it does

`-A` adds OpenSSH's `-A` flag to cage's SSH/SCP invocations. The OpenSSH
protocol then:

1. Creates a Unix socket on the cage VM (e.g. `/tmp/ssh-XXX/agent.NNN`)
2. Sets `SSH_AUTH_SOCK` in the SSH session's environment
3. Tunnels every read/write on that socket back to your host's
   ssh-agent

So inside the cage, `ssh`/`git`/`scp` can ask "agent, sign this
challenge with the key matching this fingerprint" and the request
travels back to your host, where the real agent — holding the real key
in RAM — does the signing and returns the signature.

The private key bytes never leave your host. The protocol exposes only
*list public keys* and *sign challenge* operations; there is
deliberately no "give me the private key" operation. See
[Security model](#security-model) below for the full threat analysis.

## Sudo and auto-discovery

`cage` for root mode runs under `sudo`. The default `sudoers`
configuration has `Defaults env_reset`, which strips most environment
variables — including `SSH_AUTH_SOCK` — before invoking the command.
So `sudo cage build -A` would have nothing to forward, and the SSH
session would fall back to no auth.

Cage works around this automatically. When `-A` is set, the SSH
subprocess code in `internal/ssh/connect.go` calls
`internal/sshagent.Discover()`, which:

1. **Honours an explicit `SSH_AUTH_SOCK`** if present in the
   environment (e.g. the user passed `sudo --preserve-env=SSH_AUTH_SOCK`
   or has `env_keep += "SSH_AUTH_SOCK"` in sudoers — both still work).
2. **Otherwise, if running as root with `$SUDO_USER` set**, looks up
   that user's UID and probes the standard runtime-dir paths:
   - `/run/user/<uid>/keyring/ssh` — gnome-keyring (default on
     Ubuntu/Debian/Fedora with GNOME)
   - `/run/user/<uid>/ssh-agent.socket` — systemd user unit
     (`ssh-agent.service` shipped by some openssh packages)
   - `/run/user/<uid>/openssh_agent` — other openssh layouts
3. **Rejects non-socket files** (defense-in-depth: even if an attacker
   somehow planted a regular file at one of those paths, cage would
   not silently use it).
4. **Returns the first match**, which cage injects as `SSH_AUTH_SOCK`
   into the spawned `ssh` subprocess via `cmd.Env`.

Net result: `sudo cage build -A` *just works* on a vanilla Linux
desktop, without any `/etc/sudoers.d/*` tweak. Power users who need a
non-standard agent socket can still set `SSH_AUTH_SOCK` explicitly via
`sudo --preserve-env=SSH_AUTH_SOCK` and cage honours it.

## Lifetime

The forwarded socket exists **only while a given SSH session is open**.

- **`cage ssh -A`**: socket lives for the duration of the interactive
  shell. When you `exit`, `sshd` in the cage tears the socket down.
  A subsequent plain `cage ssh` has no agent access.
- **`cage build -A`**: every `RUN` step opens a new SSH session.
  The agent is reachable for that step only — usually seconds to a few
  minutes. Once the step's command returns, the session closes and the
  socket is destroyed. The next `RUN` opens a fresh session with its
  own short-lived socket.

## Security model

This is the place to read carefully before using `-A` against an
untrusted target. For a cage you control (you're the only operator, no
foreign code) it's a non-issue; the analysis below is to make the trust
boundary explicit.

### What an attacker *inside the cage* can NOT do

- **Exfiltrate the private key bytes.** ssh-agent's wire protocol has
  no `RETRIEVE_KEY` operation. It can list public keys and sign
  challenges; it cannot disclose private key material. This is a design
  choice from the 1990s precisely because agent forwarding was meant to
  be usable against partially-trusted hops.

### What an attacker *inside the cage* CAN do (while a session is open)

- **Impersonate you to any other host that trusts your key.** They can
  open new SSH connections through the forwarded socket: `ssh
  user@other-server`, `git push git@github.com:you/repo`, etc. The
  agent will gladly sign the auth challenge.
- **Manipulate the agent state** via `ssh-add -D` / `ssh-add -d`.
  Annoying — your live agent loses identities — but recoverable:
  `ssh-add ~/.ssh/id_ed25519` (re-prompting for passphrase if
  applicable) restores it. No disk files are touched. **Note**:
  manipulating the agent through a forwarded socket is *protocol-level*
  allowed; not all ssh-agent implementations expose `REMOVE_IDENTITY`
  through forwards by default (`ssh-add -c` and lock features can
  block it).

### Mitigations

- **Don't pass `-A` when you don't need it.** Cage's default is no
  forward.
- **`ssh-add -c <keyfile>`** — agent prompts you (GUI confirmation
  dialog from gnome-keyring/seahorse, or terminal in plain ssh-agent)
  for every signing request. Annoying for many-`git`-per-day flows,
  safe-as-can-be for paranoid contexts.
- **`ssh-add -t <seconds>`** — bounded-lifetime keys that auto-expire
  in the agent. Good for "I'll use the cage for an hour."
- **Per-purpose key** — generate a dedicated keypair, register the
  public half with the upstream service (e.g. Gitea), add the private
  half to the agent only when you want cage to have access. This
  isolates blast radius from your "main" SSH key.

For typical dev cages where you are both host user and cage operator,
no mitigation beyond "don't pass `-A` when not needed" is necessary —
the agent risk surface is the same as `ssh -A user@your-vm`, which is
a workflow most developers already use unconsciously.

## Where it lives in the code

| File | Role |
|---|---|
| `internal/sshagent/discover.go` | `Discover()` — picks `SSH_AUTH_SOCK` or probes `$SUDO_USER`'s runtime dir |
| `internal/ssh/connect.go` (`applyAgentEnv`) | Calls `Discover()` and injects `SSH_AUTH_SOCK` into the `ssh` subprocess when `-A` is set |
| `internal/cmd/ssh.go` (`-A` flag) | Surfaces `-A` on `cage ssh`, passes through to `SSHOptions{ForwardAgent: true}` |
| `internal/cmd/build.go` (`-A` flag) | Same on `cage build`, propagates to `BuildConfig.ForwardAgent` |
| `internal/build/executor.go` (`executeRun`) | Calls `ssh.ExecCaptureWithOpts(..., ForwardAgent: e.config.ForwardAgent)` |

## Troubleshooting

- **`Permission denied (publickey)` on `git clone`**
  Most likely SSH_AUTH_SOCK was empty when cage spawned ssh.
  - Verify on the host: `ssh-add -l` lists your keys.
  - Verify it survives sudo: `sudo bash -c 'echo $SSH_AUTH_SOCK'` —
    if empty, auto-discovery should kick in; if it still fails, cage
    couldn't find the socket. Check `/run/user/$(id -u)/` for what
    socket path your desktop actually uses.
  - Workaround: `sudo --preserve-env=SSH_AUTH_SOCK cage build -A`.

- **`Host key verification failed`** — *unrelated* to agent forwarding.
  The cage VM has no prior knowledge of the upstream host's SSH host
  key. Run `ssh-keyscan -H <host> >> ~/.ssh/known_hosts` once inside
  the cage (or in a `RUN` step in the Cagefile) before the `git
  clone`.

- **`Could not open a connection to your authentication agent`** —
  cage didn't find an agent socket and didn't see one in env. Either
  the desktop's agent isn't running, or it's at a non-standard path.
  Set `SSH_AUTH_SOCK` explicitly and pass it through with
  `sudo --preserve-env=SSH_AUTH_SOCK`.
