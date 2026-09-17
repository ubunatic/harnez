# 413 — Manage local context-link policy through harnez init

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `AGENTS.md`, `scripts/canary-agenticloop-lite/main.go`, `harnez init`

---

## Problem & Motivation

The local context-link policy is manually maintained in the repository
`AGENTS.md` and duplicated in the canary's generated temporary `AGENTS.md`.
It should be managed by `harnez init` so new and repaired projects receive the
same behavior automatically.

The policy semantics are:

- Local `@<file>` links are read immediately.
- Local `See <file>` links are read before corresponding work begins.
- Local `@<image>` and `See <image>` links are token-efficient PNG cheat sheets
  containing the full source-document content and must be inspected as docs.

## Scope

- Put the policy in the appropriate `<!-- harnez:begin ... -->` managed block.
- Ensure `harnez init` installs and repairs it while preserving local content.
- Make canary-generated `AGENTS.md` use the same managed policy text.

## Acceptance Criteria

- Fresh and repeated `harnez init` runs produce and restore the policy.
- Canary and initialized projects have equivalent semantics.
- Existing unrelated `AGENTS.md` content is preserved.
- Relevant initialization, canary, and test checks pass.

## Verification Guidance

Run init idempotency/smoke checks and inspect a generated canary workspace for
the policy in its temporary `AGENTS.md`.
