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

## Milestones

- **M7 — Freeze ownership/security contract**: committed `f87b48db` on `sprint/1181`; `HTO=0 make test-q1` and `go test -v ./internal/sessioncontract` passed. `1181-m7-review` (`codex:sol:low`) flagged 3 major gaps (`livefetchcache` owner mismatch, weak JSON schema uniqueness/`minItems`, `docs/Spec.md` literal duplication in `spec.go`), all embedded as M8 pre-work.
- **M8 — Pre-work & centralize runner-local state**: committed `ad1cb455` on `sprint/1181` after host-verified `HTO=0 make test-q1` exit 0 (`internal/runstate` 0.004s, `internal/sessioncontract` 0.017s, `internal/usage` 7.922s, `cmd/harnez` 61.528s, `scripts/stage-install-test.sh` pass). `1181-m8-review` flagged 3 major gaps (`livefetchcache.go:60` still writing/migrating shared XDG files, discarded errors in 6 callers including `powerPeaksPath` returning `""`, and test coverage/barrier gaps), all embedded as M9 pre-work.
- **M9a & M9b — AGY meter/backoff & `livefetchcache` isolation**: M9a (`64403936`) isolated AGY meter/bundle/log and host-only shim provisioning; M9b isolated `livefetchcache`, AGY auth backoff, and `registry.go` error reporting (`HTO=0 make test-q1` passed). Remaining M9c (error propagation + barrier/AGY tests) and M9d (auxiliary writers) dispatching next.
