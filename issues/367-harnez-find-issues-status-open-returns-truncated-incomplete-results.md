# 367 — harnez find issues status:open returns truncated/incomplete results

**Status:** Closed — surface stderr truncation notice when capped and document -a in templates

**Severity:** Major — silently corrupts backlog reconciliation and roadmap planning

## What happened

In the `loom` project (`/home/uwe/projects/loom`, 66 tickets, 28 of them open),
running:

```
harnez find -d . issues status:open
```

returned only 10 results — issues 057–066, the most recently filed batch —
and silently omitted the other 18 open tickets (014–019, 034, 036–039, 042,
047, 048, 050–052, 056). Confirmed independently twice: once by an agent
running a full roadmap-reconciliation pass (who caught the discrepancy by
cross-checking against a `grep '**Status**' issues/*.md` sweep and got 28
matches instead of 10), and once by direct re-run in a second session.

`harnez find -d . issues next` and `harnez issues new` in the same project
worked correctly during the same session (ticket numbering was not affected),
so this looks scoped to the `status:open` search/filter path specifically,
not the tracker/index machinery in general.

## Why this is worse than a normal search bug

`status:open` is the documented, recommended way to "list active open
issues" per this project's own conventions (see e.g. loom's
`AGENTS.md`/`CLAUDE.md` "Harnez Managed Conventions" block). Two consecutive
roadmap-reconciliation passes in loom (`docs/Roadmap.md`, dated 2026-09-11 and
initially 2026-09-16) explicitly cite this exact command as their
backlog-survey source. If the truncation is long-standing, both of those
passes — and potentially any other project's roadmap/planning work that
trusted this command — may have silently reconciled against an incomplete
view of the backlog, without any error or warning surfaced to the operator or
the agent running it.

## Suspected cause (unconfirmed)

Not investigated in depth from the loom-side session. Worth checking whether
the recently-filed batch (057–066, filed in immediate succession via
`harnez issues new`) triggered some kind of pagination/limit/cache-freshness
issue in the search index specific to newly-created tickets, or whether
`status:open` filtering has a general result-count cap that silently
truncates rather than erroring.

## Reproduction

```
cd /home/uwe/projects/loom
harnez find -d . issues status:open   # returns 10 (057-066 only)
grep -l '\*\*Status\*\*: Open' issues/*.md | wc -l   # returns 28
```

## Suggested fix direction

- Fix the truncation/filter bug itself.
- Consider making any result-count cap (if that's the cause) visible in the
  command's own output (e.g. "showing N of M matches") rather than silent,
  so a wrong result doesn't look identical to a correct one.
