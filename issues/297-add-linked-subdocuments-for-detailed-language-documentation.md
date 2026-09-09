# 297 — Add linked subdocuments for detailed language documentation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Documentation
**Related**: `docs/lang/Go.md`, `docs/lang/GoWork.md` (proposed), `harnez init --docs`

---

## 1. Problem & Motivation

The copyable language documents should remain short and token-efficient for
ordinary coding work. Detailed concerns currently expand the main document,
even when most agents only need them for specialized tasks. For example,
Go-workspace management is important when managing multiple modules, copying
projects, or diagnosing workspace resolution, but it is not needed for most Go
implementation tasks.

## 2. Technical Specification / Findings

Support linked subdocuments for language and concern-specific guidance. The
main document should contain the normal rules plus a concise pointer that says
when the detailed document is relevant. The detailed material can then live in
a focused document such as `docs/lang/GoWork.md`.

Subdocuments should:

- use the same copyable-doc installation and project-init mechanism as the
  parent language document;
- be addressable through the standard documentation reference syntax so agents
  can discover and read them on demand;
- include a clear “read this when” hint in the parent document;
- avoid duplicating the parent document's general language rules; and
- remain independently useful when investigating the specialized concern.

The design should account for whether `harnez init --docs golang` installs only
the parent document or the parent plus its declared subdocuments, and how
references behave when a subdocument has not been copied into a project yet.

## 3. Implementation & Verification Plan

- Define the subdocument naming and reference convention.
- Add a first concrete subdocument, likely `docs/lang/GoWork.md`, and move
  detailed Go workspace guidance out of `Go.md`.
- Keep `Go.md` concise and add an explicit hint for workspace/module work.
- Update `harnez init --docs` and any global-doc installation path as needed so
  declared subdocuments are available through the normal documentation flow.
- Add or update tests and smoke checks for installation, references, and
  idempotent initialization.
- Verify that ordinary Go guidance remains short while workspace-focused agents
  can reach the detailed document without relying on conversation context.
