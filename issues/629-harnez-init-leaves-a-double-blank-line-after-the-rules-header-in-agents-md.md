# 629 — harnez init leaves a double blank line after the rules header in AGENTS.md

**Status**: Closed — Fixed rules header spacing after Local Overlays migration; regression test added. make test-q1 encountered 17 known internal/claude failures tied to unrelated config.yaml edits and one initial assertion issue corrected after the quota run; make install passed.
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 625

---

## 1. Problem & Motivation
When `harnez init` replaces the old `Local Overlays` block with the new one-call rules header
(issue 625), AGENTS.md ends up with two blank lines between the header and
"Adhere to the following conventions." Seen in loom on 2026-09-28. Cosmetic, but it shows up
in every migrated repo's diff.

## 2. Technical Specification / Findings
Resulting AGENTS.md start:

```
**Before any work, read all Harnez rules in one call: `harnez read ...`.**
<blank>
<blank>
Adhere to the following conventions.
```

The header writer likely appends its own trailing blank line while the blank line that
followed the removed `<!-- harnez:end Local Overlays -->` marker is kept.

## 3. Implementation & Verification Plan
- Collapse to one blank line when writing the rules header.
- Test: migrate a fixture AGENTS.md with a `Local Overlays` block and assert exactly one blank
  line after the header; running `harnez init` again must not change the file.
