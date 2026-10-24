# 897 — Add neus as optional dependency with harnez install verb and server indexer

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation

`neus` provides neural code search, Go AST symbol extraction, and pure-Go YAML spec indexing for local repositories. `harnez` is the developer harness and consumer of search finders (`harnez find code|docs`). Today, installing Neus requires manual Go installation steps, and indexing relies on separate manual CLI invocations.

Because `harnez server` is a daemon that routinely runs while a user is working online, it is the natural host to orchestrate background repository indexing. Furthermore, adding `neus` as an optional dependency and allowing users to install and manage it via a Harnez verb simplifies developer onboarding across the ecosystem.

## 2. Technical Specification & Discovery

- **Discovery Questions**:
  - Should Neus be installed via `harnez install neus` or integrated into `harnez server bootstrap`?
  - How can `harnez server` invoke the Neus indexing engine via Go API rather than shelling out to CLI binaries?
  - **Service Coordination & No-Conflict Invariant**:
    - Neus is also designing an independent background auto-indexer daemon / systemd user service (`neus issue 057`).
    - If a user runs `harnez server` AND `neus-indexer.service`, how do we prevent duplicate file scans and lock contention on `~/.cache/neus/index.db`?
    - Implement cooperative lock or mutual exclusion: either `harnez server` claims the indexing responsibility if running, or Neus exposes a shared socket/protocol that both share.
- **Harnez CLI Verb**:
  - `harnez install neus`: Installs `neus` binary via `go install ubunatic.com/neus/cmd/neus@latest` and runs `neus install` for initial bootstrap.
  - Expose finder configuration in `harnez` so `harnez find` delegates to `neus search` seamlessly.

## 3. Implementation & Verification Plan

```markdown
/goal Add neus as an optional dependency in harnez, support harnez install neus, and integrate neus indexing into harnez server with clean service coordination, or stop and report when blocked on a user decision or denied permission.
```

- **Discovery & Models**:
  - **Discovery**: `haiku:med` / `luna` (inspect `harnez server` daemon lifecycle and `harnez find` finder registration).
  - **Advisor**: Harnez advisor evaluation.
  - **Developer**: `sol:med`
  - **Reviewer**: `sol:med` or `opus:med` + Antigravity orchestrator review.
- **Verification**:
  - Run `harnez install neus` in a test environment.
  - Verify `harnez server` performs background indexing of active repositories without locking conflicts.
