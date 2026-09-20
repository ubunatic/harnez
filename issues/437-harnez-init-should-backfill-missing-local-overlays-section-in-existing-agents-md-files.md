# 437 — harnez init should backfill missing Local Overlays section in existing AGENTS.md files

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Project Initialization / Configuration Drift
**Related**: #076, #311

---

## 1. Problem & Motivation

When `harnez init` runs on a brand new repository, it scaffolds `AGENTS.md` from `docs/templates/AGENTS.md`, which includes:
```markdown
<!-- harnez:begin Local Overlays -->
- Local ephemeral overrides: @AGENTS.local.md
<!-- harnez:end Local Overlays -->
```

However, when `harnez init` runs on an **existing repository** whose `AGENTS.md` was created before `Local Overlays` was introduced (such as `../psync`), `harnez init` adds `AGENTS.local.md` to `.git/info/exclude` and updates `Harnez Managed Conventions` and `Language Conventions`, but **fails to backfill** the `Local Overlays` block into the existing `AGENTS.md`.

As a result, agents running in such repositories do not automatically read `@AGENTS.local.md` on startup unless the block is manually edited in by hand.

---

## 2. /goal & Acceptance Criteria

### /goal
Ensure `harnez init` automatically detects if the `<!-- harnez:begin Local Overlays -->` section is missing from an existing `AGENTS.md` file and inserts it at the top of the file.

### Acceptance Criteria
1. When `harnez init` runs in a directory with an existing `AGENTS.md` lacking the `Local Overlays` block, it injects the `<!-- harnez:begin Local Overlays -->` block near the top (e.g. before other managed sections).
2. Existing local contents outside managed blocks remain preserved.
3. Unit tests in `internal/claude/init_test.go` verify that re-running `init` on a legacy `AGENTS.md` adds the missing `Local Overlays` block idempotently.
