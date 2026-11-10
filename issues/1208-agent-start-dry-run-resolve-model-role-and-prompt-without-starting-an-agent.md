# 1208 — agent start --dry-run: resolve model, role and prompt without starting an agent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 545 (backlog bundle that lists `agent start --dry-run` among lean-sprint guardrails), 1203, 1019

---

## 1. Problem & Motivation

Trial 2026-11-10: `harnez agent start --dry-run -p x` fails with `Error: unknown flag: --dry-run`.
While checking 1203 (official model names), the only way to see how `--model` resolves was a live start
with a name that fails before launch; a valid name (e.g. `gemini-4-argon:low`, scarce quota) could not
be checked without spending a real turn.

## 2. Goal

/goal `harnez agent start --dry-run` (and `agent resume --dry-run`) prints what a real start would do:
resolved `provider:model:tier`, role, session name, working dir, the provider argv, and the prompt
source, then exits 0 without launching a provider, creating a session record or touching quota; errors
are the same as a real start. Covered by tests that fail if a provider process is started. Stop and ask
the owner when output format choices need their judgment. On a loop or no progress, clear the goal and
write a failure report here.

## 3. Acceptance

- `--dry-run` with a valid model prints the resolution; with an invalid one, the same error as today.
- No provider process, no session file, no telemetry row (test with a fake provider on PATH that fails if run).
- Help and `docs/` mention the flag; the lean-sprint skill can use it to check a model before dispatch.
- 545 gets a note that its dry-run item is done here.

## 4. Notes / Unverified

- Whether resume can dry-run without reading a live session is unverified.
- Check the live start path and recent commits (971 changed stream output) before starting.
