# 209 — Move `agy-hooks` Management into `harnez apply`

**Status**: Closed — folded agy-hooks management into harnez apply and status, hid agy-hooks CLI command
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor / Architecture
**Related**: [193-research-agy-hook-surface-for-transparent-exec-distill.md](193-research-agy-hook-surface-for-transparent-exec-distill.md), [196-agy-native-hooks-plan-alongside-claude-hooks.md](196-agy-native-hooks-plan-alongside-claude-hooks.md), [200-codex-native-hooks-preTooluse-wiring.md](200-codex-native-hooks-preTooluse-wiring.md), [119-harnez-hook-agent-hook-management.md](119-harnez-hook-agent-hook-management.md)

---

## 1. Problem & Motivation

`harnez` currently exposes `harnez agy-hooks` as a top-level CLI command to manage Antigravity's `~/.gemini/config/hooks.json` (`apply` and `status`).

However:
1. **Inconsistent CLI Surface**: Claude Code hooks are managed automatically via `harnez apply` (and checked via `harnez status`). Codex hook management was also folded into `harnez apply` (issue 200). Having `agy-hooks` sit as an unmanaged, separate top-level command is an ergonomics anomaly that clutters `harnez --help`.
2. **Setup Friction**: Users configuring an environment with `harnez apply` expect all supported harness integrations (Claude, Codex, AGY) to be applied or managed uniformly, without needing to discover and invoke a separate top-level `harnez agy-hooks apply` command.

## 2. Proposed Solution

1. **Fold AGY Hook Application into `harnez apply`**:
   - Make `harnez apply` configure `~/.gemini/config/hooks.json` by default alongside `~/.claude/settings.json` and `~/.codex/config.toml` (or guarded by an agent target flag, e.g. `--target` / `--agent=all|claude|agy|codex`).
2. **Move Status & Drift Detection into `harnez status`**:
   - `harnez status` should report whether AGY hooks are installed, up-to-date, or drifted, just as it does for Claude and Codex.
3. **Demote or Retain Internal Runtime Hook**:
   - Keep the hook execution endpoint itself (e.g. `harnez agy-hooks hook` or rename to `harnez hook agy` / `harnez exec hook --agent=agy`) as a non-top-level plumbing command called by `hooks.json`.
   - Deprecate or remove `agy-hooks` as a top-level user-facing management command.

## 3. Acceptance Criteria

1. Running `harnez apply` applies the `hooks.json` configuration for Antigravity when `~/.gemini` exists.
2. `harnez status` reports AGY hook status alongside Claude and Codex.
3. `harnez --help` no longer lists `agy-hooks` as a top-level user command (it is either removed or hidden).
4. `go test ./...` passes cleanly across `cmd/harnez` and `internal/agy`.

---

## Implementation Plan

### Current state (verified)

- `internal/agy/hooks.go` already exposes exactly the API this needs:
  `HooksPath(home)`, `BuildHooksDoc()`, `Apply(path)`, `Status(path)`,
  `Remove(path)` — the same shape as `internal/codex/hooks.go`.
- Codex's precedent is in `internal/claude/apply.go` ~line 743: a
  `cfg.CodexHooksTarget != ""` block calling `codex.Apply(fsutil.ExpandHome(...))`,
  driven by `codex_hooks_target: ~/.codex/config.toml` in `config.yaml` and
  `Config.CodexHooksTarget` in `internal/claude/config.go:31`.
- `cmd/harnez/agyhooks.go` registers `agy-hooks` with three subcommands
  (`apply`, `status`, `hook`), registered top-level in `cmd/harnez/main.go:602`.
- Gap worth noting: `internal/claude/status.go` does **not** report Codex hook
  status either — `codex.Status` and `codex.Remove` have zero non-test callers.
  So acceptance criterion 2 means adding *both*, not just AGY.

### Steps

1. **Config** — add `AgyHooksTarget string \`yaml:"agy_hooks_target"\`` to
   `internal/claude/config.go` next to `CodexHooksTarget`, and
   `agy_hooks_target: ~/.gemini/config/hooks.json` to `config.yaml` next to the
   codex entries. This keeps target paths declarative and makes the feature
   disableable by blanking the key, exactly like Codex's.
