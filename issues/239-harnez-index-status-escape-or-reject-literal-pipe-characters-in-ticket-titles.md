# 239 — harnez index/status: escape or reject literal pipe characters in ticket titles

**Status**: Open — harnez status catch during 2026-09-05 evergreen review
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: `internal/index/index.go` (`IssuesTable`, ~line 79 — writes `f.Title` into a
table cell with no escaping), `internal/issues/issues.go` (`ParseTrackerTable`, `Lint`,
`tableNumCount` — the reader side that mis-parses the resulting row), ticket 233 (the
ticket whose own title, containing `docs/practices|lang|other source`, triggered this
during the 2026-09-05 `/evergreen` review)

---

## 1. Problem & Motivation

Discovered live during a routine `harnez status` check: ticket 233's title contained
literal, unescaped `|` characters (`docs/practices|lang|other source`). `IssuesTable`
writes ticket titles directly into a Markdown table cell with no escaping, so the `|`
characters were interpreted as extra column separators when the table was regenerated.
`ParseTrackerTable`/`Lint` then mis-parsed the row, and `harnez status` reported a false
`unindexed ticket file '233-...' not present in README table` diagnostic even though the
ticket was correctly indexed — its row just didn't parse as one row anymore.

Worked around for 233 by rewording the title to avoid literal pipes (commit `d3d2dbd`),
but nothing stops the next ticket title (agent-authored, often built from user phrasing)
from doing the same thing again.

## 2. Technical Specification / Findings

Two independent bugs, either of which would resolve this if fixed alone:

- **Write side** (`IssuesTable`, `internal/index/index.go` ~line 79): should escape any
  `|` in `f.Title` as `\|` before writing the table row, per standard Markdown table
  escaping.
- **Read side** (`ParseTrackerTable`, `internal/issues/issues.go`): should either handle
  `\|`-escaped pipes correctly when splitting a row into cells, or (if the write side is
  fixed) at least not silently misinterpret an unescaped pipe as a column boundary in a
  way that produces a plausible-looking-but-wrong row.

## 3. Implementation & Verification Plan

Not yet planned — no implementation plan section added by this ticket's filing pass.
Fixing the write side alone (escape on generation) is likely sufficient for all
future-created tickets; the read side only matters for titles already on disk (or
future titles created without the escaping fix, e.g. by a stale binary).

Verification: a table-driven test with a ticket title containing `|`, round-tripped
through `IssuesTable` generation and `Lint`, asserting zero diagnostics.
