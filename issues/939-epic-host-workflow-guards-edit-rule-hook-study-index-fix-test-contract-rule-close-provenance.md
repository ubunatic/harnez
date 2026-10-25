# 939 — Epic: host workflow guards (edit-rule hook, study index fix, test contract rule, close provenance)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Epic
**Related**: 937, 916, 936, 915, story `docs/studies/2026-10-25-four-days-of-epics-server-monitor-and-skills.md`

---

/goal Every task below is closed, skipped with a reason, or parked in its own ticket; each code task has tests,
`make install` and a live check; the epic ends with a sprint study, `/evergreen` and `/wrap`. Stop and report
when an owner decision blocks a task. Circuit breaker: when a worker loops, a blocker is hard to solve, or the
host sees no progress, clear the goal and write a failure report here (what was tried, why it failed, resume
point).

## Why these together

All four are small guards against slips seen in host sessions 2026-10-22 to 2026-10-25: shell edits despite the
Edit/Write rule (937), a failed first commit on every new study (916, hit again in sprint 938), tests that copy
the expected value from the code (936), and closes that lose which model did the work (915).

## Out of scope

Blocking shell edits (owner chose warn, 937 Decision); a full provenance view in the usage UI.

## Open decisions

- 916: lint message only vs. a verb or hook that stages `docs/README.md` (ticket asks to decide).
- 915: where the fields live and whether the index shows them.
Host decides both after the code fanout and records them in each ticket's `## Decision`.

## Prior-art web fanout

Skipped: internal harness guards and doc rules; no external tool to reuse.

## Tasks

| Ticket | What | Group | Searcher → architect → developer | Reviewer | Status |
|---|---|---|---|---|---|
| 937 | PreToolUse hook warns on shell edits, asks for feedback, escalates to token-count proof | A | s939 → — → sol:low | opus:low | Closed (709df88a, ed8dbeaf) |
| 916 | `harnez index --stage`, lint names it | B | s939 → — → luna:high | host diff | Closed (ac726732) |
| 936 | practice rule: external-contract tests (AgenticLoop full+lite, lean-sprint checklist) | B | s939 → — → haiku:high | host diff | Closed (68618f27) |
| 915 | `issues close` records model and session (env defaults) | C | s939 → — → luna:high | host diff | Open |

Code fanout (s939, luna:low): 937 is medium: extend the single existing PreToolUse/Bash hook (`config.yaml:279`
forbids a second one), new per-session counter state, threshold and wording in `spec/`. 916 lint at
`internal/issues/maintenance/rebase.go:586`. 936 fits AgenticLoop (`docs/practices/AgenticLoop.md:123`). 915 close
in `internal/issues/status_action.go:98-148`. Decisions in 916 and 915. Stray tool-call text removed from 915/916.
| — | sprint study, `/evergreen` | final | host | — | Open |
| — | `/wrap` | final | host | — | Open |

Phases: A, B and C are independent; run them one after another (one developer at a time). The epic closes after
the final tasks.
