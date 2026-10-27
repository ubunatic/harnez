# 975 — harnez find issues should list the Priority

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 194 (harnez find issues next), 318 (issues list verb)

---

/goal `harnez find -d <repo> issues -a status:active` prints each active ticket's Priority (P0-P3) next to its status,
verified by a test and a live run; or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

`harnez find -d <repo> issues -a status:active` lists number, status, title and file path, but not the ticket's
Priority. Users and agents must open each file to see how urgent a ticket is before choosing what to work on next.

## 2. Technical Specification / Findings

- The text listing is produced by the issue find path (`cmd/harnez/issues.go`, `runIssuesList` and the find output
  it feeds). Confirm the exact column source in code before changing it; the Priority is already in each ticket's
  `**Priority**:` header line.
- Keep the existing columns and their order; add Priority as a new column. Show the code only (`P0`-`P3`), not the
  parenthesised label, to keep the rows short.
- Tickets without a Priority header show a blank cell rather than failing the listing.

## 3. Implementation & Verification Plan

1. Add the Priority column to the active-issue text listing; keep `-I/--image` output unchanged unless it shares the same row builder.
2. Add or extend a test that asserts the Priority appears in the listed row for a known ticket.
3. Verify with `harnez find -d /home/uwe/projects/harnez issues -a status:active`, then `make test-q1` and `make install`.

## 4. Searcher findings (2026-10-27, luna:low)

- Rows: `find.Search` (`internal/find/search.go:32`) builds `Result`s; `find.FormatTSV` (`search.go:101`) formats
  them; `runFindIssues` (`internal/find/command.go:330`) prints. `harnez issues list` uses the same path
  (`cmd/harnez/issues.go:479`). `--json` serializes `Result`; `-I` uses its own card renderer
  (`internal/readcard/issues_card.go:193`); `next`, `similar`, `history` have separate output paths.
- Priority is not parsed: `IssueFile` (`internal/issues/issues.go:18`) and `ParseIssueFile` (`issues.go:144`) carry
  title and status only.
- Documented contract: four TSV columns `NUMBER, RAW_STATUS, PLAIN_TITLE, PATH` (`README.md:386`, ticket 158
  line 137). No script, Makefile, skill, hook or test splits these rows on tabs.
- Tests: `TestFormatTSV`, `TestFormatTSV_SanitizesEmbeddedTabsAndNewlines` (`internal/find/search_test.go:225`),
  `TestRunFind` (`internal/find/command_test.go:43`).

## Decision

- Priority is appended as **column 5** (`NUMBER, RAW_STATUS, PLAIN_TITLE, PATH, PRIORITY`) instead of next to the
  status, so the documented column positions stay valid for any outside consumer. Reversible; the owner can ask
  for column 2 instead.
- `--json` gains a `priority` field through `Result` (additive). `-I` cards stay unchanged.
- Priority value: the code only (`P0`-`P3`) parsed from `**Priority**:`; anything else or missing → empty cell.
- Developer: `luna:med` (973 default). Searcher findings above replace a plan-only turn.

## Milestones

- **M1 (Priority column):** parse Priority in `internal/issues`, carry it through `find.Result`, append in
  `FormatTSV`; update README and the four-column note in ticket 158 is history, leave it. Tests: a ticket with
  `P1 (High) — reason` gives `P1`; missing header and a malformed value (`High`, `P9`) give empty; tab
  sanitising still holds; `TestRunFind` row has five columns. Verify per section 3. One commit naming issue 975 M1.
