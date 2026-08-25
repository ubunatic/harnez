# 060 — Triage Sibling Managed Docs Drift Captured from Inbox Sweep

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Documentation
**Related**: [059-capture-managed-docs-drift-to-inbox.md](059-capture-managed-docs-drift-to-inbox.md), [013-promote-command.md](013-promote-command.md), [018-mark-bundled-docs-in-frontmatter.md](018-mark-bundled-docs-in-frontmatter.md)

---

## 1. Problem & Motivation

A `harnez diff --capture-docs` sweep across 30 sibling git repositories under `/home/uwe/projects` produced widespread managed-doc drift.

The inbox reports were generated on 2026-08-25 and then summarized before cleaning `issues/inbox/`. The aggregate result across 423 file/section comparisons:

- 63 changed,
- 211 missing,
- 3 extra,
- 146 identical.

The highest-signal drift is not random:

- `AGENTS.md#Language Conventions` changed in 27 repos.
- `docs/Git.md` changed in 26 repos.
- `docs/Cpp.md` is missing in 30 repos.
- `docs/Rust.md` is missing in 29 repos.
- `docs/Zig.md` and `docs/GTK4.md` are missing in 28 repos each.
- `docs/IssueTracking.md` is missing in 26 repos.
- `docs/AgenticLoop.md` is missing in 24 repos.

This means the managed-doc ecosystem is split between older project-local snapshots and the current harnez bundle. Agents working in sibling repos may receive materially different workflow guidance depending on repo age.

## 2. Findings from Inbox Assessment

Repos with all configured docs missing:

- `/home/uwe/projects/homeserver`
- `/home/uwe/projects/trafficsim`
- `/home/uwe/projects/videos`

Repos with the most changed docs:

- `/home/uwe/projects/harnez.org`: 5 changed, 7 missing.
- `/home/uwe/projects/webman`: 4 changed, 5 missing.
- `/home/uwe/projects/lmcoder`: 3 changed, 4 missing, 2 extra.
- `/home/uwe/projects/pdf-doctor`: 3 changed, 6 missing.
- `/home/uwe/projects/spriteview`: 3 changed, 6 missing.
- `/home/uwe/projects/venile.de`: 3 changed, 6 missing.
- `/home/uwe/projects/ziggo`: 3 changed, 6 missing.

Sample high-value divergence:

- `docs/Git.md` differs in commit/checkpoint guidance. Some repo-local copies omit the newer rule to commit issue-tracker entries immediately in their own small commit.
- `AGENTS.md#Language Conventions` often lacks newer entries for Rust, Zig, C/C++, GTK4, Agentic Loop Practices, and Issue Tracking Practices.

## 3. Proposed Work

Triage the sibling repos in batches and decide for each drift class whether to:

- run `harnez init --doc <name>` or equivalent backfill,
- preserve repo-local deviations as intentional overrides,
- promote useful local changes back to harnez source docs,
- add repo-mode or doc-selection metadata so intentionally smaller repos do not keep appearing as noisy drift.

The all-missing repos should be checked first to determine whether they are real projects that should be harnez-managed or repositories where docs capture should be explicitly skipped.

## 4. Acceptance Criteria

- [ ] Review the 30-repo drift summary and group repos into backfill, intentional divergence, and skip categories.
- [ ] Backfill missing managed docs for repos that should follow the current harnez bundle.
- [ ] Preserve intentional project-local deviations with clear AGENTS.md notes or future config metadata.
- [ ] Promote any genuinely better repo-local guidance back into harnez source docs.
- [ ] Re-run `harnez diff --capture-docs` across sibling repos and verify the remaining drift is intentional and substantially lower-noise.
- [ ] Consider improving capture report filenames to include the source repo basename for easier inbox triage.
