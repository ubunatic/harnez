# 1256 — issues close: the reason becomes a 200+ char commit subject, and --commit is easy to mistake for the reason

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX
**Related**: 232, 1174

---

## 1. Problem & Motivation
Reported 2026-11-21 from the webman session (harnez 0.5.10+5413). Two related problems with the commit that `harnez issues close` makes:

1. **Long reason, long subject.** `harnez issues close 055 "<reason>"` with a 190-char reason committed the subject `docs(issues): close 055, Kept one binary: reflector-only linux/386 build starts at ~49 MiB ulimit -v vs ~54 MiB ...` (about 230 chars). The close skill asks for a reason with the decisive evidence, so long reasons are the normal case.
2. **`--commit` taken for the reason.** On three tickets the agent wrote `harnez issues close -d . N --commit "<outcome text>"`. The commit subject became the bare outcome text (not a conventional commit), the Status line stayed a bare `Closed`, and no reason was recorded in the ticket. Nothing warned. History on a shared main can't be rewritten, so the result is permanent.

## 2. Technical Specification / Findings
- `issues --help`: `close [reason]` sets `Closed — <reason>`; `--commit "<msg>" replaces the default message`.
- The default message inlines the whole reason into the subject.

## 3. Implementation & Verification Plan
/goal Close commits keep a short conventional subject with the full reason in the body, and a likely misuse of `--commit` is caught before the commit. Stop and report if a change would break scripts that rely on the current subject format.

- Default message: `docs(issues): close <n>, <reason cut to fit ~72 chars>`, with the full reason in the commit body.
- When a status verb gets `--commit` but no positional reason, and the `--commit` value has no `type(scope):` prefix, refuse with a hint (`did you mean the reason? close <n> "<text>"`), or at least warn.
- Tests: a 200-char reason gives a subject of at most 72 chars plus the full body. `close 5 --commit "Kept one binary"` is refused or warned, and `--commit "docs(issues): ..."` still works.
