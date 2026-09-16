# 374 — Refresh README CLI coverage and website link

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: [README](../README.md), [092](092-latest-release-links-and-readme-install-section-consolidation.md)

---

README drift found while refreshing harnez.org against source revision `0a2cce5`:

- Debloat documentation names Claude and Codex but omits implemented Antigravity support.
- `scan-docs` is documented with `-c <dir>`; actual syntax is `harnez scan-docs <dir>`, while `-c` selects a config file.
- Command reference omits `docs variant`, `exec`, `rate`, `stats`, and `issues`.
- Website link still points to `ubunatic.com/harnez`; update it to the standalone `https://harnez.org/` site.

Acceptance: refresh these sections against current command help and configuration, distinguish source-only features from released availability, and verify documented syntax and links. This is one bounded README correction, separate from #092's broader installation-convention work.
