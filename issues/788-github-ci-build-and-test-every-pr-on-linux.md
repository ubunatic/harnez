# 788 — GitHub CI: build and test every PR on Linux

**Status**: Resolved
**Priority**: P1 (High)
**Severity**: Major
**Category**: Infrastructure
**Related**: #338, #781, #783, #784, #786

---

## 1. Problem & Motivation
PRs from Jules and Claude cloud sessions now land on the GitHub mirror
(#19-#22) and get merged with no automated check. The only workflow,
`.github/workflows/macos-hello.yaml`, is `workflow_dispatch`-only. A PR that
breaks the build, `gofmt`, `go vet` or a test merges unnoticed; #782 (git 2.43
checkout) is an example a Linux CI would have caught.

## 2. Technical Specification / Findings
- Local gate is `make check`: `gofmt -l`, `GOWORK=off go vet ./...`,
  `GOWORK=off go test ./...`. `make lint` checks command docs registration.
- `go.mod` pins `go 1.26.5`; use `actions/setup-go` with `go-version-file: go.mod`.
- `scripts/install-dev-deps.sh` installs minisign (needed by `internal/release`, #781).
- Known Linux/container failures to expect: #783 (process-group kill without a
  reaping init; GitHub runners have a normal init, likely fine), #784 (agent
  resume, root cause unknown).
- GitHub is a mirror; Codeberg stays primary (CLAUDE.md "Repo Setup"). This
  workflow only guards PRs opened on GitHub; it must not push or release.
- Ubuntu 24.04 runners ship git 2.43, matching #782/#786.

## 3. Implementation & Verification Plan
1. Add `.github/workflows/ci.yaml`:
   - `on: pull_request` (all branches) and `push` to `main`.
   - `permissions: contents: read`; `concurrency` group per ref with
     `cancel-in-progress: true`.
   - One job, `runs-on: ubuntu-24.04`, `timeout-minutes: 20`.
   - Steps: `actions/checkout@v4`; `actions/setup-go@v5` (`go-version-file: go.mod`,
     module cache on); `bash scripts/install-dev-deps.sh`; `git --version`;
     `make build`; `make lint`; `make check`.
   - Set `HOME` to a temp dir for the test step if #784 turns out to be `~/.harnez` leakage.
2. Do not skip or quarantine failing tests (AgenticLoop rules). If #781, #783 or
   #784 fail on the runner, fix them in this ticket or land their fix first.
3. Do not touch `macos-hello.yaml`; #338 decides its trigger.
4. Document the workflow in `docs/Testing.md` (one line next to `make check`) and
   close #786's "no CI guards git 2.43" part.
5. Verify:
   - Open a PR with the workflow; the `ci` check runs and is green.
   - Push a throwaway commit to that PR with a `gofmt` violation; the check turns
     red; revert it.
   - Note run time in this ticket; keep the job under 10 minutes.
