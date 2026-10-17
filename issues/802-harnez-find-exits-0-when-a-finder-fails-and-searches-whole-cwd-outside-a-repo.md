# harnez find exits 0 when a finder fails and searches whole cwd outside a repo

Status: Open

## Problem
Follow-up from 801 (dev-harnez review, 2026-10-17):
- `harnez find` returns exit status 0 even when a finder (rg) errors, so agents and scripts cannot detect failure.
- In a non-repo cwd (e.g. `/tmp`, `~/projects`), `harnez find code` scans the whole cwd recursively, which is slow and noisy.

## Acceptance
- Non-zero exit (distinct code) when any finder errors without results; `partial` keeps 0 but prints a warning to stderr.
- Outside a git repo, find code requires `-d` or limits depth / prints a hint.
- Tests for both.
