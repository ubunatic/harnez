# 1073 — Jules setup script installs harnez, loom and the tools for working the harnez way

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1071 (Jules provider), 785 (cloud session bootstrap), 780 (cloud agent compatibility)

---

/goal One bash script, pasted into the setup field of the Jules machine template, gives a fresh Jules
machine `harnez`, `loom` and the tools the repository rules expect, verified by a real Jules session that
runs them. Stop and report when blocked on an owner decision or a denied permission. Circuit breaker: on a
loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

Owner, 2026-11-01: the Jules UI lets a user set a machine template, so a single setup script can prepare
the machine. `AGENTS.md` and `.harnez/rules/` tell agents to run `harnez read`, `harnez find`,
`harnez issues` and `make test-q1`, but a fresh cloud machine has no `harnez` (785). Without them Jules
cannot follow the repository's workflow.

Deadline: Jules sessions can only be finished until 2026-11-11, so this is only worth doing before then,
and only if the Jules provider (1071) proves useful.

## 2. Findings

- Install paths: `go install ubunatic.com/harnez/cmd/harnez@latest` (README). `ubunatic.com/loom` has a
  `cmd/loom`; harnez depends on loom v0.3.10 as a library, so check that the CLI helps Jules at all.
- 785 plans `scripts/cloud-setup.sh` for all cloud agents, and `scripts/setup-env.sh` and
  `scripts/install-dev-deps.sh` already install `minisign`. Reuse them or share one script with 785 rather
  than adding a third copy.
- Unverified: the template's exact fields and limits, whether its script runs in the repository root, what
  the machine has preinstalled (Go version, `make`), and whether the result is cached between sessions.

## 3. Plan & Verification

- Write the script to `docs/Bash.md` rules: idempotent, no secrets, `$(go env GOPATH)/bin` on PATH for later
  commands, and a failure that names the tool. "More" means whatever a real session shows it lacks; 780 lists
  the test failures to expect.
- Document where to paste it next to the Jules setup in `docs/HarnezAgentArchitecture.md`.
- Verify with one real Jules session on `harnez` that runs `harnez read`, `harnez find` and `make test-q1`,
  and record the result here.
