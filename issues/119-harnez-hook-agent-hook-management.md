# 119 — Fold telemetry-hook install into `apply`; runtime endpoint via `harnez exec hook`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[118-harnez-exec-shell-interceptor]], [[122-agent-instruction-tool-feedback-protocol]], `internal/claude/apply.go` (existing `"hooks"` managed key), `cmd/harnez/distill.go` (`newDistillHookCmd`, existing precedent)

## Decision (2026-08-31)

The source spec proposed a new top-level `harnez hook install/uninstall/
status` command family. That does not match this repo's actual
precedent, which this ticket originally missed:

- Hook **installation** already happens inside `apply`'s existing
  managed-keys merge (`internal/claude/apply.go`, the `"hooks"` key) —
  `apply` writes the hooks block into `~/.claude/settings.json` (and the
  other agent configs it targets) as part of its normal global-sync
  flow. There is no separate `harnez <feature> hook install` installer
  command anywhere in the codebase today.
- The **runtime endpoint** an agent's hook JSON actually invokes follows
  the `harnez distill hook` pattern (`cmd/harnez/distill.go`,
  `newDistillHookCmd` / `runDistillHook`): a subcommand *of the feature
  command itself* that reads the hook payload from stdin and reacts,
  not a separate top-level noun.
- Install-state visibility (installed / current / drifted) is already
  `status`'s and `diff`'s job across the whole config surface — a
  feature-specific `status` subcommand would duplicate that rather than
  extend it.

**Decided**: no new `harnez hook` command. Instead:

1. Telemetry-hook wiring (pre/post-tool-use entries that wrap shell
   calls with `harnez exec`, per-agent) is added to `apply`'s existing
   managed-hooks merge, alongside distill's — same code path, one more
   managed hook entry per supported agent.
2. The runtime endpoint agents' hook configs point at is
   `harnez exec hook` (mirroring `newDistillHookCmd`), reading the tool
   invocation off stdin and writing the [[116]]-backed telemetry row,
   not a bespoke installer subtree.
3. `apply --agent <claude|antigravity|codex>` (or reuse of whatever
   per-agent selection `apply` already supports, if any — check before
   adding a new flag) gates which agents get the telemetry hook, rather
   than a `--all`/`--agent` pair on a standalone command.
4. Uninstall is `apply`'s existing `clean` counterpart, not a new
   `harnez hook uninstall`.

## Scope

- Extend `internal/claude/apply.go`'s hooks-merge logic to add the
  telemetry pre/post-tool-use hook entries (Claude Code first — see
  canary caveats below for Antigravity/Codex), pointing at
  `harnez exec hook` and propagating `CLAUDE_SESSION_ID` into the child
  environment.
- Add `harnez exec hook` per the `harnez distill hook` precedent:
  stdin-driven, writes one telemetry row via [[116]], exits fast.
- **Google Antigravity**: hook wiring goes into whatever `apply`
  currently writes for Antigravity (if anything — confirm current
  scope) or is deferred to a follow-up if `apply` has no Antigravity
  target yet. **Canary first** — no prior integration with Antigravity's
  hook format exists in this repo; probe its actual config schema
  before writing against assumed shape (per `docs/other/Canary.md`).
- **OpenAI Codex**: same canary caveat. `internal/usage`'s `CollectCodex`
  integration (issue 111) may document what's already known about
  Codex's environment.
- `status`/`diff` should surface the telemetry hook's installed/current/
  drifted state the same way they already do for distill's hooks —
  extend those, don't add a parallel status command.

## Acceptance Criteria

- [ ] `apply` installs the telemetry hook for Claude Code as part of its
      normal run; a real tool call afterward produces a telemetry row
      via `harnez exec hook`.
- [ ] `harnez status` reports the telemetry hook's state using the same
      language/mechanism it uses for distill's hooks — no separate
      status surface.
- [ ] `clean` removes the telemetry hook entries the same way it removes
      other managed keys.
- [ ] Antigravity/Codex wiring only ships once each has its own
      canary-verified mechanism; until then `apply` should skip them
      with an explicit message, not silently no-op.
- [ ] No new top-level `harnez hook` command exists in the shipped CLI.

## Notes

This ticket was filed against the source spec's assumed command shape
before checking it against `docs/CLIDesign.md` and the existing
`apply`/`distill hook` precedent — see conversation on 2026-08-31. Read
`docs/CLIDesign.md` in full before touching `apply.go`; the
apply/init separation is called out there as load-bearing. The
rewrite-now/capture-later hook shape itself is now documented in
`docs/HookRewritePattern.md` — read that before implementing
`harnez exec hook`.
