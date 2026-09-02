# 184 — Add `harnez feedback` Command for Agent-Filed Gap/Bug Reports

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [issues/181](181-narrow-harnez-rate-to-failure-cases.md) (narrowed rate policy — the sibling mechanism this complements), [issues/183](183-session-state-assessment-and-reminders.md) (session-state store this can piggyback on)

---

## 1. Problem & Motivation

`harnez rate` records a per-tool-call quality signal against a specific ticket the agent already
knows about. There's no equivalent mechanism for an agent to autonomously surface something new it
noticed mid-session — a gap in its own instructions, a harnez bug, a confusing doc, a missing
feature — without that observation either being lost when the session ends or requiring the agent
to manually hand-author a full `issues/NNN-*.md` file (which most agents currently don't do
unprompted). This ticket adds a low-friction command an agent can call the moment it notices
something, so the observation gets captured durably instead of only living in the conversation.

## 2. Technical Specification / Findings

Add `harnez feedback issue "<short description>" [--severity ...] [--project <dir>]` (exact flag
names TBD during implementation — mirror `harnez rate`'s existing argument style where sensible)
that:

- Writes a lightweight feedback record — NOT necessarily a full `issues/NNN-*.md` ticket file
  immediately. Consider two tiers:
  1. A cheap append-only feedback log (e.g. `~/.harnez/feedback/<project-hash>.jsonl` or similar,
     consistent with the session-state storage convention from issue 183) for low-friction capture.
  2. An explicit `--file-ticket` flag (or a separate `harnez feedback promote <id>`) that turns a
     logged feedback entry into a proper `issues/NNN-*.md` ticket following `docs/IssueTracking.md`'s
     metadata schema, for cases where the agent judges the observation is substantial enough to
     track formally.
- Should be genuinely low-effort for an agent to call in the middle of other work — a single
  command with a free-text description, not a multi-field form — since friction is exactly what
  suppresses this behavior today.
- Needs a review/triage story: who looks at the raw feedback log? At minimum, `harnez status` or a
  new `harnez feedback list` should surface unreviewed entries so they don't silently pile up
  forgotten (same failure mode as `harnez rate` being ignored — see issue 181's motivation).

## 3. Implementation & Verification Plan

1. Decide the storage tier design (raw log vs. immediate ticket file vs. both) — default to the
   two-tier design above unless implementation reveals a simpler approach.
2. Implement `harnez feedback issue <description>` writing to the log; implement `harnez feedback
   list` (or extend `harnez find`) to surface unreviewed entries.
3. If scope allows, implement promotion to a full ticket file reusing existing `issues/` filing
   logic/conventions (ticket numbering via `harnez find`/`harnez index`, not manual `ls`).
4. Add unit tests for the log format and (if implemented) the promotion path.
5. Verify with `go test ./...`; run `make install`; update `issues/README.md`.
