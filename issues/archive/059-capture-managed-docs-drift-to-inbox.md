# 059 — Capture Managed Docs Drift to Inbox Markdown

**Status**: Closed — resolved in 32218cb
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [docs/CLIDesign.md](../docs/CLIDesign.md), [013-promote-command.md](013-promote-command.md), [018-mark-bundled-docs-in-frontmatter.md](018-mark-bundled-docs-in-frontmatter.md)

---

## 1. Problem & Motivation

Harnez can install and compare configured docs, but there is no workflow for capturing how a current repository's docs have diverged from the configured source docs into a durable review artifact.

When a repo-local `AGENTS.md`, bundled doc, or copied language/practice doc has drifted, an agent or maintainer should be able to ask harnez to produce a focused markdown report that shows:

- which managed docs were compared,
- where the current repository diverges,
- the unified diff or summarized diff hunks for each changed file,
- enough source context to decide whether the repo-local edits should be promoted, discarded, or turned into new bundled guidance.

This is adjacent to `harnez diff`, but the output target is a plain markdown inbox artifact suitable for later triage rather than terminal-only inspection.

## 2. Proposed Feature

Add a command or flag that captures configured-docs drift into a markdown file.

Suggested UX:

```bash
harnez diff --capture-docs
harnez diff --capture-docs --out issues/inbox/my-repo-doc-drift.md
```

Default output path:

```text
<harnez-repo-if-accessible>/issues/inbox/<simple-file>.md
```

If the harnez repo is not accessible, the command should fall back to a sensible local path in the current repository, or require `--out` with a clear error.

The generated inbox file should be intentionally lightweight. Inbox reports are not full issue tickets and do not need the issue-tracker metadata block.

## 3. Inbox Report Format

Inbox report files should be plain markdown with optional YAML front matter.

Recommended front matter:

```yaml
---
source_repo: /absolute/or/display/path/to/source/repo
files:
  - AGENTS.md
  - docs/Go.md
  - docs/practices/AgenticLoop.md
---
```

The body should include:

- source repo path or VCS identity,
- harnez config/source doc set used for comparison,
- list of files compared,
- per-file diff sections,
- short summary of added, removed, and changed docs.

Front matter must remain optional so other reporters can drop plain markdown into `issues/inbox/` without conforming to the full issue schema.

## 4. Acceptance Criteria

- [x] Add a docs-drift capture workflow that compares configured docs against the current repository docs.
- [x] Write the sum of diffs to a new markdown file.
- [x] Default to `<harnez-repo-if-accessible>/issues/inbox/<simple-file>.md`.
- [x] Allow an explicit output path override.
- [x] Include source repo and compared file list in optional YAML front matter.
- [x] Keep `issues/inbox/*.md` exempt from the formal issue front matter and tracker index requirements.
- [x] Add tests covering changed, missing, extra, and identical managed docs.