2. **Apply** — in `internal/claude/apply.go`, directly after the existing Codex
   hooks block, add the mirrored AGY block: expand the target, call `agy.Apply`,
   count the change, `addStat("agy hooks", "up to date")` when unchanged.
   Guard per acceptance criterion 1: only apply when the agent is actually
   installed, i.e. `~/.gemini` exists (`os.Stat(filepath.Dir(filepath.Dir(path)))`)
   — otherwise `apply` would create a `~/.gemini` tree on machines with no
   Antigravity install. Note `agy.Apply` itself `MkdirAll`s, so the guard must
   live in the caller.
3. **Status** — extend `internal/claude/status.go`'s `checks` slice with entries
   for both hook integrations, using their `Status(path)` results rather than a
   bare file-exists check so drift is visible:
   - `agy hooks.json [harnez]` → `installed && !drifted`
   - `config.toml [hooks.harnez]` → same via `codex.Status`
   The existing `check func() bool` shape only renders `ok`/`missing`; either
   accept that (drifted reads as `missing`, which is actionable) or widen the
   entry to return a string state. Recommend widening — "drifted" vs "missing"
   is the distinction `agy-hooks status` currently gives users and the one this
   change would otherwise regress.
4. **Demote the management command** — in `cmd/harnez/agyhooks.go`, drop the
   `apply` and `status` subcommands and set `Hidden: true` on the `agy-hooks`
   parent, leaving only `agy-hooks hook` (the PreToolUse handshake). Do **not**
   rename the command: `internal/agy/BuildHooksDoc` writes the literal
   `"harnez agy-hooks hook"` string into every already-installed `hooks.json`, so
   a rename orphans existing installs until the next `apply` runs. Update the
   file header comment to point at `apply` the way `cmd/harnez/codexhooks.go`'s
   header already does.
5. **Tests** — `internal/claude/apply_test.go`: applying with `AgyHooksTarget` set
   writes the hook entry, is idempotent on a second run, and is skipped when the
   `~/.gemini` parent is absent. `internal/claude/status_test.go`: installed /
   missing / drifted states render distinctly. `cmd/harnez/agyhooks_test.go`:
   delete the apply/status subcommand tests, keep the `hook` handshake tests, and
   assert `agy-hooks` is hidden from root help.
6. `go test ./...`, `make check`, `make install`, then `bash scripts/smoke-test.sh`
   — the smoke test exercises apply/drift/repair and is the right place to catch a
   regression in the new apply block.

### Design decisions / tradeoffs

- **Config-driven target, not a `--target`/`--agent` flag.** The ticket floats
  `--agent=all|claude|agy|codex`. Reject it: `docs/CLIDesign.md` makes `apply`'s
  flag surface (`-c`, `-t`, `-d`) load-bearing, Codex set the precedent of a config
  key rather than a flag, and an agent selector is a new axis nothing yet needs.
- **Keep `agy-hooks hook` hidden rather than removed**, and keep the command string
  stable — removing or renaming it breaks live `hooks.json` files on disk.
- **`~/.gemini` existence guard** is the only behavioural difference from Codex's
  block; call it out in the code comment so a later reader does not "unify" it away.

### Risks / open questions

- Codex's block has no install guard, so `apply` may already be creating
  `~/.codex/config.toml` on machines without Codex. Adding the guard to AGY invites
  the question of whether Codex should get one too — worth a follow-up ticket
  rather than silently changing Codex behaviour here.
- `harnez clean` has no `agy.Remove`/`codex.Remove` call today. Folding apply in
  without folding clean in leaves an asymmetry: `apply` installs it, nothing
  uninstalls it. Either add both `Remove` calls to clean in this ticket or file it
  as a follow-up; recommend adding them here since the code already exists.
- Widening `status.go`'s `check func() bool` to a tri-state touches every existing
  check entry. Keep that edit mechanical.

### Scope

**Medium** — small, well-precedented diffs across 5 files, but it spans
config + apply + status + clean + CLI surface, and the status tri-state widening
touches shared code.
