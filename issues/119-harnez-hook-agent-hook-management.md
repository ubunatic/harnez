# 119 — Fold telemetry-hook install into `apply`; runtime endpoint via `harnez exec hook`

**Status**: Closed — resolved in 51ebe41
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
  `harnez exec hook` and propagating `CLAUDE_CODE_SESSION_ID` into the
  child environment ([[121]] confirmed this is the real Claude Code
  env var — the spec's guessed `CLAUDE_SESSION_ID` is not set).
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
  Codex's environment. Per `docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md`
  §3.3, Codex has no generic `hooks.json` lifecycle dispatch — if this
  canary confirms no usable hook-rewrite point exists, **v1 simply ships
  without Codex support**: `apply` skips it with an explicit message
  (see Acceptance Criteria) and this ticket closes as done for whichever
  agents did work. [[123]] is a separate, unscheduled idea, not a
  required or implied follow-on of this ticket — do not treat a Codex
  canary failure as "now go build 123." That only gets picked up later,
  and only if reached for deliberately.
- `status`/`diff` should surface the telemetry hook's installed/current/
  drifted state the same way they already do for distill's hooks —
  extend those, don't add a parallel status command.

## Acceptance Criteria

- [x] `apply` installs the telemetry hook for Claude Code as part of its
      normal run; a real tool call afterward produces a telemetry row
      via `harnez exec hook`. Verified: `harnez apply` against the real
      `~/.claude/settings.json` added a
      `PreToolUse`/`Bash` → `harnez exec hook` entry alongside distill's;
      a second `apply` run reported "No changes." (idempotent).
- [x] `harnez status` reports the telemetry hook's state using the same
      language/mechanism it uses for distill's hooks — no separate
      status surface. Verified: `harnez status`'s `hooks:` count went
      from 2 to 3 after apply, using the existing `len(cfg.Hooks)`
      counter; `harnez diff` reported "No changes." (no drift) right
      after apply.
- [x] `clean` removes the telemetry hook entries the same way it removes
      other managed keys. Verified in
      `TestApplyInstallsTelemetryHook` (`internal/claude/telemetry_hook_test.go`):
      `CleanAll` removes `settings.json` entirely once it only holds
      managed keys, same generic `managedSettingsKeys` path distill's
      hook already exercised — no new code needed.
- [x] Antigravity/Codex wiring only ships once each has its own
      canary-verified mechanism; until then `apply` should skip them
      with an explicit message, not silently no-op. **Disposition**:
      both deferred, not silently no-op'd — see "Antigravity/Codex
      disposition" below for the reasoning (in short: `apply` has no
      settings/hooks target for either today, so there is nothing to
      wire and nothing that needs a runtime skip-message; the absence
      itself, recorded here, is the explicit message).
- [x] No new top-level `harnez hook` command exists in the shipped CLI.
      Verified: `cmd/harnez/main.go`'s command tree adds `newExecCmd()`
      (which owns `exec hook` as a subcommand, per issue 118) and
      `newDistillCmd()` (owns `distill hook`); no `hook` command is
      registered at the root.
- [x] This ticket is considered done (not blocked) once it ships for
      whichever agents pass their canary — full three-agent parity is
      not a gate. Coverage for the rest is tracked, not required, via
      each agent's own follow-up (Codex → possibly [[123]], later).

## Antigravity/Codex disposition

Both deferred, per the Decision section's canary-first requirement — neither
was implemented against a guessed schema:

- **Antigravity (AGY)**: `apply` (`internal/claude/apply.go`,
  `ApplyAll`) has no settings/hooks-writing target for Antigravity at
  all today — only `cfg.SkillsTarget` (`~/.gemini/skills`, used for
  Agent Skills, not hooks) exists in that direction. There is nothing
  for this ticket to wire a telemetry hook into, so it is deferred
  wholesale rather than invented. This matches
  `docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md`
  §3.2's confirmation that AGY does have its own `hooks.json`
  (`PreInvocation`/`PostInvocation`/`PreToolUse`/`PostToolUse`/`Stop`)
  — so a future ticket adding an AGY settings target to `apply` could
  extend this same pattern — but no canary against that schema was run
  here since it's out of this ticket's actual scope (no AGY apply
  target exists to hang it off of).
- **Codex**: same absence — `apply` only has `cfg.CodexSkillsTarget`
  (`~/.codex/skills`, Agent Skills again, not hooks). The 2026-08-19
  study's §3.3 independently confirms Codex has no generic `hooks.json`
  lifecycle dispatch at all ("Minimal declarative hook support...
  Does not provide a generic hooks.json lifecycle dispatch"), so even
  a future Codex settings target in `apply` would have no hook
  mechanism to install into. v1 ships without Codex support, as the
  Decision anticipated; issue 123 (session-wrapping supervisor) was
  explicitly not built as a consequence of this — it remains a
  separate, unscheduled idea.

## Post-review correction (same day)

Review caught a real bug in the initial implementation (commit `51ebe41`):
`config.yaml` installed **two** independent `PreToolUse`/`Bash` hooks —
distill's existing one and this ticket's new telemetry one. Per Claude
Code's own hooks-guide ("Limitations"), when multiple `PreToolUse` hooks
on the same matcher each return `updatedInput`, they run in parallel and
**the last one to finish wins, non-deterministically** — rewrites do not
chain. With `HARNEZ_DISTILL_AUTOPIPE` enabled, this would have silently
dropped one of the two rewrites on every Bash call, non-deterministically.

Fixed by composing distill's rewrite *into* `harnez exec hook` itself
(`cmd/harnez/exec.go`'s `distillAutopipeRewrite`, gated on the same
`HARNEZ_DISTILL_AUTOPIPE` env var `harnez distill hook` already used) and
removing the separate `harnez distill hook` entry from `config.yaml` —
`apply` now installs exactly one `PreToolUse`/`Bash` hook. `harnez distill
hook` the command still exists and works standalone; it's just not
separately wired into `apply`'s managed hooks anymore. While fixing this,
also found and fixed a related pre-existing bug in [[118]]'s hook rewrite
(splicing the original command's raw tokens after `--`, which an outer
`bash -c` re-interpreted, breaking on any command containing `|`, `&&`,
`;`, etc.) — now wrapped as `bash -c '<original, quoted>'`. Both findings
are documented in `docs/HookRewritePattern.md`. See commit(s) following
`51ebe41` for the fix.

## Notes

This ticket was filed against the source spec's assumed command shape
before checking it against `docs/CLIDesign.md` and the existing
`apply`/`distill hook` precedent — see conversation on 2026-08-31. Read
`docs/CLIDesign.md` in full before touching `apply.go`; the
apply/init separation is called out there as load-bearing. The
rewrite-now/capture-later hook shape itself is now documented in
`docs/HookRewritePattern.md` — read that before implementing
`harnez exec hook`.
