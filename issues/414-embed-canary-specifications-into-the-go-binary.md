# 414 — Embed canary specifications into the Go binary

**Status**: Open

---

**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `scripts/canary-agenticloop-lite/fixtures.yaml`, `scripts/canary-agenticloop-lite/main.go`, `go:embed`

## Problem & Motivation

Installed Go tools currently depend on the caller's working directory to find
runtime specification files such as `scripts/canary-agenticloop-lite/fixtures.yaml`.
This makes an installed `canary-agenticloop-lite` unreliable outside the source
checkout and can silently use the wrong or stale specification.

## Scope

- Embed `scripts/canary-agenticloop-lite/fixtures.yaml` into the binary with
  `go:embed`.
- Embed every document addressable through the link mechanism, including
  linkable Markdown documents and their PNG cheat-sheet variants, so installed
  commands do not depend on source-checkout paths.
- Identify and embed other runtime specification files that installed commands
  require.
- Keep an explicit development override/path mechanism where useful, with clear
  precedence over the embedded defaults.
- Ensure embedded YAML remains the single source of truth and is not duplicated
  in Go structs or string literals.

## Acceptance Criteria

- An installed canary runs from outside the repository without a source-tree
  `fixtures.yaml` or any linkable document.
- All supported `@<file>`, `See <file>`, `@<image>`, and `See <image>` targets
  resolve from embedded assets when running installed binaries.
- Development runs can still select an explicit YAML file for rapid iteration.
- Missing, malformed, or overridden specs produce clear errors.
- Tests cover embedded loading, override precedence, and installed-style use.
- Relevant repository checks pass.
