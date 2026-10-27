---
kind: sprint
---

# Two-agent lean sprint: 975, Priority column in `harnez find issues`

Date: 2026-10-27. Host: Claude Opus 5.5. Searcher `codex:luna:low` (`search-975`, one turn, 27s, 47k new /
230k cached). Developer `codex:luna:med` (`dev-975`, one turn, 3m, 79k new / 2.1M cached).

## Searcher findings

Rows come from `find.Search` and `find.FormatTSV` (`internal/find/search.go`), shared by `harnez issues list`;
Priority was not parsed anywhere. The README and ticket 158 document a four-column TSV contract; no script, skill,
hook or test in the repo splits the rows on tabs.

## Decision

Priority appended as column 5 (after PATH) rather than next to the status, so the documented column positions stay
valid for outside consumers. `--json` gains `priority`; `-I` cards unchanged. The issue index (`issues/README.md`)
is not touched: the owner raised index churn, and `find` reads tickets live, so no stored copy needs updating.

## Delivery

- M1 `58ce3c60`: `ParseIssueFileWithPriority`, `IssueFile.Priority`, `Result.Priority`, TSV column 5, README
  contract; tests for `P1 (High) — reason` → `P1`, missing / `High` / `P9` → empty, five columns per row.
- Closed by the developer (`dd387c11`) with live and test evidence.

## Verification

Developer: `make test-q1` 58 `ok`, 0 `--- FAIL`. Host: touched packages ok; live listing of 47 active tickets, all
five columns, Priority present on every row (37 P2, 9 P3, 1 P1).

## Note

The priority regex scans every line until the first match, not only the header block, so a ticket without a header
Priority but with `**Priority**: P0` in its body would show P0. No current ticket hits this.
