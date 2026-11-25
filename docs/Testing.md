# Testing

Harnez uses several complementary verification layers. Start with the smallest
layer that exercises the change, then broaden checks when the change crosses a
boundary or depends on the real environment. The shared Makefile tiers
(`make lint`, `test-pkg`, `test-short`, `test`, `container-test`) are defined in
[TestTiers.md](TestTiers.md); this doc covers harnez-specific layers.

- **Package tests** — Go unit and component tests live beside the code in
  `*_test.go` files. Run `go test ./...` for the full suite or
  `make check-fast` for the same fast feedback loop.
- **Integration-style tests** — Tests such as
  `internal/claude/integration_test.go`, CLI tests under `cmd/harnez/`, and
  tests using temporary files, HTTP servers, subprocess seams, or PTYs cover
  interactions across package and operating-system boundaries.

  **Isolate the real binary's state with `t.Setenv("HOME", tmpDir)`, not
  invented env vars.** A test that builds and runs the actual compiled
  `harnez` binary as a subprocess (e.g. `cmd/harnez/exec_test.go`'s
  `TestGearMulticallExecution`) cannot pass Go structs to isolate its
  DB/state path the way an in-process test does (`testExecOptions.DBPath`,
  `statsOptions.DBPath`, `logOptions.DBPath`, etc.) — the only channels are
  argv, env, and cwd/`$HOME`. `telemetry.DefaultDBPath()` and
  `resolve.DefaultStateDir()` resolve **only** from `os.UserHomeDir()`; there
  is no `HARNEZ_DB_PATH`/`HARNEZ_STATE_DIR` (or any other path-shaped)
  production override today (see issue 331/332). A test that invents such an
  env var and never verifies it took effect will silently write into the
  *developer's real* `~/.harnez/tool_catalog.sqlite` on every run — this
  happened for months before it was caught, because the polluted rows
  (project `"harnez"`, command `exec`) are indistinguishable from genuine
  usage. Use `t.Setenv("HOME", tmpDir)` for real isolation, the pattern
  `internal/claude/bash_shim_test.go` already uses, and assert against the
  DB/state path *under that overridden `$HOME`* rather than a path nothing
  reads.

  **Also clear the agent-session env.** Tests run from inside a Claude Code
  or Codex session inherit its session variables, so code paths that read
  them (e.g. the `harnez tip` nudge via `sessionTipHook`) behave differently
  than in a plain shell or CI: a test can pass in `make check` and fail when
  an agent runs it. Clear every name in `resolve.SessionEnvVars` with
  `t.Setenv(name, "")` alongside the `HOME` override (see
  `cmd/harnez/agent_test.go` `TestAgentResumePrintsReplyNotStructDump`).
- **Negative tests need a provably valid base fixture.** When invalid cases
  are built by mutating one shared "valid" fixture, first assert that the
  unmodified fixture passes. Otherwise a schema change that makes the base
  invalid turns every negative subtest green for the wrong reason (seen in
  `TestParseUsageSpecRejectsInvalidAge` when view modes changed, issue 689).
