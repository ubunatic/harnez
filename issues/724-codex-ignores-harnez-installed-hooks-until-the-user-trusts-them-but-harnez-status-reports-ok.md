# 724 — Codex ignores harnez-installed hooks until the user trusts them, but harnez status reports ok

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [#723 agent container](723-consolidated-agent-container-with-in-container-lmcoder-smoke-test.md), [#200 Codex hooks wiring](200-codex-native-hooks-preTooluse-wiring.md)

---

## 1. Problem & Motivation
Found by the #723 smoke test. Codex 0.160 runs config hooks only when each one
has a matching `trusted_hash` under `[hooks.state]` in `~/.codex/config.toml`.
On a fresh machine `harnez apply` writes the hooks, `harnez status` reports
`[hooks.harnez] ok`, but Codex never runs them (no `codex-telemetry` or
`codex-hook` calls in `harnez log`) until the user approves them in Codex.
Existing installs work only because their hooks were trusted earlier.

/goal `harnez status` (and `apply`'s summary) tells the user when installed
Codex hooks are not trusted and how to trust them. Stop and report before
having harnez write `trusted_hash` entries itself: that bypasses Codex's
security prompt and needs a user decision.

## 2. Technical Specification / Findings
- Trust entries look like
  `[hooks.state."/home/u/.codex/config.toml:session_start:0:0"]` with
  `trusted_hash = "sha256:..."`, one per hook event.
- A changed hook command presumably changes the hash and needs re-trust;
  confirm how Codex computes it before comparing hashes.
- Automation can skip trust per call with `codex exec
  --dangerously-bypass-hook-trust` (used by the #723 smoke test).

## 3. Implementation & Verification Plan
- Status check: hook installed but untrusted → warning with the fix.
- Test with a config without `[hooks.state]`; live check in the #723 container.

## Audit 2026-11-08

Jules audit (GitHub PR 139, report `reports/jules/724.md`, checked against d8c61da; all 4 references hold): still valid. `internal/codex/hooks.go:202` `Status` reports only installed/drifted, and `internal/claude/status.go:166-172` prints `ok` with no trust check; no Go code reads `trusted_hash` or `[hooks.state]`. Smallest step: have `codex.Status` also report untrusted harnez hooks from `[hooks.state]` and print a warning with the trust hint in `RunStatus`. Needs a real Codex `config.toml` sample with `[hooks.state]` first (canary).
