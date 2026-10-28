# 997 — harnez apply should give agy an allowlist of common read tools so interactive sessions stop asking for ls, cat and harnez

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Feature
**Related**: 995, 996, 373

---

## 1. Problem & Motivation

An interactive `harnez-agy` session (owner, 2026-10-28, harnez 0.2.6+4022) asks for approval of
`ls`, `cat`, `harnez` and other everyday tools. `harnez apply` configures Claude's permission
allowlist (46 allow rules) but writes nothing permission-related for agy: it only sets
`statusLine` in `~/.gemini/antigravity-cli/settings.json` and hooks in
`~/.gemini/config/hooks.json`. agy should get the same common-tool allowlist.

995 solved the headless side for advisor/reviewer with `--dangerously-skip-permissions --sandbox`;
interactive sessions need a real allowlist, not skip-all.

## 2. Technical Specification / Findings

- `agy --help` has no allowlist flag (only `--dangerously-skip-permissions`, `--sandbox`).
- The agy binary (`~/.local/bin/agy`, `strings`) knows: JSON keys `agentPermissions`, `allowlist`,
  `allow`, `deny`, `autoExecutionPolicy`; enums `AUTO_EXECUTION_{OFF,AUTO,EAGER,PROCEED_IN_SANDBOX}`,
  `AUTO_EXECUTION_POLICY_{REQUIRE_REVIEW,PROCEED_IN_SANDBOX,NOT_ENFORCED}`,
  `PERMISSION_PRESET_{NONE,DEFAULT,AUTO,REQUEST_REVIEW}`, `PermissionGrants`. The settings.json
  shape for them is not documented.
- Current `settings.json` keys: `colorScheme`, `model`, `statusLine`, `trustedWorkspaces`.

## 3. Implementation & Verification Plan

1. Canary first (host): find the settings shape agy honours for a command allowlist and an
   auto-execution policy (agy docs, its `/permissions`-style UI writing settings.json, or a diff
   of settings.json after approving "always allow" once). Record the exact shape here.
2. Spec: the common-tool list lives in `spec/` (reuse the source of Claude's 46 allow rules where
   the semantics match: read tools, `ls`, `cat`, `rg`, `git status/diff/log/show`, `harnez`
   read commands, `make test*`), with its schema; no Go list.
3. `harnez apply` merges the agy allowlist into `~/.gemini/antigravity-cli/settings.json`
   idempotently, keeping user keys; drift shown by status like the agy hooks.
4. Tests: merge into empty / existing settings, idempotency, user keys kept.
5. Canary: a fresh `harnez-agy` session runs `ls`, `cat`, `harnez issues list` without a prompt
   and still asks before a write outside the allowlist.
