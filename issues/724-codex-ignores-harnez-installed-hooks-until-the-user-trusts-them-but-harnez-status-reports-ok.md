# 724 — Codex ignores harnez-installed hooks until the user trusts them, but harnez status reports ok

**Status**: Open
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
