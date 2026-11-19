# 944 — quota-1: count targeted and coverage go test runs toward the one-run budget

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 896; gokay issue 044; gokay docs/studies/2026-10-25-two-agent-lean-sprint-gokay-skills.md

---

## 1. Problem & Motivation
gokay 044 (developer `claude:haiku:high`, 2026-10-25) ran `go test -run ... .` before `make test-q1` and `go test -coverprofile .` after it. The q1 run passed and nothing was changed between the runs. That makes three test runs under a one-run budget, and harnez neither counted nor blocked the two extra ones.

Ticket 896 did not cause this. Its exception covers only "after fixing a failure from that run", and here there was neither a failure nor a fix. The gap predates 896:
- `.harnez/rules/Quota.md` says "run the test suite once per step/turn", but never defines whether a targeted or coverage `go test` counts as that run.
- `detectQuota1` (`internal/exec/exec.go:142-159`) only activates on `--quota-1`, `HARNEZ_QUOTA1=1` or a matching env prefix. Plain `go test` is never counted or blocked.

Second opinion (`codex:sol:low`, read-only): the developer did not comply, because the "modify source before running again" line already forbids the coverage run. The texts also leave "test suite" ambiguous for targeted plain `go test`.

## 2. Technical Specification / Findings
Proposed Quota.md wording (from sol, host-agreed): "Every test execution, including targeted `go test` and coverage runs, counts toward the one-run allowance and must use the quota wrapper. Further runs require an intervening source modification; under lean-sprint §3, after fixing a failure from that run, the developer may rerun only touched packages and must report the full run owed to the host."

Coverage should come from the q1 run itself (e.g. a `-coverprofile` passed through `make test-q1` or `harnez exec --quota-1`), not from a separate run.

## 3. Implementation & Verification Plan
- M1: update the Quota.md source, the lean-sprint and lean-sprint-v1 §3 wording, and run `harnez init` so the installed rules show the change.
- M2 (enforcement): count plain `go test` / `gotestsum` runs by agents against the quota-1 state, for example through the Bash hook, and block a repeat without a source change. The 896 touched-package rerun stays allowed after a failed run.
- Verification: a test where a passing q1 run followed by `go test -cover .` with no edit is refused, and a test where a failing q1 run, a fix, and a touched-package rerun is allowed.

## Decision

