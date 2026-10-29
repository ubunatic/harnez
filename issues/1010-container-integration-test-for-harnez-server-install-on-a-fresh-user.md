# 1010 — Container integration test for `harnez server install` on a fresh user

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Testing
**Related**: 991, 782

---

/goal A repeatable podman test proves `harnez server install` works for a fresh user with no
harnez directories: the collector unit starts, stays active, and writes a first snapshot. It runs
on demand (`make` target) and is ready to run in GitHub CI later. Stop and report when blocked on a
user decision or denied permission. Circuit breaker: on a loop, a hard container/systemd blocker or
no progress, clear the goal and write a failure report here (what was tried, why it failed, resume
point).

## 1. Problem

991 fixed the `226/NAMESPACE` start failure (`ReadWritePaths=` entries now `-`-prefixed, `Install`
creates the cache/data/state dirs) and an empty-store first fetch, but only unit tests cover it.
The real path needs a systemd user manager, which a host that already has `~/.cache/harnez` cannot
reproduce. The owner prefers a container test over a manual live check.

## 2. Scope

- One script plus a `make` target (e.g. `make container-test-server`), next to `container-test`
  and `install-canary`; podman first, docker if it works the same way.
- Container with systemd as PID 1 (`podman run --systemd=always`, Ubuntu 24.04 like
  `Containerfile.test`), a fresh non-root user with lingering enabled, the locally built binary.
- As that user, with no `~/.cache/harnez`, `~/.local/share/harnez` or `~/.local/state/harnez`:
  run `harnez server install`, then assert: unit active, no `226/NAMESPACE` in its journal, the three
  directories exist, `harnez server status` shows systemd as socket owner, and the store gets a
  first snapshot (no vendor login needed: an empty snapshot counts).
- Network off where possible; no real agent accounts.

Out of scope: running it in CI now (note the needed runner flags instead), testing vendor quota
fetches.
