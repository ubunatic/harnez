# 786 — make test-ubuntu cannot pass until 781/783/784 are fixed; no CI guards git 2.43

**Status**: In Progress — round 2: CI + container-test
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #780, #781, #782, #783, #784, #788

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

## 4. Resolution (2026-10-17)
- `make container-test` (`test-ubuntu` is an alias; `scripts/test-ubuntu.sh` removed)
  builds `Containerfile.test` (Ubuntu 24.04, git 2.43, Go from `go.mod`, minisign,
  python3, gcc) and runs `make check` as non-root `tester` with `--init` and
  `--network none`. Only `TEST_OUT` (default `/tmp/harnez-container-test`) is mounted;
  the log lands in `test.log`. `ENGINE ?= podman || docker`; podman runs `--userns=keep-id`
  and the image's tester UID is the caller's, so the log is owned by the caller.
- Container fixes found on the way: tests that set `HOME` to a temp dir re-downloaded
  modules (absolute `GOMODCACHE`/`GOCACHE` now); `.dockerignore` drops `.git`, so the
  image commits a snapshot repo for tests that locate the worktree.
- `.github/workflows/ci.yaml`: `actions/checkout@v7` with `lfs: true` (GitHub serves the
  17 LFS objects; `internal/bench` stops skipping the saved sessions), `actions/setup-go@v7`
  from `go.mod` with cache, `go build ./...`, `make build`, `make lint`, `make check`,
  `go test -race ./...`. `macos-hello.yaml` got the same action bumps (Node 20 deprecation);
  its trigger stays with #338.
- #781, #783, #784 do not reproduce here: minisign is installed, `--init` reaps, and the
  agent resume test passes. They stay Draft for the cloud-sandbox case.

### Tests that skip in the container
No test needs audio, a TTY, a GPU or the network: the suite passes with `--network none`
and no TTY. These skip on missing tools, so no container env flag was added:
- `TestQuota1SandboxBlocksHomeWrite` — bwrap not installed (it needs user namespaces).
- `TestReadAdaptersRuntimeCanary` — node unavailable.
- `TestDefaultLocalClassifier_LiveManual` — opt-in `RUN_LIVE_CLASSIFY_TEST=1` (live model).
- `TestForegroundWorkerHelper`, `TestDetachedLaunchHelper` — subprocess helpers, always skipped as tests.

### Verification
- `make container-test` with podman 5.7.0: exit 0 (no `--- FAIL`).
- `go test -race ./...` in the same image: exit 0, 139 s.
- `actionlint` (via `go run github.com/rhysd/actionlint/cmd/actionlint@latest`): clean.
- docker not verified: `docker` on this host is the podman shim.
- CI logs cannot be downloaded with the local `gh` token (HTTP 403, admin rights);
  step conclusions show the last success (39007748459) ran `make check` green.
