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

## 3. Harnez-Owned Doc Boundary Marker

Add a stop-marker convention for harnez-owned copied docs in `docs/`, such as `docs/Go.md`, `docs/Git.md`, `docs/Make.md`, and other language/practice docs that really belong to harnez.

The marker separates the managed upstream portion from repo-local customization:

- Everything before the marker is harnez-owned content.
- The marker itself means "the harnez-managed part ends here."
- Everything after the marker belongs to the repository/user and can be ignored by drift capture, apply/init reconciliation, and promotion checks unless explicitly requested.

This lets repos add local notes to the same `docs/Go.md`, `docs/Git.md`, or `docs/Make.md` file without forcing a proliferation of sidecar files and without making every local addition look like source-doc drift.

Suggested semantics:

- If repo-local changes occur before the marker, harnez should treat them as candidate upstream drift: adopt, embed, consolidate, or reject explicitly.
- If repo-local changes occur after the marker, harnez should preserve them as local customization and exclude them from normal drift noise.
- If a harnez-owned doc has no marker yet, current whole-file comparison behavior can remain the fallback until the marker is introduced.
- The marker should be inserted only into docs that harnez owns and installs/copies as docs.

Do **not** apply this marker scheme to `AGENTS.md` managed sections. `AGENTS.md` is likely managed by multiple harness-like tools and already uses explicit begin/end markers for sections. The stop-marker convention is specifically for harnez-owned docs under `docs/`.

## 4. Consolidate and Port-Back Pilot

The ticket should also prove the full loop for one controlled sibling repo. Promotion and reapply are two sides of the same workflow:

1. Detect a useful repo-local change in a harnez-owned copied doc.
2. Consolidate and embed the accepted change into the harnez source doc.
3. Reapply/port the updated managed content back to the project where the drift was found, when that project is under our control.
4. Verify that the repo no longer reports that accepted pre-marker drift while any post-marker customization remains preserved.

Promotion/reapply scope:

- Copied docs under `docs/` can be promoted/reapplied.
- Root `AGENTS.md` managed sections can also be reapplied, using the existing begin/end marker model.
- Default reapply should update all managed documents/sections that are already present in the target repo.
- Missing docs are skipped by default; reapply should not install every possible doc just because it exists in harnez.
- Add an option to reapply only one named document/section, e.g. a single `docs/Git.md` or `AGENTS.md`.

Pilot project: `/home/uwe/projects/webman`.

Running this workflow across all sibling projects is explicitly out of scope for this ticket. `webman` is the single pilot for proving the mechanism and clarifying the operator flow.

## 5. Proposed Work

Triage the sibling repos in batches with applicability as the first question. For each repo and doc, decide whether to:

- run `harnez init --doc <name>` or equivalent backfill only when the repo uses that technology/workflow,
- mark the doc as intentionally not applicable so future capture reports do not treat it as noise,
- preserve repo-local deviations as intentional overrides,
- promote useful local changes back to harnez source docs,
- add repo-mode, doc-selection, or auto-detection metadata so intentionally smaller repos do not keep appearing as noisy drift.
- add the harnez-owned doc stop marker and teach capture/reconciliation to ignore post-marker customization for copied docs.
- add a port-back path so accepted/consolidated pre-marker changes can be reapplied to the controlled source project where the drift was found.
- add a default reapply mode for present docs/sections and a focused reapply mode for one named doc/section.

The all-missing repos should be checked first to determine whether they are unmanaged projects, intentionally minimal projects, or repositories where docs capture should be explicitly skipped.

## 6. Acceptance Criteria

- [ ] Review the 30-repo drift summary and group repos into applicable backfill, intentional divergence, intentionally not applicable, and skip categories.
- [ ] Backfill missing managed docs only for repos where the technology or workflow applies.
- [ ] Add or design metadata for docs that are intentionally not applicable to a repo.
- [ ] Define a stop marker for harnez-owned copied docs under `docs/`.
- [ ] Update docs capture/reconciliation so pre-marker changes are reported as upstream drift and post-marker changes are preserved/ignored as local customization.
- [ ] Keep `AGENTS.md` on its existing begin/end managed-section model; do not apply the copied-doc stop-marker scheme there.
- [ ] Preserve intentional project-local deviations with clear AGENTS.md notes or future config metadata.
- [ ] Promote any genuinely better repo-local guidance back into harnez source docs.
- [ ] Use `/home/uwe/projects/webman` as the single pilot to prove consolidate/embed/port-back behavior.
- [ ] Reapply `AGENTS.md` managed sections in `/home/uwe/projects/webman` as part of the pilot.
- [ ] Default reapply updates present managed docs/sections only and skips missing docs.
- [ ] Add an option to reapply one specific document/section.
- [ ] After accepting a pre-marker change from `webman`, embed it in harnez and reapply it back to `webman` without touching unrelated sibling repos.
- [ ] Verify post-marker `webman` customization remains preserved and ignored by normal drift capture.
- [ ] Re-run `harnez diff --capture-docs` across sibling repos and verify the remaining drift is intentional and substantially lower-noise.
- [ ] Consider improving capture report filenames to include the source repo basename for easier inbox triage.
