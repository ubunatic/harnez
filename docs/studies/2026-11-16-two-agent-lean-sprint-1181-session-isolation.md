---
kind: sprint
---
# Lean sprint 1181: isolate Harnez session state and constrain installation destinations (2026-11-16)

Goal: Execute M7+ of issue 1181 on branch `sprint/1181` to isolate remaining shared session state via `$TAIA_SESSION_DIR` and a launch-bound collector broker, implement a constrained host-promoter installation flow, establish the `bwrap` prerequisite contract, and verify cross-session denial under Taia.

Prior work M1–M6 (`e437a41d`..`963d0131`) already landed session-scoped locks/procs (`internal/runstate`, `internal/procs`) and private workspace staging (`scripts/stage-install.sh`, `scripts/stage-install-test.sh`).

## Agents

- Searcher `1181-searcher` (`codex:luna:low`): 6m47s, 183.8k new / 1.7M cached tokens, context 90.1k.
- Architect `1181-architect` (`codex:sol:med`): 8m50s, 97.0k new / 586.4k cached tokens, context 25.8k.

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

## Milestones

- **M7 — Freeze ownership/security contract**: pending dispatch on `sprint/1181`.
