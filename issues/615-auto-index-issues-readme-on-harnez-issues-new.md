# 615 — Auto-index issues README on harnez issues new

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation

When creating a new issue with `harnez issues new -d <repo> "<title>"`, it creates the ticket markdown file, but does not update `issues/README.md`. The user or agent must separately execute `harnez index -d <repo>`.
In contrast, `harnez issues <verb>` (e.g. `close`) updates `issues/README.md` automatically.

Making `harnez issues new` automatically update the index eliminates this asymmetric step.

## 2. Technical Specification / Findings

- After the O_EXCL reservation successfully creates the placeholder, call the same in-process `index.UpdateIssuesReadme` logic used by other `issues` verbs.
- Preserve non-JSON stdout exactly as `<NUMBER>\t<PATH>` for scripts; write the "README refreshed" note to stderr.
- The index shows the placeholder title until ticket content is written. Other `issues` verbs refresh the index later when they update the ticket.

## 3. Implementation & Verification Plan

### Goal
Automatically keep `issues/README.md` in sync when allocating new tickets.

### Acceptance Criteria
- [x] Running `harnez issues new` creates the ticket and immediately updates `issues/README.md`.
- [x] Unit tests in `cmd/harnez` verifying tracker index synchronization on new issue creation and preserving stdout format.

## 4. Outcome

`harnez issues new` now refreshes `issues/README.md` after the race-safe reservation, and reports the refresh on stderr while retaining the existing stdout contract. The generated row contains the placeholder title until content is written; later `issues` verbs refresh it as they update the ticket.
