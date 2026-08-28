# 081 — Table Hyperlink Labels Break Layout; Flowing Text Should Keep Full Paths

**Status**: Closed — resolved in docs/lang/Markdown.md § Terminal hyperlinks (OSC 8): label text by context
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [Doc / Ticket / Commit references]

## Problem

Agents correctly use terminal OSC 8 hyperlinks (clickable file links) instead of printing raw
paths — this is good and should continue. But the rule for the *visible label text* needs to
differ by context:

- **In tables**: agents often use the full/long filename as the hyperlink label. Long filenames
  blow up column width and break table layout/alignment in the terminal.
- **In flowing text** (non-table prose): the user wants the *full path* shown as visible text
  (not just a short clickable label), so they can visually scan it and drag-select/copy it
  directly, rather than having to click the link and extract the URL. Full paths should only be
  shortened here if they are genuinely too long to read comfortably inline.

## Desired Behavior

- Table cells containing file links: use a short/truncated label text with the OSC 8 hyperlink
  target set to the full path. Keep columns narrow and aligned.
- Flowing text file references: show the full path as visible text, with the OSC 8 hyperlink
  layered on top for click-to-open convenience. Truncate only when the full path would be
  unreasonably long for inline prose.

## Next Steps

- Add guidance to `docs/lang/Markdown.md` (or a terminal-output-specific doc) codifying this
  context-dependent rule: short label + hyperlink in tables, full visible path + hyperlink in
  prose.
