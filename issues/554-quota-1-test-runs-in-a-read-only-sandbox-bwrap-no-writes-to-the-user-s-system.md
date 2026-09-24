# 554 — quota-1 test runs in a read-only sandbox (bwrap), no writes to the user's system

**Status**: Open
**Priority**: P1
**Severity**: High
**Category**: Feature / Tests
**Related**: [[553-tests-delete-real-local-bin-harnez-agy-and-harnez-shims-via-cleanall-without-home-isolation]], [[545-lean-sprint-guardrails-history-preflight-optional-luna-low-cpu-check-for-loop-wait-changes-agent-start-dry-run-host-only-ticket-close]]

---

## Problem

`make test-q1` ran tests that deleted the user's real `~/.local/bin/harnez-agy` and
`~/.harnez/shims/bash` (553, fixed by a temp HOME in internal/claude). Per-package HOME isolation
only fixes what we noticed. User rule (2026-09-24): a quota-1 test run must not change the user's
system at all; read-only access is fine.

## /goal

`harnez exec --quota-1 -- <cmd>` runs <cmd> in a sandbox where the whole filesystem is read-only
except the repo, the build caches it needs (Go: GOCACHE), and a private /tmp. Applies to every
project, since `make test-q1` is scaffolded by `harnez init`. harnez's own quota/telemetry state is
written by the outer `harnez exec`, outside the sandbox. Without bwrap: fall back with a warning
(temp HOME at least).

## Canary (host, 2026-09-24)

- `bwrap --ro-bind / / --dev /dev --proc /proc --tmpfs /tmp --bind $PWD $PWD --bind $GOCACHE $GOCACHE`
  around `GOWORK=off go test ./...`: writes to ~/.harnez are refused; suite runs in ~32 s.
- One failure, a second real leak the sandbox exposed: TestGearMulticallExecution's `harnez exec`
  writes `/run/user/1000/harnez/procs/<pid>.json` (XDG_RUNTIME_DIR). Tests must set their own
  runtime dir; the sandbox could also mount a tmpfs there.

## Open questions

- Other projects' tests may write caches (~/.cache, cargo, npm). Decide: extra writable binds via
  config, or a temp HOME inside the sandbox.
- Network stays on (bwrap default); module downloads need GOMODCACHE writable or GOPROXY=off.

## Decision (user, 2026-09-24)

- No sandbox HOME for now. Real HOME stays mounted read-only; GOCACHE and GOMODCACHE are bound
  writable, so the Go cache stays shared across all projects.
- Any other write fails loudly (read-only file system); handle case by case (fix the test or add a
  writable bind). A shared temp home is deferred until a real case needs it.

## M1 delivered (53a483f), host review

- Installed harnez: `touch ~/.x` inside quota-1 fails (read-only), repo writes work. Full `make test`
  inside the same bwrap args: green.
- Process note: the developer used the quota-1 bypass for a manual check, which agents must not do.
