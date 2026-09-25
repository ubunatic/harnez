# 558 — agy-meter-run shares launch env with agent driver

Status: Open
Priority: P2
Severity: Minor
Category: Refactor

## Problem

agy is metered on two paths that share the proxy (`agymeter.RunWithEnvDir`) but build agy's
environment separately:

- `harnez-agy --meter`: the launcher shell script generated in `internal/claude/apply.go`
  (~lines 875–945) sets `ANTIGRAVITY_AGENT=1` and the PATH shim itself, then
  `exec "$harnez_bin" agy-meter-run -- "$agy_path" "$@"`.
- `harnez agent` (agy subagents): `subagent.AgyLaunchEnv` (`internal/subagent/agy.go:52`).

Nothing checks the two agree, so they can drift.

## Goal

One Go source of truth for agy's launch env on metered launches.

## M1 — agy-meter-run builds env via AgyLaunchEnv

- `cmd/harnez/agy_meter.go`: build env with `subagent.AgyLaunchEnv` (or a shared function it
  moves into) and call `agymeter.RunWithEnv`/`RunWithEnvDir` instead of `agymeter.Run`.
- Launcher `--meter` branch: stop duplicating env setup that Go now owns; just resolve
  `agy_path` and exec harnez. The non-meter branch stays pure shell (must work without harnez).
- Check the PATH shim semantics match: the launcher resolves agy on the *original* PATH and
  prepends the shim dir; make sure `AgyLaunchEnv` yields the same result, or reconcile and say which
  behaviour wins in the commit message.
- Tests: one test pinning `AgyLaunchEnv` output (ANTIGRAVITY_AGENT, PATH shim); one test that
  `agy-meter-run` uses it (e.g. inject a fake command that dumps env). Update launcher script
  tests for the slimmer `--meter` branch.
- Verify: `make test-q1` once, then commit `refactor(agy): ... (issue 558 M1)`.
