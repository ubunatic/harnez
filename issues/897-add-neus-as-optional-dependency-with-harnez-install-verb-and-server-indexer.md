# 897 — Add neus as optional dependency with harnez install verb and server indexer

**Status**: Closed — M1 install verb, M2 opt-in indexing, M3 fixes verified by TestNeusIndexerBackgroundChildHoldingStdoutDoesNotBlock and TestUnknownDependencyAndDryRunWithoutTools; daemon persistence parked in 913
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

## Milestone M3 — Pre-Work / Required Refinements (opus review of df568bec, b363b288)

M1 (install verb) and M2 (opt-in server indexing) delivered. Review verdict: ship. Fix first:

1. `internal/server/neus_index.go:72`: stdout/stderr set to `io.Discard` create copy goroutines; a neus child
   holding the pipes can hang `cmd.Run()` after the timeout kill. Leave them `nil` or set `cmd.WaitDelay`;
   add a test with a fake neus that backgrounds a sleeper holding stdout.
2. `cmd/harnez/server_cmd.go:492`: replace the variadic `indexing ...server.NeusIndexConfig` with a plain
   parameter; load the spec once.
3. `internal/server/server.go:176-181`: comment why returning on service-wait timeout leaks nothing.
4. `internal/server/neus_index.go:35,77`: handle or comment the ignored `time.ParseDuration` errors; same for
   the never-failing `TryLock` at :61.
5. `internal/dependency/install_test.go`: the "dry" case and the second dry run in `TestUnknownDependency…`
   must assert the printed commands / error output.

Parked: daemon cannot index because `server install` does not persist the flags → 913.
