# 1208 — agent start --dry-run: resolve model, role and prompt without starting an agent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 545 (backlog bundle that lists `agent start --dry-run` among lean-sprint guardrails), 1203, 1019

---

## 1. Problem & Motivation

Trial 2026-11-10: `harnez agent start --dry-run -p x` fails with `Error: unknown flag: --dry-run`.
While checking 1203 (official model names), the only way to see how `--model` resolves was a live start
with a name that fails before launch; a valid name (e.g. `gemini-4-argon:low`, scarce quota) could not
be checked without spending a real turn.

## 2. Goal

/goal `harnez agent start --dry-run` (and `agent resume --dry-run`) prints what a real start would do:
resolved `provider:model:tier`, role, session name, working dir, the provider argv, and the prompt
source, then exits 0 without launching a provider, creating a session record or touching quota; errors
are the same as a real start. Covered by tests that fail if a provider process is started. Stop and ask
the owner when output format choices need their judgment. On a loop or no progress, clear the goal and
write a failure report here.

## 3. Acceptance

- `--dry-run` with a valid model prints the resolution; with an invalid one, the same error as today.
- No provider process, no session file, no telemetry row (test with a fake provider on PATH that fails if run).
- Help and `docs/` mention the flag; the lean-sprint skill can use it to check a model before dispatch.
- 545 gets a note that its dry-run item is done here.

## 4. Notes / Unverified

- Whether resume can dry-run without reading a live session is unverified.
- Check the live start path and recent commits (971 changed stream output) before starting.

## Searcher findings (s1208, codex:luna:low)

- Start (`cmd/harnez/agent_run.go`): role :395, model :402, Codex preflight :457, session record :501,
  quota capture :389/:636, launch :680. Resume: session lookup :717, stale repair write :748, role/model
  :782/:787, preflight :803/:808, compaction :835, running state :943, launch :974.
- Argv: pure builders for claude (`internal/subagent/claude.go:54`), codex (`codex.go:124`), pi (`pi.go:113`);
  agy builds inline (`agy.go:121`, :155).
- Output precedent: release dry run (`internal/release/runner.go:56`). Fake CLI: `installFakeCLI`
  (`internal/subagent/claude_stream_test.go:49`), PATH fakes in `cmd/harnez/agent_signal_test.go:125`.

## Decision

- 2026-11-10 host (decide mode): one milestone. Start dry run returns after role/model resolution and
  argv building, before Codex preflight (:457). Resume dry run does a read-only session lookup and
  returns before stale repair (:748). Extract a pure argv builder for agy so all drivers print argv.
  Output: lines `dry-run: provider:model:tier`, `role`, `name`, `dir`, `argv` (prompt shown as
  `<prompt: N bytes from -p|-f path>`, not inline), then exit 0.

## M1 delivered: agent start/resume --dry-run (5a440c8b)

Host: owed full `make test-q1` after the developer's golden fix: 206 ok, 0 FAIL. Live dry runs for
`gemini-4-argon:low`, `claude:fable:high`, `codex:luna:low` printed provider, role, name, dir, argv with
`<prompt: 5 bytes from -p>`; `~/.harnez/agents` count unchanged (1556).

## M2 Pre-Work (reviewer r1208, codex:terra:low)

1. P2: `cmd/harnez/agent_cmds.go:77` echoes the raw prompt in an error when it starts with `--` (any mode).
   Show only its first token (the would-be flag) and the byte count, never the full prompt.
2. P2: add a dry-run test through the cobra command (start and resume): fake provider fails the test if run;
   session dir entries exactly unchanged; stored session file bytes unchanged on resume; no telemetry DB.
3. P3, host decision: restore the old agy resume argv for an empty conversation ID (`--conversation ""`)
   so real runs keep exact behaviour; a test pins both forms.
4. P3, host decision: `MkdirAll` of the session-store dir in dry run is accepted (no session data written).

## M2 Pre-Work, round 2 (host full run after the developer's post-run fixes)

1. Host `make test-q1`: 205 ok, 1 FAIL: `TestDryRunCobraLeavesStoreAndTelemetryUntouched`
   (`cmd/harnez/agent_dry_run_test.go:154`): `data/harnez/telemetry.sqlite` was created. The log shows a
   session timeline "[resume] one synchronous turn on fake:model ... [done] turn finished" in that test:
   find whether a dry run (likely resume via cobra) still reaches the turn runner or only opens telemetry
   early; fix the code, not the assertion. Then rerun only `go test ./cmd/harnez/ ./internal/subagent/`
   and report; the host runs the full suite.
