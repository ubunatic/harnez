# 595 — Codex start failure hides the real error (401 shown as 'Reading additional input from stdin')

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 594, 592

---

## 1. Problem & Motivation
`harnez agent start --model codex:terra:med` failed twice (rev-594c, rev-594d, 2026-09-26) with
`codex exec: exit status 1: Reading additional input from stdin...`. That is only Codex's startup banner.
The real cause was in the rollout file: `task_complete.error.message` = `unexpected status 401 Unauthorized`
(expired ChatGPT login; `codex login status` still said "Logged in"). The host could not see this and
suspected stdin handling instead.

## 2. Technical Specification / Findings
- Rollout: `~/.codex/sessions/2026/09/26/rollout-2026-09-26T00-44-31-01a0dabd-ddd4-7022-9512-b204f8d57764.jsonl`.
- Codex reports turn errors as `event_msg` `task_complete` with `error.message` (also likely as a JSON `error`/`turn.failed` event on stdout).

## 3. Implementation & Verification Plan
- On a failed Codex turn, surface the turn error message (JSON event or rollout `task_complete.error`) instead of the first stderr line.
- Recognize 401 and say: "Codex login rejected; run `codex login`".
- Test with a recorded failing event stream.

## Second case (2026-10-06)

`harnez agent start --role advisor --model luna:med` (codex:gpt-6-luna) failed after
confirmation with the same "Reading additional input from stdin..." text. The real error was only
in the Codex rollout JSONL (`~/.codex/sessions/.../rollout-*-<session>.jsonl`), on the turn's
completion event: `"error":{"message":"Selected model is at capacity. Please try a different
model.","codex_error_info":"server_overloaded"}`. That event is a likely source for the real
message; a transient overload like this one should also say that a retry may work.
