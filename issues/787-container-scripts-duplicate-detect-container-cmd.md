# 787 — container scripts duplicate detect_container_cmd

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #782

---

## 1. Problem & Motivation
`detect_container_cmd` is copied verbatim into `scripts/install-canary.sh`
and `scripts/test-ubuntu.sh`. Copies drift; a fix in one misses the other.

## 2. Technical Specification / Findings
- `scripts/agent-canary/run.sh` must stay podman-only: docker has no `pod`
  or `volume exists`. It now fails fast when `CONTAINER_CMD` is not podman.
- No `scripts/lib/` exists yet.

## 3. Implementation & Verification Plan
- Move `detect_container_cmd` to `scripts/lib/container.sh`; `source` it via
  `"$(dirname "$0")/lib/container.sh"` in both scripts.
- Keep Bash.md style (`if test`, `then` on its own line).
- Verify: `bash -n` on all three scripts; `CONTAINER_CMD=docker make install-canary`
  selects docker; unset with only podman installed selects podman.
