# 750 — AgenticLoop.lite.md: heading markers inside bullets and pointers to an uninstalled full doc

**Status**: Closed — cleaned heading markers in AgenticLoop.lite.md, added variant commands in lite docs, and added structural gate test
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [681 — AgenticLoop links to non-copyable TUIDesign](681-copyable-agenticloop-doc-links-to-non-copyable-tuidesign.md), [701 — init duplicates Quota-1 section](701-harnez-init-duplicates-the-quota-1-rules-section.md), [358 — variant marker](358-self-describing-variant-marker-so-drift-detection-tolerates-lite-docs.md)

---

## 1. Problem & Motivation
Found while reviewing `harnez init --quota-1 --variant lite` (harnez 0.1.24) in the cati
repo on 2026-10-07. The lite docs are what agents read in place of the full docs, so
leftover markup and pointers to text that is not installed cost context and mislead.

**Goal**: The lite docs render as clean Markdown and every "see the full doc" pointer
tells the reader the exact command to get it.

## 2. Technical Specification / Findings
`docs/practices/AgenticLoop.lite.md`:
- Lines 30–34: each phase bullet starts with a literal heading marker,
  `- **### Phase 1: …**`. The `###` renders as text inside bold. Leftover from
  condensing the full doc's `### Phase N` headings.
- Line 37: `### Workflow Selection Matrix:` sits mid-paragraph, same leftover.
- Lines 9–10: "See `AgenticLoop.md` (`harnez docs variant <name> full`)". In an installed
  project the lite file *is* `AgenticLoop.md`, so this points at itself, and `<name>` is a
  placeholder. `IssueTracking.lite.md` line 4 shows the right form:
  `harnez docs variant issue-tracking full`.
- Line 29: "see the full doc for step-by-step detail", with no command.

`docs/practices/GoRelease.lite.md` line 5: "Full doc carries the rationale and worked
examples", with no command either.

Minor and cosmetic: the `<!-- harnez:variant=lite -->` marker is line 1, so the YAML
frontmatter (`title`, `weight`) is no longer at the top of the file. No harnez Go code
parses that frontmatter (checked: no `---` prefix parsing outside issues and skill export),
so this only matters for external renderers that expect frontmatter on line 1.

## 3. Implementation & Verification Plan
- Remove the `###` markers from the bullets and from the mid-paragraph matrix label in
  `AgenticLoop.lite.md`.
- Replace every bare "full doc" pointer in `*.lite.md` with the concrete
  `harnez docs variant <doc-id> full` command, as `IssueTracking.lite.md` already does.
- Add a test over `docs/**/*.lite.md` that rejects `**###` and ` ### ` inside a line and
  "full doc" mentions without a `harnez docs variant` command.
- Verify: `harnez init --variant lite` in a scratch repo; `grep -n '###' docs/AgenticLoop.md`
  returns only real headings.
