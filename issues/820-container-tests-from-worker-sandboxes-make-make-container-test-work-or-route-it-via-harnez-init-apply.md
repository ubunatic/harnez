# Container tests from worker sandboxes: make make container-test work (or route it) via harnez init/apply

Status: Open
Priority: P2
Category: Feature

## Problem
Workers under the quota-1 sandbox cannot run rootless podman ("namespace cloning denied"; remote
socket also blocked), so cati 101 and loom 335 could not verify `make container-test`; the host
re-ran every container test itself (2026-10-17). Proxy env vars forwarded into containers also
broke network steps (voxi scrubbed them by hand).

## Proposal (solve generally via harnez init/apply)
- Option A: quota-1/exec allows a declared container runner (podman with user namespaces) for
  `make container-test`, configured by `harnez apply`.
- Option B: workers request a host-run container check (`harnez exec --host container-test`) that
  the host session or a daemon executes and reports back.
- `harnez init` provides a shared `container-test` Make fragment (engine detection, proxy-env
  scrub, read-only source mount, output dir) so repos stop hand-rolling it.

## Acceptance
- A developer worker can run or request `make container-test` and get a real pass/fail.
- Repos initialised by harnez share the same container-test fragment; tested in harnez CI.

## Decision (user, 2026-10-20)

Fragment now, broker later. This ticket delivers only the shared `container-test` Make fragment
for `harnez init` (plan M3). The host keeps running container tests for workers meanwhile.
Rejected: exposing the podman socket in the quota-1 sandbox (grants arbitrary execution as the
user). Deferred: the scoped host container-check broker (plan M1+M2) moves to epic 825.

Plan findings (sol, read-only): sandbox is bwrap in cmd/harnez/exec.go:232 with masked
$XDG_RUNTIME_DIR; Ubuntu AppArmor `bwrap-userns-restrict` denies capabilities, so nested rootless
podman fails with `unshare(CLONE_NEWNS|CLONE_NEWUSER) = EPERM`. Current repos: cati copies source,
read-only root, /out rw; loom copies source, /out rw,Z, writable root; voxi /src:ro, /output rw, label
disabled, only one scrubbing proxies.

## Milestone: shared fragment

- `docs/templates/ContainerTest.mk` (or the init template location the repo uses), installed by
  `harnez init` reconciliation (internal/claude/init_phases.go), preserving existing project targets.
- Engine detection (podman, docker), copied-source or read-only-source mode, output dir ownership,
  SELinux label handling, project-specific check command variable.
- Scrub HTTP(S)_PROXY, http(s)_proxy, ALL_PROXY, NO_PROXY (both cases) at build and run; explicit opt-in.
- Clear diagnostic when run inside the quota-1 sandbox: "container tests need the host; ask the host".
- Tests: fake-engine argv/env, paths with spaces, idempotent init, conflict with an existing target,
  failure propagation. Docs: docs/Containerfile.md (+ copies).
