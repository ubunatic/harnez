# 834 — lean-sprint: developer runs the real deploy or host command once before committing

**Status**: Backlog — lower priority than the active 20; parked to keep the cap
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `docs/commands/lean-sprint.md`, issue 770, ubunatic.com issue 080

---

## 1. Goal

/goal The lean-sprint skill requires the Developer to run the real command once (dry run, read-only host probe, or the installed binary) for any milestone that adds deploy, SSH or host-facing scripts, before committing; or stop and report when the wording conflicts with other skill rules and needs a user decision.

## 2. Findings (ubunatic.com sprint 080, 2026-10-20)

Staging deploy scripts passed `bash -n` and `make check` but three bugs showed only on the real host, each costing a review round:

1. `go run ./scripts/stagearchive` from the repo root failed (module lives in `scripts/go.mod`); one local run would have shown it.
2. The SSH account is chrooted (`/` = `/var/www/vhosts/venile.de`), so an existence check on the Apache path failed.
3. The password file at mode 640 was unreadable for Apache (500 on every login).

A Canary probe (`docs/Canary.md`) at the start would have caught 2 and 3 before code was written.

## 3. Plan

1. In `docs/commands/lean-sprint.md` §3: deploy/host milestones run the real command once (dry run or read-only probe) and paste the output; tests and syntax checks alone do not count.
2. In §1: for host-facing tickets the Searcher does a read-only host probe (environment, paths, permissions) when SSH access is non-interactive.
3. Reinstall the skill and verify the installed copy.
