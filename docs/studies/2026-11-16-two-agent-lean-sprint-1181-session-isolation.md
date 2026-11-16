---
kind: sprint
---
# Lean sprint 1181: isolate Harnez session state and constrain installation destinations (2026-11-16)

Goal: Execute M7+ of issue 1181 on branch `sprint/1181` to isolate remaining shared session state via `$TAIA_SESSION_DIR` and a launch-bound collector broker, implement a constrained host-promoter installation flow, establish the `bwrap` prerequisite contract, and verify cross-session denial under Taia.

Prior work M1–M6 (`e437a41d`..`963d0131`) already landed session-scoped locks/procs (`internal/runstate`, `internal/procs`) and private workspace staging (`scripts/stage-install.sh`, `scripts/stage-install-test.sh`).

## Agents

- Searcher `1181-searcher` (`codex:luna:low`): 6m47s, 183.8k new / 1.7M cached tokens, context 90.1k.
- Architect `1181-architect` (`codex:sol:med`): 8m50s, 97.0k new / 586.4k cached tokens, context 25.8k.

- Reviewer `1181-m7-review` (`codex:sol:low`): M7 diff review in 57s, 27.5k new / 85.0k cached tokens, context 38.6k.

## Searcher Pass: remaining state, staging vs install, Taia/bwrap, M6 status

- Confirmed M6 hidden-file regression check (`dotglob`/`.unexpected`) is present in `scripts/stage-install-test.sh:3,116-125`.
- Mapped remaining unscoped paths (`~/.harnez/agents`, `telemetry.sqlite`, `internal/usage/*`, `internal/modelblock/store.go`, `internal/subagent/tier_overrides.go`, `internal/agymeter/meter.go`, `internal/server/spec.go`).
- Confirmed `Makefile:41-47`, `cmd/harnez/install.go`, `internal/selfinstall/install.go`, and `cmd/harnez/man.go` lack destination allowlisting and symlink/path-substitution pinning for shared installs.
- Confirmed `internal/exec/exec.go:252-288` invokes `bwrap` directly; `bwrap: Failed to make / slave: Operation not permitted` remains undiagnosed in the ticket.

## Architect Pass (`1181-architect`, `codex:sol:med`): M7–M17 plan & owner decisions

- Caught additional shared files (`livefetchcache.go:55`, `usage/agy.go:149`, `logging/logging.go:11`, `feedback/feedback.go:60`, `bench/store.go:16`, `legacyarchive.go:53`, `subagent/agy.go:48`), `telemetry.DefaultDBPath` migration side effects (`telemetry.go:30`), and collector `/shutdown` + `merge_legacy` exposure on the shared socket (`server.go:133`, `telemetry_server.go:175`).
- Owner confirmed M7 decisions (`2026-11-16`):
  - **A1 (Broker)**: Inherited connected `socketpair`/FD from trusted launcher; no admin routes on runner channel.
  - **A2 (Policy)**: Global `modelblock`/`tier_overrides` read-only for runners + runner-local overlays under `$TAIA_SESSION_DIR`; global edits host-only.
  - **A3 (Registry)**: Runners register/update/query own launch-bound record via broker; cross-session stop/resume host-only.
  - **A4 (Promoter)**: Trusted host defaults (`GOBIN`/`GOPATH/bin` + `XDG_DATA_HOME/man/man1`) with immutable copy validation and `O_DIRECTORY|O_NOFOLLOW`-pinned atomic per-file promotion.
- Milestone plan frozen as M7–M17 in `issues/1181-isolate-harnez-session-state-and-constrain-installation-destinations.md`.

- `1181-dev` (`codex:terra:low`): M7 (4m11s, 43.2k new / 888.8k cached), M8 pre-work+impl (7m48s, 59.4k new / 2.9M cached; hit 99.0k context); rated 4/5 and retired.
- Reviewer `1181-m8-review` (`codex:sol:low`): M8 diff review in 1m36s, 37.7k new / 172.7k cached tokens, context 48.4k.
- `1181-dev-m9` (`codex:terra:low`): M9a slice in 5m9s, 59.4k new / 1.9M cached, context 65.2k; rated 4/5 and retired.
- `1181-dev-m9b` (`codex:terra:low`): M9b slice in 5m15s, 62.1k new / 1.9M cached, context 71.2k; host verified fixed test fixture and retired.
- `1181-dev-m9c` (`codex:terra:low`): M9c slice in 5m47s total, 113.6k new / 2.1M cached, context 65.9k; committed `7e0d68c7` and retired.
- `1181-dev-m9d` (`codex:terra:low`): M9d slice in 6m51s total, 61.8k new / 1.8M cached, context 70.5k; failed 4 tests on missing `0700` fixtures, fixed them afterward; host verified `HTO=0 make test-q1` exit 0 (`cmd/harnez`, `bench`, `feedback`, `logging`, `runstate`, `usage` all green) and retired `1181-dev-m9d`.
- Reviewer `1181-m9-review` (`codex:sol:low`): M9 diff review (`ad1cb455..c771270d`) in 1m43s, 85.9k new / 153.2k cached, context 61.3k.
- `1181-dev-m10` (`codex:terra:med`): M10a slice (`bc1b5066`, `e05d0bf9`) across 3 turns in 8m48s total, 81.5k new / 3.6M cached, context 83.9k; retired after green `make test-q1`.
- `1181-dev-m10b1` (`codex:terra:med`): M10b-1a initial receipt/guard slice; caught and moved collector open into `internal/server`, host completed `Config.TelemetrySpoolDir()` wiring + test and verified `HTO=0 make test-q1`. Direct host implementation + `opus:low` review from M10b-1b onward.