- **Docs & rule text (M1)**: Update both `config.yaml:765-772` (`quota_section`) and `config.yaml:905-914` (`Quota.md` rule source), plus `docs/commands/lean-sprint.md` §3 and `docs/commands/lean-sprint-v1.md` §2 (correcting ticket's §3 citation), keeping existing `internal/claude/quota1_init_test.go` anchor phrases (`Single-Test Boundary`, `Clean Tree First`, `make test-q1`, `QUOTA_BYPASS=1`). Regenerate `.harnez/rules/Quota.md` and synced copies via `make install && ./harnez init -d .`. (Host `/decide`, 2026-11-19.)
- **When plain `go test` / `gotestsum` triggers Quota-1 (M2)**:
  - Detect `go test` and `gotestsum` command invocations at shell-segment command position (unwrapping `bash -c`/`sh -c`, stripping leading `VAR=VAL` assignments, splitting on `;`/`&&`/`||`/`|`, and checking the executable token so arguments like `echo "go test"` or `rg "go test"` and commands like `go vet`/`go build` do not match).
  - Enforce Quota-1 on matched `go test` / `gotestsum` runs ONLY when Quota-1 is enabled in the workspace (`.harnez/repo.yaml` sets `rules.quota: true`, `.harnez/rules/Quota.md` exists, or a `quota_1.state` file already exists), and ONLY when NOT already inside an active parent `--quota-1` execution (`HARNEZ_QUOTA1_ACTIVE=1` exported into the wrapped child env by `runExecWrapper`).
  - Keep existing `QUOTA_BYPASS=1` / `HARNEZ_QUOTA_BYPASS=1` and `/disable` owner suspension working, and keep source-change gating (`snapshot.go`, which naturally allows a failing run -> source edit -> targeted `go test ./touched` rerun). (Host `/decide`, 2026-11-19.)

## M1+M2 — Docs sync and Quota-1 enforcement for `go test` / `gotestsum`

- Write set (expected): `config.yaml`, `.harnez/rules/Quota.md`, `docs/commands/lean-sprint.md`, `docs/commands/lean-sprint-v1.md` (and `AGENTS.md` / synced docs if `./harnez init -d .` updates them — revert unrelated diffs), `internal/exec/exec.go` (command-position `go test` / `gotestsum` detector + repo Quota-1 check + `HARNEZ_QUOTA1_ACTIVE=1`), `internal/exec/exec_test.go` (+ `quota1` tests).
- Locked tests:
  1. Command detector unit tests: matches `go test .`, `go test -run Foo ./pkg`, `GOWORK=off go test -coverprofile=c.out ./...`, `cd sub && gotestsum ./...`; rejects `go build ./...`, `go vet ./...`, `echo "go test"`, `rg "go test" .`, `make test-q1`.
  2. Passing `--quota-1` run followed by `go test -cover .` (and `GOWORK=off go test -run Foo .`) with NO source change in a Quota-1 repo -> refused.
  3. Plain `go test -run Foo .` first run records Quota-1 state so a repeat `make test-q1` or `go test .` with NO source change -> refused.
  4. Failing `--quota-1` run -> edit source `.go` file -> targeted `go test ./pkg` -> allowed.
  5. Recursive `HARNEZ_QUOTA1_ACTIVE=1`: `go test` inside an active `--quota-1` run is NOT blocked; non-Quota repo (`rules.quota` absent/false, no `Quota.md`, no `quota_1.state`) leaves plain `go test` unblocked.
  6. `internal/claude/quota1_init_test.go`, `rules_init_test.go`, and `TestRootDocCopiesMatchSources` pass.

## Refinements (from `rev-944` review of `46948f89` / `2921e4cd`)

1. **[MEDIUM] Separate Quota-1 counting from `bwrap` sandboxing (`internal/exec/exec.go`)**:
   - Keep `quota1.CheckAndRecord` gating + `HARNEZ_QUOTA1_ACTIVE=1` active for BOTH explicit `--quota-1`/`HARNEZ_QUOTA_1=1` AND detected `go test` / `gotestsum` (`quota1Active`), while restricting `bwrap` sandboxing (`quota1Sandbox`) strictly to explicit `--quota-1` / `HARNEZ_QUOTA_1=1` (`explicitQuota1`). Add a unit test verifying detected plain `go test` counts/gates under Quota-1 without enabling `quota1Sandboxed`.
2. **Wrapper prefixes (`internal/exec/quota1_command.go`)**:
   - Handle flag-value pairs cleanly in `skipCommandPrefix` for `env` (`-u`, `-C`, `--unset`, `--chdir`, `-S`, `--split-string`), `timeout` (`<duration>` plus flags like `-s`/`-k`), `nice` (`-n <val>`), and `time` (`-p`, `-f`), with table tests covering them.
3. **Sync `config.yaml` and `.harnez/rules/Quota.md`**:
   - Ensure `config.yaml` (`quota_section` and `files.Quota.md`) includes the `Report Untested Edits` bullet cleanly and matches `.harnez/rules/Quota.md` so `make install && ./harnez init -d .` is an exact no-op on `.harnez/rules/Quota.md`.
4. **Test all 3 `quota1Configured` triggers (`internal/exec/exec_test.go`)**:
   - Add tests verifying plain `go test` gating triggers cleanly for (a) `.harnez/repo.yaml` with `rules: { quota: true }` (no `Quota.md`), (b) `.harnez/rules/Quota.md` alone, and (c) existing `quota_1.state` alone. Also confirm `internal/claude` tests (`quota1_init_test`, `rules_init_test`) pass.
