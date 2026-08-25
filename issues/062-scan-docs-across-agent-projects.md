# 062 — `harnez scan-docs`: Combined Managed-Docs Scan Across Agent Projects

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [059-capture-managed-docs-drift-to-inbox.md](059-capture-managed-docs-drift-to-inbox.md), [060-triage-sibling-managed-docs-drift.md](060-triage-sibling-managed-docs-drift.md)

---

## 1. Problem & Motivation

After `harnez diff --capture-docs`, scanning many sibling projects still requires ad-hoc shell loops. We need one read-only command that can inspect a directory of projects, select only likely agent/coding projects, and produce a compact combined docs-drift report without touching unrelated directories.

## 2. Proposed Command

```bash
harnez scan-docs /home/uwe/projects
```

Default behavior:

- Scan immediate child directories only; no recursion for now.
- A child is eligible only if it has `AGENTS.md` or `CLAUDE.md`; symlink vs regular file does not matter.
- `.git` alone is not enough.
- Never modify scanned projects.
- Use existing docs-drift comparison logic.
- Suppress missing-doc noise by default; do not report missing-doc counts.
- Produce one combined report for multi-project scans.
- When scanning one repo directly, allow a more detailed repo report.
- Clean repos are summarized as one line with basenames only.

Suggested output:

```text
Scanned: 18 eligible, 12 skipped
Clean: books, emojig, wayreel
Drift:
  webman      docs/Git.md changed, AGENTS.md changed
  lmcoder     docs/Go.md changed, docs/README.md extra
```

## 3. Acceptance Criteria

- [ ] Add `harnez scan-docs <dir>`.
- [ ] Scan only immediate children by default.
- [ ] Require `AGENTS.md` or `CLAUDE.md` for child eligibility.
- [ ] Ignore `.git`-only directories.
- [ ] Generate one combined report for directory scans.
- [ ] Generate a more detailed report when the target is a single eligible repo.
- [ ] Suppress missing-doc counts and missing-doc sections by default.
- [ ] Include clean repo basenames in one compact line.
- [ ] Add focused tests for eligibility, symlink acceptance, skipped dirs, clean summary, and missing-doc suppression.