## Milestones

- **M7 — Freeze ownership/security contract**: committed `f87b48db` on `sprint/1181`; `HTO=0 make test-q1` and `go test -v ./internal/sessioncontract` passed. `1181-m7-review` (`codex:sol:low`) flagged 3 major gaps (`livefetchcache` owner mismatch, weak JSON schema uniqueness/`minItems`, `docs/Spec.md` literal duplication in `spec.go`), all embedded as M8 pre-work.
- **M8 — Pre-work & centralize runner-local state**: committed `ad1cb455` on `sprint/1181` after host-verified `HTO=0 make test-q1` exit 0 (`internal/runstate` 0.004s, `internal/sessioncontract` 0.017s, `internal/usage` 7.922s, `cmd/harnez` 61.528s, `scripts/stage-install-test.sh` pass). `1181-m8-review` flagged 3 major gaps (`livefetchcache.go:60` still writing/migrating shared XDG files, discarded errors in 6 callers including `powerPeaksPath` returning `""`, and test coverage/barrier gaps), all embedded as M9 pre-work.
- **M9 (M9a–M9d) — Pre-work + isolate AGY & auxiliary writes**: M9a (`64403936`), M9b (`5ee6ada0`), M9c (`7e0d68c7`), and M9d (`c771270d`) landed on `sprint/1181` (`HTO=0 make test-q1` exited 0). `1181-m9-review` flagged 1 major (`agy.go` `writeAGYAuthBackoff`/`applyAGYMeterQuota` error suppression) and 3 minor (`feedback.DefaultDir` `"."` fallback, live-cache/symlink test depth & shared-dir check in concurrent test, open AGY producer-to-reader integration test), all embedded as M10 pre-work.
- **M10a — M9 review refinements**: committed `bc1b5066` + fixture fix `e05d0bf9` on `sprint/1181`; `HTO=0 make test-q1` passed (`internal/agymeter` 0.727s, `internal/usage` 7.842s).
- **M10b-1a — Telemetry receipts & runner spool path**: committed `54bbc95d` on `sprint/1181`; `1181-m10b1a-review` (`opus:low`) reported no Critical/Major findings and 5 Minor findings (remaining `d.sql` reads, multi-row rollback test, ID-less host commands, M11 read paths, receipt table growth).
- **M10b-1b — Inherited-FD runner telemetry channel & relay**: landed `373748b7` on `sprint/1181`; `1181-m10b1b-review` (`opus:low`, 5/5) caught C1 (shared-OFD `flock` across inherited subprocesses), M1 (timeout desync), M2 (replay wire byte cap mismatch), M3 (batch-wide quarantine), plus Minors (`SO_DOMAIN` check, typed `HTTPStatusError`). Replaced shared stream with `AF_UNIX` `SOCK_SEQPACKET` + kernel-atomic per-request `SCM_RIGHTS` stream handoff (`ServeRunnerBroker`/`RelayRunnerBroker`) and per-record replay results in `8cbd461f`; verified with concurrent multi-process, timeout, wire-limit, and mixed-batch tests (`HTO=0 make test-q1` exit 0, `internal/server` 0.073s, `cmd/harnez` 61.148s) and `make install`. `1181-m10b1b-fix-review` (`opus:low`, 5/5) confirmed C1/M1 resolved and flagged 6 hardening/test items.
- **M10b-2 — Launcher/Taia FD plumbing & M10b-1b hardening**: folded all 6 hardening items (`EOF wg.Wait`, `MSG_CTRUNC`, 32-stream cap + idle deadline, non-blocking `Sendmsg`, permanent-4xx `408/413/429` exclusion, `MaxFrameBytes > ReplayMaxBytes`, concurrency barrier, real `keep.json` 413 test), added `PrepareCmdRunnerBroker` (`internal/server/runner_launch.go`) across `internal/exec/exec.go`, `internal/subagent/{process,codex,interactive}.go`, and `cmd/harnez/subagent.go`, confirmed live `../taia` strips FDs > 2 so Taia runners fail closed while direct/bound `bwrap` preserves the FD (`runner_launch_test.go`), and updated `docs/SessionIsolation.md`. Verified `HTO=0 make test-q1` (`cmd/harnez` 61.149s, `internal/exec` 4.423s, `internal/server` 0.093s, `internal/subagent` 9.210s) and `make install`.
- **M11–M17**: continuing directly with `opus:low` review per milestone.