- **Doc-copy drift** — `TestRootDocCopiesMatchSources` fails when a root
  `docs/*.md` copy differs from its copyable source; see
  [LanguagePipeline.md](LanguagePipeline.md#lite-variants-and-root-copies).
- **Static checks** — `make check` runs `go vet ./...` and `go test ./...`
  with `GOWORK=off`; `make lint` checks registered command documentation.
- **GitHub Linux CI** — `.github/workflows/ci.yaml` runs on `ubuntu-24.04` for
  every PR and push to `main` (LFS checkout), executing `go build ./...`,
  `make build`, `make lint`, `make check` and `go test -race ./...`
  (guarding against git 2.43 regressions, issues 788, 786). Test output is
  `go test -json`; on failure `scripts/gotest-summary` writes the failing tests
  and their output tail to the step summary and as `::error` annotations, which
  `gh run view <id>` shows without the admin rights that log downloads need.
- **Container tests** — `make container-test` runs `make check` offline
  (`--network none`, `--init`) as a non-root user in `Containerfile.test`
  (Ubuntu 24.04, git 2.43, the same as the CI runner), with podman if installed,
  else docker (`ENGINE=`). Only the `TEST_OUT` log dir (default
  `/tmp/harnez-container-test`) is mounted from the host. `make test-ubuntu` is an alias.
- **Smoke tests** — `make smoke` (host only) runs the tests, builds the binary
  and runs `go run ./scripts/smoke`: real apply idempotency, Claude plugin
  status, PATH harnez version and usage output. Drift repair is the hermetic
  `TestApplyDiffRepairCycle`.
- **Canaries and live checks** — `scripts/canary-*.sh` and the matching Go
  programs probe external CLIs, network/SSH behavior, microphones, and PTYs.
  Use `make smoke-container` to run Pi and Codex against a local model in one
  podman pod (issue 723); set `LMCODER_CACHE_DIR` to download models to a
  scratch disk. Use `make install-canary` to verify the latest release
  installation. Use the focused canaries when a fake cannot establish that the real mechanism works.
  The canary guidance is in [other/Canary.md](other/Canary.md).
- **Manual or visual verification** — TUI, terminal, media, and desktop
  integration changes may need a real installed binary (`make install`) and a
  live observation such as `harnez usage --watch`. Tests prove invariants;
  they do not prove every terminal or desktop rendering outcome.
- **Quota-1 recorded runs** — `make test-q1` runs the test suite as `harnez exec --quota-1 -- make test`.
  Quota-1 is advisory: it records the run and, when the tree is unchanged since the last run, prints a
  repeat note and runs anyway. It never blocks and does not sandbox (`docs/SessionIsolation.md`).
  Quota-1 detects file `mtime` across the repository root, so parallel doc or ticket edits can make a
  repeat look like a change.
  Pitfall: the single run is the only evidence you get, so write the full output to a file
  (`make test-q1 > <scratch>/q1.log 2>&1`) and grep it for `--- FAIL`. Piping into `tail`
  loses the failing test's name.
- **Hermetic tests** — a test must pass in any caller's environment (agent shell, sandbox, CI, a
  session with `TMPDIR` set to a private directory). Isolate everything the code reads:
  - `t.TempDir()` follows `$TMPDIR`. Unix socket paths must fit the 107-byte `sun_path` limit, so use
    `internal/testutil/socktest` (`socktest.Listen`, `socktest.Socket`), never a raw `net.Listen("unix", …)`.
    `TestNoRawUnixListen` enforces this; its allow-list is the two production listeners.
  - Override `HOME`, `GOBIN`, `GOPATH` and `XDG_*` with `t.Setenv` under `t.TempDir()`, and snapshot the
    real host install dirs before and after (`TestInstallCommandLocalSelfInstall`). A test that
    installs must never leave a symlink in the real `~/go/bin`.
  - A fake git repository needs a `HEAD` file. `FindGitDir` ignores a bare `.git` directory (a stray
    `/home/uwe/.git` with only `info/` used to capture writes).
  - Give the agent store its own directory with `--store-dir` or `HARNEZ_STORE_DIR`; do not let a test
    write `~/.harnez/agents` or the default model access state.
  - Wait for a server with `awaitServerStart(t, started, done)`, which fails fast if the server
    exits first. An unbounded `<-started` hung a run for 10 minutes once.
  - Do not add product workarounds for a test environment (no `TMPDIR` rewrites in production, no
    `shortRuntimeDir` helper). Fix the test's assumption, or report the sandbox cause to its owner.
  - `go test` defaults to a 10 minute timeout. `cmd/harnez` takes about 2 minutes; a `-timeout 120s`
    flag on an ad-hoc run is the test's own failure, not a hang.
- **Remote-OS CI** — `make macos-ci` dispatches `.github/workflows/macos-hello.yaml`
  on the GitHub mirror's real `macos-14` runner and polls quietly for a
  PASS/FAIL result (no `gh run watch` job-tree spam — safe to call repeatedly
  from an agent). Use it for anything cross-platform-shaped where `go build`
  alone isn't evidence — see
  [MacOSPortability.md §2.7](MacOSPortability.md#27-ci-verification-macos-hello).
  `workflow_dispatch`-only for now; not yet wired into push/PR (issue 338).

Common entry points:

```sh
make test-q1     # run test suite under Quota-1 guardrail enforcement
make check       # vet plus the complete Go suite
make smoke       # build plus live smoke checks
make container-test  # make check offline in Containerfile.test (podman or docker)
make macos-ci    # trigger + quietly watch real macOS CI on the GitHub mirror
make install     # install the current binary to ~/.local/bin (harnez install)
```

For advanced cases, inspect the nearest package tests, the relevant canary, and
the Makefile recipe before adding a new test layer.
