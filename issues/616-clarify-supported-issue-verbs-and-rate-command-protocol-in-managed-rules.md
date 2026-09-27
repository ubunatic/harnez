# 616 — Clarify supported issue verbs and rate command protocol in managed rules

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation

1. In `.harnez/rules/Tools.md` and `docs/IssueTracking.md`, the documentation references:
   `harnez issues <verb> -d <repo> <n> [reason]`
   without explicitly listing valid verbs (`open`, `start`, `block`, `close`, `done`, `draft`), which can cause failed tool attempts when agents guess verbs like `resolve` or `finish`.
2. `harnez tip` frequently suggests `harnez rate` on failed commands or tool call milestones, but managed rules in `.harnez/rules/Tools.md` do not provide a concise summary of the rating protocol and its expected lifecycle.

## 2. Technical Specification / Findings

- Update `.harnez/rules/Tools.md` and `docs/IssueTracking.md`:
  - Enumerate supported verbs: `open`, `start`, `block`, `close`, `done`, `draft`.
  - Add a 2-line summary on `harnez rate` usage in `.harnez/rules/Tools.md`.

## 3. Implementation & Verification Plan

### Goal
Provide clear, actionable syntax documentation in managed rules and docs.

### Acceptance Criteria
- [ ] Supported issue status verbs clearly documented in `Tools.md` and `IssueTracking.md`.
- [ ] `harnez rate` guidance documented in managed rules.
