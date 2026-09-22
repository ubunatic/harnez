# Testing

Harnez uses several complementary verification layers. Start with the smallest
layer that exercises the change, then broaden checks when the change crosses a
boundary or depends on the real environment.

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
- **Static checks** — `make check` runs `go vet ./...` and `go test ./...`
  with `GOWORK=off`; `make lint` checks registered command documentation.
- **Smoke tests** — `make smoke` runs `scripts/smoke-test.sh`, which builds the
  binary and exercises apply, idempotency, drift repair, and selected live
  output behavior.
- **Canaries and live checks** — `scripts/canary-*.sh` and the matching Go
  programs probe external CLIs, network/SSH behavior, microphones, and PTYs.
  Use `make agent-canary-static` for the deterministic container checks and
  `make install-canary` to verify the latest release installation. Use the
  focused canaries when a fake cannot establish that the real mechanism works.
  The canary guidance is in [other/Canary.md](other/Canary.md).
- **Manual or visual verification** — TUI, terminal, media, and desktop
  integration changes may need a real installed binary (`make install`) and a
  live observation such as `harnez usage --watch`. Tests prove invariants;
  they do not prove every terminal or desktop rendering outcome.
- **Quota-1 enforced checks** — `make test-q1` runs the test suite wrapped in `harnez exec --quota-1 -- make test`. Under Quota-1 guardrails, test execution is gated: running tests consecutively without modifying workspace files is blocked with a non-zero exit code to prevent tight test-retry loops. Note that Quota-1 detects file `mtime` across the repository root; parallel doc/ticket edits in a shared workspace will update repo timestamps and satisfy the check.
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
make macos-ci    # trigger + quietly watch real macOS CI on the GitHub mirror
make install     # install the current binary to ~/go/bin
```

For advanced cases, inspect the nearest package tests, the relevant canary, and
the Makefile recipe before adding a new test layer.
