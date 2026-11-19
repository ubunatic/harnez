---
kind: sprint
ticket: 771
date: 2026-11-19
branch: sprint/taia-harnez-pack
---

# Two-Agent Lean Sprint: 771 — `harnez exec` Spill-File Output Truncation & `spec/exec.yaml`

## 1. Scout & Consult Audit
- **Scout (`search-771`, `claude:haiku:med`, 53s)**: Mapped `cmd/harnez/exec_command.go`, `internal/exec/exec.go` (`runExecWrapper`), `headTailBuffer` (256 KB + 1 MB), Quota-1 logs, `HARNEZ_DISTILL_AUTOPIPE`, and `docs/HookRewritePattern.md:217-218`'s stale uncapped claim.
- **Consult (`consult-771`, `agy:neon:high`, 2m17s)**: Designed a live head-capped writer with lazy spill-file creation sharing one byte/line budget across stdout and stderr, `!term.IsTerminal` TTY skip, `--distill` & `--quota-1` integration, `ToolCall.OutputBytes` telemetry, and `spec/exec.yaml` + `spec/schemas/exec.schema.json`.

## 2. Milestones & Review
- **M1 (`fa47f5ae`)**: `dev-771` (`claude:haiku:high`, 15m38s, 4.55M tokens incl. cached) added `spec/exec.yaml` (`51200` bytes / `2000` lines), `spec/schemas/exec.schema.json`, `internal/exec/{spec,truncate}.go`, `--max-output-bytes` + env/prefix overrides (`HARNEZ_EXEC_MAX_OUTPUT_BYTES`, `HARNEZ_EXEC_TRUNCATE`), synced `docs/other/Spec.md` & `docs/Spec.md` via `./harnez init -d .`, updated `docs/HookRewritePattern.md` & `help.golden`, and passed `make lint` + `HTO=0 make test-q1`.
- **Cross-vendor Review (`rev-771`, `agy:neon:med`, 2m33s)**: Returned **FAIL** catching one high-severity state bug (`outputCap.allowance` did not short-circuit on `c.withheld`, allowing small writes after a UTF-8 rune-boundary back-off to leak into `dst` and skip the spill file) plus minor `moveFile` partial-dst cleanup and sandbox `XDG_STATE_HOME` test isolation.
- **Refinements (`99cc0430`)**: `dev-771` (`claude:haiku:high`, 4m28s, 1.26M tokens incl. cached) fixed `allowance` short-circuiting on `c.withheld`, cleaned up partial destination files in `moveFile`/`adoptQuotaLog`, isolated `XDG_STATE_HOME` in `execRunOpts`, and passed `make lint` + `HTO=0 make test-q1`.
- **Re-review (`rev-771-r2`, `agy:neon:med`, 2m47s)**: Returned **PASS**.
- **Host Verification**: `make install`, `make lint` (`0 issues.`, `lint: ok`), `HTO=0 go test -race ./internal/exec/...` (`47.909s`, `ok`), and `HTO=0 go test ./cmd/harnez/...` (`69.009s`, `ok`).

## 3. Outcomes
- Piped `harnez exec` commands now cap caller-visible stdout+stderr at `spec/exec.yaml` limits (`50.0KB` / `2000` lines), lazily spill full streams, and print `[Full output: <path>. Truncated: <n> lines shown (<limit> limit)]` while preserving interactive TTYs, full Quota-1 failure diagnostics, `--distill` caps, and `ToolCall.OutputBytes`.
