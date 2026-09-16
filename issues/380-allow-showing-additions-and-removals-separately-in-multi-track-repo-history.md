# 380 — Allow showing additions and removals separately in multi-track repo history

**Status**: In Progress
**Priority**: P2 (Medium)  
**Severity**: Minor  
**Category**: Feature  

**Related**: [376](376-multi-track-git-history-evolution-sparks-across-code-tests-docs-skills-and-issues.md), [379](379-double-resolution-braille-sparklines-and-addition-removal-timeline-for-repo-evolution.md)  

---

## 1. Summary & Motivation

In `harnez dochistory --tracks` and `harnez repo-history`, each track's sparkline currently displays net volume over time.

However, developers frequently want to see **additions and removals/pruning separately** to distinguish between:
- Clean continuous additive feature work ($+50\text{k}$ LOC, $-2\text{k}$ LOC).
- Heavy restructuring / refactoring churn ($+50\text{k}$ LOC, $-48\text{k}$ LOC).

This ticket adds `--diff` / `--diffs` flag support to `harnez repo-history` and `harnez dochistory --tracks` to compute and display separate Addition ($+$ in Green) and Removal ($-$ in Red) Braille sparklines and metrics alongside net totals.

---

## 2. Visual Specification

### With `--diff` Flag (`harnez repo-history --diff`)

```text
Repo Evolution Diffs (harnez · 1150 commits · 120d)
Code:
  [+] Adds: [⣀⣠⣤⣴⣶⣶⣿⣿⣿⣿] +54.2k LOC
  [-] Rms:  [⣀⣀⣠⣤⣤⣶⣶⣿⣿] -10.7k LOC
  [=] Net:  [⣀⣠⣤⣴⣶⣶⣿⣿⣿⣿]  43.5k LOC (400k tokens)
Tests:
  [+] Adds: [⣀⣀⣠⣤⣴⣶⣿⣿⣿⣿] +38.1k LOC
  [-] Rms:  [⣀⣀⣀⣠⣤⣤⣶⣶⣿] -7.6k LOC
  [=] Net:  [⣀⣀⣠⣤⣴⣶⣿⣿⣿⣿]  30.5k LOC (278k tokens) · 0.70 test/code ratio
Docs:
  [+] Adds: [⣀⣠⣤⣶⣶⣶⣶⣿⣿⣿] +340k tokens
  [-] Rms:  [⣀⣀⣠⣤⣤⣶⣶⣿⣿] -62k tokens
  [=] Net:  [⣀⣠⣤⣶⣶⣶⣶⣿⣿⣿]  278k tokens (146 files)
Skills:
  [+] Adds: [⣠⣤⣴⣶⣶⣶⣾⣿⣿⣿] +24 skills
  [-] Rms:  [⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀]  -5 skills
  [=] Net:  [⣠⣤⣴⣶⣶⣶⣾⣿⣿⣿]  19 skills (17.1k tokens)
Issues:
  [+] Adds: [⣀⣀⣠⣤⣴⣶⣶⣿⣿⣿] +380 tickets
  [-] Rms:  [⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀]    0 tickets
  [=] Net:  [⣀⣀⣠⣤⣴⣶⣶⣿⣿⣿] 380 tickets (150 open · 230 closed)
```

---

## 3. Acceptance Criteria

- [ ] `internal/assess/tracks.go` computes cumulative additions and removals time-series per track.
- [ ] `TrackEvolution` includes `AddedPoints`, `RemovedPoints`, `TotalAdded`, `TotalRemoved`, `AddSparkline`, and `RemoveSparkline`.
- [ ] `harnez repo-history --diff` and `harnez dochistory --tracks --diff` render the expanded additions/removals breakdown.
- [ ] `--json` output returns `total_added`, `total_removed`, `add_sparkline`, and `remove_sparkline` for each track.
- [ ] Unit tests in `internal/assess/` and `cmd/harnez/` verify diff calculation accuracy and terminal rendering.
