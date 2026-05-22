# Rebrand "Claude Cage" -> "Cage" - plan

Date: 2026-05-22

## Goal
Drop all "Claude" branding. Cage is now a generic QEMU/KVM-based sandbox CLI for
running VMs. All features are unchanged - this is naming/positioning only.

## Decisions (validated with the owner)
- Full rename including hard identifiers: Go module path and state directory.
- Module path: `github.com/s-oravec/claude-cage` -> `github.com/s-oravec/cage`.
- State dir: `~/.claude-cage` -> `~/.cage`. NO migration code (YAGNI): the owner
  moves it once (`mv ~/.claude-cage ~/.cage`) or wipes and re-pulls. The repo
  rename on github/gitea + `git remote set-url` is the owner's host-side action.
- `CLAUDE_HOME` is only a test fixture key (internal/runtime/env_test.go), not a
  real cage env var; renamed to a generic example kept alphabetically first.
- `CLAUDE.md` stays (Claude Code tooling/instruction file, not product branding).
- Claude Code-centric docs are rewritten to generic ("workload"/"any CLI" in the
  sandbox), keeping the features (yolo/unrestricted mode, etc.).
- Historical design/plan docs (docs/plans/*, docs/superpowers/*) keep their dated
  prose; only mechanical string replacements apply there.

## New positioning
- Tagline: "Cage - QEMU/KVM sandbox CLI for running VMs"
- Long: "Cage creates isolated QEMU/KVM virtual machines for running workloads in
  a secure sandbox with full Docker support."

## Tasks (each: change -> `go build ./...` + `go test ./internal/...` green -> commit)

### R1 - module path
`github.com/s-oravec/claude-cage` -> `github.com/s-oravec/cage` in go.mod and all
*.go imports (~189). Mechanical. Verify build + tests.

### R2 - state dir + test fixtures
- `internal/config/config.go` Dir(): the two `".claude-cage"` literals -> `".cage"`.
- All `~/.claude-cage` mentions in .go doc comments / help strings -> `~/.cage`.
- `internal/runtime/env_test.go`: `CLAUDE_HOME` -> `APP_HOME` (`/home/app`), kept
  alphabetically first so ordering asserts hold; `"it's Claude's cage"` fixture ->
  a generic apostrophe-bearing string with matching assertion.
Verify tests.

### R3 - CLI branding in code
- `internal/cmd/root.go:17-20`: Short/Long -> the new positioning above.
- Any remaining "Claude Cage"/"Claude Code" in .go strings/comments -> generic.
Verify build + tests.

### R4 - docs + configs
- README.md: rebrand + reposition as a generic QEMU/KVM sandbox CLI; rewrite
  Claude Code framing to generic. ASCII only.
- docs/*.md guides + docs/development/*: de-Claude prose, generic positioning.
- Makefile, .golangci.yml: module path / repo slug / binary refs.
- Repo URL slug `claude-cage(.git)` -> `cage(.git)` everywhere.
- Mechanical replacements also applied in historical docs/plans/* + docs/superpowers/*.

## Final
- Repo-wide sweep: `git grep -i claude` should only leave CLAUDE.md and any
  legitimate Claude Code tooling references (and git-trailer authorship, which is
  not in tracked files). Build + `go test ./internal/... -race` + `go vet` green.

## Out of scope (owner's host-side actions, documented for them)
- Rename the github + gitea repos `claude-cage` -> `cage` and
  `git remote set-url` accordingly.
- `mv ~/.claude-cage ~/.cage` (or wipe + re-pull).
