# Container tests from worker sandboxes: make make container-test work (or route it) via harnez init/apply

Status: Closed — M2 refinements delivered in 0d9c8305d139f3933429a93b7a92ce44dd52f55e: explicit quota-1 bwrap marker, host-running fake-engine test, and documented /src copy-mode workdir contract; requested checks passed.
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

## Delivery

- Implemented only the shared fragment milestone in `801420da`: `harnez init` reconciles a
  managed `container-test` target into existing Makefiles, retaining customized project targets.
  The target supports Podman/Docker selection, copied or read-only mounted source, output mounts,
  optional SELinux labels, project check overrides, proxy scrubbing, failure propagation, and a
  quota-1 host guidance diagnostic. No broker or sandbox changes.
- Added tests for init idempotency/custom target preservation and fake-engine arguments, paths,
  proxy environment, failure status, and quota guidance. Fake-engine execution is skipped when
  the current worker sandbox blocks it; the quota diagnostic was exercised in this environment.
- Passed: `make lint`, `go test ./internal/claude`, `make test-short`, `make install`.
- Synchronized `docs/other/Containerfile.md` to the managed `docs/Containerfile.md` copy.

## M2 Pre-Work / Required Refinements (host review 2026-10-20)

1. Sandbox detection misfires on the host: `readlink /proc/1/ns/user` is empty for a normal user
   (pid 1 belongs to root), so `"" != user:[...]` is true and `make container-test` refuses on the
   host too. The fake-engine test therefore SKIPS on the host as well (verified:
   `go test ./internal/claude -run TestContainerTestTargetScrubsProxyAndPropagatesFailure -v` -> SKIP).
   Use a reliable marker set by harnez's own sandbox instead (cmd/harnez/exec.go wraps quota-1 with
   bwrap; have it export e.g. `HARNEZ_SANDBOX=quota1` inside, plus keep HARNEZ_QUOTA_1=1), and treat
   unreadable namespace links as "not sandboxed". Unit-test the detection for: host (no marker),
   quota-1 bwrap (marker), unreadable /proc/1/ns/user.
2. The fake-engine test must run (not skip) on a normal host; skip only when the marker says sandboxed.
3. In copy mode (default) `--workdir /src` assumes the Containerfile copies source to /src; document
   that contract in the fragment header and docs/Containerfile.md, or make the workdir a variable.

## M2 Delivery

- Addressed the review refinements in commit `0d9c8305d139f3933429a93b7a92ce44dd52f55e`:
  quota-1 bwrap children receive `HARNEZ_SANDBOX=quota1`; the shared Make target uses the explicit
  marker and no longer reads `/proc` namespace links. The fake-engine test now runs on a normal
  host and skips only when that marker is present. The fragment header and Containerfile docs state
  the default copy-mode `/src` workdir contract.
- Passed: targeted sandbox-marker and container-target tests, `make lint`,
  `go test ./internal/claude/ ./cmd/harnez/`, `make test-short`, and `make install`.
