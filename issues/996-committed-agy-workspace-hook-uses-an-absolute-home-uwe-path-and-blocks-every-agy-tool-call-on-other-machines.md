# 996 — committed agy workspace hook uses an absolute /home/uwe path and blocks every agy tool call on other machines

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 373, 995

---

## 1. Problem & Motivation

`.agents/hooks.json` (added in f2d345a7 for the issue 373 tool-observation canary) is an agy
workspace hook with an absolute path:
`"command": "/home/uwe/projects/harnez/scripts/canary-agy-tool-hook.sh"`, matcher `*`, enabled.
agy loads it for every session whose workspace is a harnez checkout. On any machine or user
where that path does not exist or is not executable, every tool call fails before it runs:

```
JSON hook "jsonhook__harnez-tool-observer_PreToolUse_0_0" failed: command failed:
exit status 126, stderr: sh: line 1: /home/uwe/projects/harnez/scripts/canary-agy-tool-hook.sh: Permission denied
```

Seen 2026-10-28 (user alfiz, harnez 0.2.6+4019, session `argon-991b`): the first agy turn with
tool approval (995) could not read a single file and stopped with a blocked report. Reviewers and
advisors on agy are unusable in harnez checkouts until this is fixed. Forgewerk jobs that run agy
in a harnez worktree would hit the same wall.

## 2. Technical Specification / Findings

- The script exists in the repo (`scripts/canary-agy-tool-hook.sh`, mode 755); only the absolute
  path is wrong for other checkouts.
- Unknown: the working directory agy uses for workspace hook commands, and whether it expands
  variables. Check agy docs or one canary before choosing the path form.

## 3. Implementation & Verification Plan

1. Make the hook path checkout-independent: a repo-relative command if agy runs workspace hooks
   from the workspace root, or install the observer through `harnez apply` (issue 373) and remove
   the committed workspace hook. The hook must not fail closed on a missing script.
2. Add a lint or test that rejects absolute home paths in committed hook files (`.agents/`,
   `.claude/`, `.codex/`).
3. Canary: one `harnez agent --role advisor --model agy:argon:low` turn in a harnez checkout
   reads a file and replies.

## Decision (host, 2026-10-28, owner go for the fix)

373 is Closed (0314dfbd): `harnez apply` installs the observer globally in
`~/.gemini/config/hooks.json` (`harnez hook agy` PreToolUse, `harnez hook agy-post` PostToolUse,
matcher `*`). The committed `.agents/hooks.json` is a canary leftover that duplicates it with a
machine path. Fix: remove `.agents/hooks.json` from the repo (keep `scripts/canary-agy-tool-hook.sh`
only if a doc still uses it; update `docs/HookRewritePattern.md`), and add a test that fails on an
absolute home path (`/home/<user>/`, `/Users/<user>/`) in committed hook configs (`.agents/`,
`.claude/`, `.codex/`, `.gemini/`). Observation stays on through the global hook.
