# 119 — `harnez hook`: agent hook install/uninstall/status (Claude, Antigravity, Codex)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[118-harnez-exec-shell-interceptor]], [[122-agent-instruction-tool-feedback-protocol]], `harnez distill` hook installer (existing)

## Problem

The spec wants shell-tool interception ([[118]]) wired automatically
into each supported agent's hook system, reusing the existing hook
injection pattern already shipped for `harnez distill`, rather than
requiring the user to manually prefix every command with `harnez exec`.

## Scope

Command signature:

```
harnez hook install [--all | --agent <claude|antigravity|codex>]
harnez hook uninstall
harnez hook status
```

- Reuse the `harnez distill` hook installer's existing mechanics (file
  locations, idempotency/drift-repair pattern from
  `scripts/smoke-test.sh`) rather than building a second installer —
  this is explicitly called out in the spec as "reuses ... existing
  hook installer framework."
- **Claude Code**: configure pre/post-tool-use hooks (per
  `~/.claude/settings.json` hooks schema) that wrap shell tool calls
  with `harnez exec` and propagate `CLAUDE_SESSION_ID` into the child
  environment.
- **Google Antigravity**: inject hook definitions into `.antigravity/`
  workspace config. **Canary first** — this repo has no prior
  integration with Antigravity's hook format; probe its actual
  config schema before writing an installer against assumed shape
  (per `docs/other/Canary.md`).
- **OpenAI Codex**: inject environment shims/shell wrappers into Codex's
  execution context. Same canary caveat — verify Codex's actual hook/
  wrapper mechanism first; the existing `internal/usage` package already
  has a `CollectCodex` integration (see issue 111) that may document
  what's known about Codex's environment already.
- `harnez hook status` reports per-agent: not installed / installed-and-
  current / installed-but-drifted (matching the drift-detection language
  used for `harnez distill`'s own status output).

## Acceptance Criteria

- [ ] `install --agent claude` round-trips: install, verify hook fires
      on a real tool call (i.e. `harnez rate`/`exec` telemetry row
      appears), uninstall, verify it's gone.
- [ ] `install --all` installs for every agent whose canary passed;
      agents without a verified mechanism are skipped with an explicit
      message, not silently no-op'd.
- [ ] `status` correctly distinguishes not-installed / current / drifted
      for at least the Claude path (others as their canaries land).
- [ ] Reuses, not duplicates, the `harnez distill` installer's file-
      write/idempotency logic — call out in the PR/commit if a shared
      helper was extracted.

## Notes

Antigravity and Codex hook support should each get their own canary
step (and may need their own follow-up ticket if the mechanism turns
out to be nontrivial) before this ticket claims support for them —
don't let "seamlessly" in the spec's acceptance criteria paper over an
unverified integration.
