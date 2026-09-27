# 610 — Crashed agent start leaves a session that cannot be resumed by name or ID

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 306 (Codex quarantine), 606 (status after interrupted turn)

---

## 1. Problem & Motivation
Peer report from neus (2026-09-27). A foreground `harnez agent start --name neus-029 --model luna:med --stream stats
-f prompt.md` crashed mid-turn: `agent start luna:med failed: codex exec: exit status 1: <raw Go source fragment>`.
It had printed `reconnect: harnez agent resume 01a0e2ce-62cd-7a20-9edf-c8691ba60d93`. Afterwards:
- `harnez agent resume --name neus-029 ...` → `session neus-029 not found`
- `harnez agent resume 01a0e2ce-... "<prompt>"` → `multiple resumable agents in .; pass --name`
  (positional ID ignored; neus-029 not among the candidates).
The worker's progress is lost; the host must start over in a fresh session.

## 2. Technical Specification / Findings
- Same day, the host's `dev-560-m4` resume failed with `codex resume: exit status 1` and the next `agent start` was
  refused with "provider codex quota is exhausted". Likely trigger for both: Codex quota ran out mid-turn.
  306's `isCodexUsageLimitError` did not classify it (the error text was raw tool output, not "usage limit").
- Suspected: the session record (with name) is only persisted on a successful start, and the positional ID is not
  used as a selector when several resumable sessions exist.
- The error message shows raw tool output (Go source) instead of a short cause.

## 3. Implementation & Verification Plan
- Persist the session record, including `--name`, as soon as the provider session ID is known, before the turn ends.
- `agent resume <id>` selects by that ID (full or unique prefix) before implicit selection.
- On a failed turn, mark the session failed/interrupted (see 606), keep it resumable, and print a short cause
  (e.g. quota exhausted) instead of the raw last output; check `codex exec` stderr for the usage-limit signal.
- Tests with a fake driver that exits 1 mid-turn: resume by name and by ID both work; status is not "completed".
