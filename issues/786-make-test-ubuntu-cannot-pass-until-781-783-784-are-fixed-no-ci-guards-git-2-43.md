# 786 — make test-ubuntu cannot pass until 781/783/784 are fixed; no CI guards git 2.43

**Status**: In Progress — round 2: CI + container-test
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #780, #781, #782, #783, #784

---

## 1. Problem & Motivation
`make test-ubuntu` (added for #782) runs the full `go test ./...` in
`ubuntu:24.04`. It fails there for known, unrelated reasons (no minisign
#781, no reaping init #783, agent resume #784), so it cannot show that the
git 2.43 fix holds. Nothing runs it automatically either, so a new
git-version-sensitive flag can regress unnoticed.

## 2. Technical Specification / Findings
- `scripts/test-ubuntu.sh` runs as root without `--init`; #783 tests need a
  reaping init (`podman/docker run --init`).
- The image has no minisign (#781).
- Only CI is `.github/workflows/macos-hello.yaml` on the GitHub mirror.
- `#782` was verified locally on git 2.43.0: `go test ./internal/skillreg/` passes.

## 3. Implementation & Verification Plan
- Pass `--init` in `scripts/test-ubuntu.sh`; install minisign in the container
  or let the script accept `PKGS=` (default `./...`) to narrow the run.
- Once #781/#783/#784 are fixed, add a Linux workflow that runs
  `make test-ubuntu` (or `go test ./...` on `ubuntu-24.04`) on the mirror.
- Verify: `make test-ubuntu` exits 0 with podman and with docker.
