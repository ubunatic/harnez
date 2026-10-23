# 868 — agy resume reports the conversation's old quota error after a successful turn

**Status**: Open
**Priority**: P2 (Medium) — a working agent looks dead; the host pauses sprints and asks the user to switch models
**Severity**: Minor
**Category**: Bug
**Related**: 714 (sprint where it happened), 791 (`stale_session_errors`), `internal/subagent/agy.go:194-205`

---

/goal A resumed agy turn that did its work is not reported as failed because of an error left over from an
earlier turn, and resumed agents get a short note not to repeat stale provider errors; verified with a test on
a recorded agy result and one live resume, or stop and report when agy offers no way to tell old from new.

## Findings (2026-10-23)

- 2026-10-22 18:36: dev-714 (agy:gemini-3.8-flash) really hit `RESOURCE_EXHAUSTED (code 429): Individual quota
  reached ... Resets in 2h46m44s` (agy log `cli-20261022_175526.log`).
- 2026-10-23 11:58: `harnez agent resume dev-714` did real work (commits `9e247bf8`, `829a4af7`), its agy log
  `cli-20261023_115800.log` has no 429, yet agy printed `status: ERROR` with the *same* message and countdown.
  `parseAgy` (`internal/subagent/agy.go:199-204`) passes that on; the host reported "quota reached" while
  `agy -p /usage` showed 70% of the 5h Gemini pool left.
- So agy itself replays the conversation's last error on resume; the model did not repeat it.

## Way forward

1. Harness check (fixes this case): when a resumed agy turn reports an error identical to the error the
   session's previous turn recorded, and the turn shows progress (new response, tokens, or no matching error
   in this run's agy log), report success with a warning "stale provider error from an earlier turn ignored".
   Test with recorded agy JSON.
2. Resume note (cheap, for model-level repeats): there is already a per-turn protocol preamble (`roles.*.rules`
   in `spec/agent.yaml`). On resume of a session idle longer than a spec threshold, add one line: "This is a
   resumed session; do not report earlier errors (quota, 503) as current unless they recur in this turn."
   Keep it to that one line in spec; no new rule docs or AGENTS.md text.
3. Live check: resume an agy session whose last turn errored; expect success plus the warning.
