# 717 — harnez apply reports env changed and rewrites settings.json on every run

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: `scripts/smoke-test.sh` (idempotency check), [CLI design](../docs/CLIDesign.md)

---

## 1. Problem & Motivation
Running `harnez apply` twice in a row (2026-10-06) prints on every run:

```
  wrote /home/uwe/.claude/settings.json
    env: changed
2 changes.
```

But `~/.claude/settings.json` is byte-identical before and after the run (checked
with `cmp`). Apply rewrites the file and reports a change that did not happen.
That breaks the "second run is a no-op" expectation, hides real changes in the
output, and touches the file's mtime needlessly.

At the time, the `env` block held only `CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1`,
`DEBUG=false` and `HARNEZ_DISTILL_AUTOPIPE=false`. The "2 changes" count includes
one more item that the output does not name.

## 2. Findings / Uncertainties
- Not yet located: where the `env` comparison happens and why it sees a
  difference (e.g. string vs bool/number types, a value computed at apply time,
  key order, or comparing against a stale or default map).
- Unclear what the second of the "2 changes" is.

## 3. Implementation & Verification Plan
/goal Make a repeated `harnez apply` report 0 changes and leave `settings.json`
untouched (byte-identical, same mtime) when nothing differs, with a regression
test for the env comparison; or stop and report when blocked on a user decision
or denied permission.

Reproduce first: run `harnez apply` twice and capture the output and the file's
`cmp`/mtime. Check `scripts/smoke-test.sh` catches it after the fix.
