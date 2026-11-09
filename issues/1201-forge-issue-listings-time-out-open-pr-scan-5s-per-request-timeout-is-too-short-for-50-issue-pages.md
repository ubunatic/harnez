# 1201 — forge issue listings time out: open_pr_scan 5s per-request timeout is too short for 50-issue pages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 1198 (forge closed/active listing), 1139 (forge status filters), 1196 (dup scan page budget)

---

/goal Forge issue listings (`find issues closed`, `-a status:active`, `status:` filters) finish reliably on
codeberg.org: issue listing gets its own timeout in `spec/pr.yaml` instead of borrowing `open_pr_scan.timeout`,
and a timed-out page reports as a timeout, not "HTTP 200 decoding response". Stop and report when blocked on a
user decision; on a loop or no progress, clear the goal and write a failure report here.

## Problem

taia session, 2026-11-09, harnez 0.5.6+4975: `find -d . issues closed` and `-a status:active` on
codeberg.org/harnez/taia failed 4/4 after 5.07–5.09 s with "forge API request failed: request to codeberg.org
failed" and once "HTTP 200 decoding response from codeberg.org" (body cut off by the deadline). An hour earlier
the same commands worked. From the harnez host the same command succeeded twice in 6.3 s and 7.2 s total.

The listing uses `open_pr_scan` (timeout 5s, page_size 50, max_pages 4); a 50-issue page with bodies on a slow
Codeberg moment exceeds 5 s. Not caused by PR #140 (that change only alters the header line).

## Want

- Separate `issue_scan` (timeout, page size, pages) in `spec/pr.yaml` + schema; issue listings use it.
- Deadline mid-body reported as a timeout with the setting name to raise.
- Test with a slow httptest server.

## Notes

- Unverified: whether Codeberg latency or body size dominates; measure a page at 50 vs 20 before picking values.
