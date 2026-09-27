# 618 — Prevent stale embedded specs when Go packages are separated from canonical specs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: loom-games Snake spec embedding regression

---

## 1. Problem & Motivation

Go's `//go:embed` only accepts files in the embedding package directory or its
descendants. A common layout puts authored YAML in `spec/` and its Go loader in
`internal/spec/`, which tempts an agent to add a second, copied YAML file solely
for embedding. A later edit to the canonical file can then rebuild successfully
while the binary silently retains its old configuration.

This happened in loom-games when changing Snake's default rendering mode. The
canonical `spec/snake.yaml` changed to `half-block`, but the embedded copy still
selected `braille`.

## 2. Technical Specification / Findings

The existing Spec guidance says to embed `spec/` but does not state the Go
package-directory constraint or provide a safe layout for it. The robust layout
is to put the loader Go files in `spec/`, allowing them to embed the authored
YAML directly. Synchronizing a duplicate during `make build` is only a fallback
and still leaves two versioned files.

## 3. Implementation & Verification Plan

/goal Make Harnez's Go spec guidance and project scaffolding prevent, or clearly
flag, duplicated embedded spec files; verify a generated or documented Go layout
embeds the canonical `spec/` YAML directly. Stop and report if the desired
prevention mechanism needs a user product decision or cannot be safely applied.

- Document the `go:embed` package-boundary constraint and the canonical
  `spec/`-package layout in the copyable Spec and Go guidance.
- Where appropriate, add a scaffold or validation check that steers generated
  Go projects away from copied specs.
- Verify the documented layout builds and a spec-only edit changes the embedded
  binary's loaded value.
