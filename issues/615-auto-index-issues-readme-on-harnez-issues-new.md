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

- Update `harnez issues new` implementation to run index generation on the tracker directory upon creating the skeleton ticket.
- Output the generated file path and note that `issues/README.md` was refreshed.

## 3. Implementation & Verification Plan

### Goal
Automatically keep `issues/README.md` in sync when allocating new tickets.

### Acceptance Criteria
- [ ] Running `harnez issues new` creates the ticket and immediately updates `issues/README.md`.
- [ ] Unit tests in `cmd/harnez` verifying tracker index synchronization on new issue creation.
