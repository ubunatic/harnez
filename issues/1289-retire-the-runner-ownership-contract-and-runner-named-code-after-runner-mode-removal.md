# 1289 — Retire the runner ownership contract and runner-named code after runner-mode removal

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: 1181, 859d787b, 1250, 1270, 1279

---

## 1. Problem & Motivation

Commit `859d787b` removed runner mode: harnez no longer reads `TAIA_SESSION_DIR` and never branches on
its caller. The ownership contract from issue 1181 was not removed with it. `spec/session_isolation.yaml`
and its schema still declare runner, broker and shared owners and the runner operations. The code still
loads that contract, and the names still say "runner". A reader who starts from `docs/SessionIsolation.md`
or from `runner_*` files is led back to a split that no longer exists.

## 2. Technical Specification / Findings

- `spec/session_isolation.yaml` and `spec/schemas/session_isolation.schema.json` are loaded by
  `internal/sessioncontract`, which is used by:
  - `internal/server/runner_channel.go` (runner operation allow-list),
  - `internal/exec/bwrap_contract.go` (bwrap host-failure guidance),
  - `internal/selfinstall/promote.go` (promoter binary and man roots; the promoter is still live).
- `internal/exec/bwrap_contract.go`: `ProbeBwrapSupport` has no non-test caller. Quota-1 no longer uses
  bwrap (advisory since `859d787b`), so the probe is a removal candidate, and `DiagnoseBwrapError` is
  used only by it.
- Runner-named code still exists: `internal/server/runner_channel.go`, `runner_launch.go`,
  `runner_registry.go`, `runner_usage.go`, and `cmd/harnez/telemetry_server.go`. The inherited broker
  FD (`SOCK_SEQPACKET` socketpair, `ExtraFiles`) is a transport, not a mode, and stays unless a
  replacement is decided.
- Two comments in `internal/server/runner_channel.go` (around lines 185 and 220) still say "in runner
  mode". No `TAIA_` reads remain in non-test Go code.
- `docs/SessionIsolation.md` was rewritten to the current model in the same change as this ticket's
  filing. Its "Remaining leftovers" section points here.

## 3. Implementation & Verification Plan

1. Decide per file: keep the broker transport and rename `runner_*` to a neutral name (for example
   `broker_*`), or remove it if nothing calls it. Record the decision in this ticket before editing.
2. Replace the ownership contract with a host-neutral spec, or delete it and the loader if no code needs
   it. `promote.go` needs its roots from somewhere; move them into a spec that has a live consumer.
3. Remove `ProbeBwrapSupport` and its tests if no caller remains. Keep `DiagnoseBwrapError` only if a
   live path prints its guidance.
4. Fix the two stale "runner mode" comments in the same commit.
5. Verify with `make lint`, `go test ./internal/server ./internal/exec ./internal/selfinstall ./internal/sessioncontract`
   and `make install`. Then rewrite the "Remaining leftovers" section of `docs/SessionIsolation.md`.
