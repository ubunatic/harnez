# 060 — Triage Sibling Managed Docs Drift Captured from Inbox Sweep

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Documentation
**Related**: [059-capture-managed-docs-drift-to-inbox.md](059-capture-managed-docs-drift-to-inbox.md), [013-promote-command.md](013-promote-command.md), [018-mark-bundled-docs-in-frontmatter.md](018-mark-bundled-docs-in-frontmatter.md)

---

## 1. Problem & Motivation

A `harnez diff --capture-docs` sweep across 30 sibling git repositories under `/home/uwe/projects` produced widespread managed-doc differences. The raw counts are useful, but they must not be interpreted as "every repo should have every doc."

Managed docs are technology- and workflow-scoped. A repo should only carry `docs/Rust.md`, `docs/Zig.md`, `docs/GTK4.md`, `docs/Cpp.md`, etc. when that guidance applies to the repo's actual stack or intended agent workflows.

The inbox reports were generated on 2026-08-25 and then summarized before cleaning `issues/inbox/`. The aggregate result across 423 file/section comparisons:

- 63 changed,
- 211 missing,
- 3 extra,
- 146 identical.

The raw missing-doc counts are therefore mostly a signal that harnez needs applicability-aware triage, not blanket backfill:

- `AGENTS.md#Language Conventions` changed in 27 repos.
- `docs/Git.md` changed in 26 repos.
- `docs/Cpp.md` is missing in 30 repos; this is only actionable for C/C++/CGO repos.
- `docs/Rust.md` is missing in 29 repos; this is only actionable for Rust repos.
- `docs/Zig.md` and `docs/GTK4.md` are missing in 28 repos each; these are only actionable for repos using those technologies.
- `docs/IssueTracking.md` is missing in 26 repos.
- `docs/AgenticLoop.md` is missing in 24 repos.

The higher-signal cross-repo drift is in docs that often apply broadly:

- `docs/Git.md` changed in 26 repos.
- `AGENTS.md#Language Conventions` changed in 27 repos, often because it lists a different applicable-doc set.
- `docs/IssueTracking.md` is missing in 26 repos, which may be valid for repos without an in-repo issue tracker.
- `docs/AgenticLoop.md` is missing in 24 repos, which may be valid for repos that do not use the agentic workflow bundle.

This means the managed-doc ecosystem is split between older project-local snapshots, intentionally smaller doc sets, and the current harnez bundle. Agents working in sibling repos may receive materially different workflow guidance depending on repo age and doc selection.

## 2. Findings from Inbox Assessment

Repos with all configured docs missing. These are not automatically wrong; they need a first-pass classification as unmanaged, intentionally minimal, or needing harnez initialization:

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
- `AGENTS.md#Language Conventions` often differs from the current generated section. Some differences may be correct because the repo does not use Rust, Zig, C/C++, GTK4, Agentic Loop Practices, or Issue Tracking Practices.

## 3. Proposed Work

Triage the sibling repos in batches with applicability as the first question. For each repo and doc, decide whether to:

- run `harnez init --doc <name>` or equivalent backfill only when the repo uses that technology/workflow,
- mark the doc as intentionally not applicable so future capture reports do not treat it as noise,
- preserve repo-local deviations as intentional overrides,
- promote useful local changes back to harnez source docs,
- add repo-mode, doc-selection, or auto-detection metadata so intentionally smaller repos do not keep appearing as noisy drift.

The all-missing repos should be checked first to determine whether they are unmanaged projects, intentionally minimal projects, or repositories where docs capture should be explicitly skipped.

## 4. Acceptance Criteria

- [ ] Review the 30-repo drift summary and group repos into applicable backfill, intentional divergence, intentionally not applicable, and skip categories.
- [ ] Backfill missing managed docs only for repos where the technology or workflow applies.
- [ ] Add or design metadata for docs that are intentionally not applicable to a repo.
- [ ] Preserve intentional project-local deviations with clear AGENTS.md notes or future config metadata.
- [ ] Promote any genuinely better repo-local guidance back into harnez source docs.
- [ ] Re-run `harnez diff --capture-docs` across sibling repos and verify the remaining drift is intentional and substantially lower-noise.
- [ ] Consider improving capture report filenames to include the source repo basename for easier inbox triage.
