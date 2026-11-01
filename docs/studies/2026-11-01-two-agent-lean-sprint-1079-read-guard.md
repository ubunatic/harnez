---
kind: sprint
---
# Lean sprint 1079: read guard message and spec threshold

- Searcher (haiku:low, 55s): located guard, threshold sites, tests; s3-dev `-L` failure not in telemetry. Wrong on one point: said ranged Claude `Read` already passed.
- Developer (haiku:med): plan turn, then M1 d34d04d3 and M2 3957689d in one turn, one `make test-q1` (0 FAIL), `make install`. It found the searcher's error by being blocked itself on a 5-line ranged Read.
- Host review: diff, every changed assertion (one deny case moved from limit 5 to limit 100, correct since 5 lines encoded the bug), live hook runs, touched packages green.
- Lesson: a searcher's "already works" claim needs a live probe; here the developer's own tool call was the probe.
- Left open: repeated-read deny text; stale comment in hook.go.
