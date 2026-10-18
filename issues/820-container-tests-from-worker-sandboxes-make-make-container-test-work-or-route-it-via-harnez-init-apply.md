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
