# 1254 — Shelve non-Go repo code; port remaining tooling to Go

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: 444, 646, 865, 1047, 1245

---

## 1. Problem & Motivation

The owner wants as little non-Go code in the repo as possible (2026-11-21). An inventory found
Python, shell, TypeScript and `.mjs` files, most of which no test, Make target or CI job runs.
Two compiled Go canary binaries (3 MB each) were committed at the repo root by mistake.

## 2. Technical Specification / Findings

Inventory (2026-11-21), by how each file is exercised:

- Product, must stay: the Pi extensions (`.pi/extensions/background-tasks.ts`,
  `.pi/lib/*.ts`, `internal/subagent/tool-call-limit.ts`) run inside Pi, which loads TypeScript;
  `internal/claude/jevcompaction/hook.ts.tmpl` is a Claude Code function hook
  (`docs/ClaudeFunctionHooks.md`); the generated `env.sh`/`env.fish` are user
  shell integration. `test/pi/*` tests the Pi extensions (`make test-pi`, CI).
- Repo tooling with an automated caller (kept for now, port candidates for M2/M3):
  `scripts/lint.sh` (`make lint`), `scripts/test-pi.sh`, `scripts/stage-install.sh` and
  `stage-install-test.sh`, `scripts/smoke-test.sh`, `scripts/drop-perm.sh`, `scripts/macos-ci.sh`,
  `scripts/install-canary.sh`, `scripts/install-dev-deps.sh` (CI), `scripts/git-hooks/pre-push`
  (tested in `internal/githooks`), `scripts/agent-canary/*` with the Containerfiles
  (`make smoke-container`), and the cloud-agent setup scripts `scripts/setup-env.sh` and
  `scripts/jules-setup.sh`.
- Shelved in M1: everything with no automated caller, listed in section 4.

Shelving means removing from the tree; git history keeps the files. Restore one with
`git checkout <shelving-commit>^ -- <path>`. Find the commit with
`git log --diff-filter=D --format=%h -1 -- <path>`.

## 3. Implementation & Verification Plan

- M1: remove the unexercised non-Go files and the committed binaries, ignore root canary
  binaries, and repoint docs and comments. Verify: `make lint`, the Go tests that read docs
  (`internal/claude`, `internal/lint`, `internal/skillreg`, `internal/assess`), and `git grep`
  for each removed path outside `issues/` and `docs/studies/`.
- M2: port `scripts/lint.sh` (docs/commands registration) to a Go test or `harnez lint --check`
  and drop its dead `contrib/*/*.js` node check (no `contrib/` exists).
- M3: port `scripts/smoke-test.sh` (with `drop-perm.sh`) and `scripts/stage-install-test.sh` to Go
  tests in the full tier, one at a time. Owner rule (2026-11-21): keep Makefile recipes thin; do
  not move script logic into the Makefile. A few small shell scripts are fine: `test-pi.sh`,
  `stage-install.sh`, the pre-push hook, the container launchers and the environment bootstrap
  scripts (`install-dev-deps.sh`, `setup-env.sh`, `jules-setup.sh`, `macos-ci.sh`) stay shell.

## 4. M1 Shelved

- Root binaries: `canary-remote-load-stream`, `canary-watch-pty` (Go builds; the sources stay in
  `scripts/canary-*/main.go`). `.gitignore` now ignores `/canary-*` binaries at the root.
- Python: `scripts/analyze_usage_timeseries.py`, `scripts/canary-823-agy-compact.py`,
  `scripts/canary-865-pam-fingerprint.py` (865 is Backlog), `scripts/md-to-braille8.py` (Dot8 is
  on hold, 444; `docs/proposed/BrailleCards.md` already plans a Go port).
- Shell canaries and benches: `scripts/benchmark-read-savings.sh`, `canary-740-channel.sh`,
  `canary-clean-workspace-docs.sh`, `canary-usage.sh`, `statusline-bench.sh`, `devin-sandbox.sh`,
  `canary-gnome-mic-indicator.sh`, `test-canary-gnome-mic-indicator.sh`,
  `canary-646-claude-compact.sh`, `canary-735-skill-hooks.sh`, and the shell originals
  `canary-remote-load-stream.sh` and `canary-watch-pty.sh` (superseded by their Go ports).
- Canary trees: `canary/646-session-compact/` (646 is Backlog), `scripts/canary-pi-capture/`,
  `scripts/canary-pi-presentation/`, `scripts/canary-pi-compaction/` (1047 is Backlog) with the
  `make canary-pi-compaction` target, and `reels/` (Wayreel demo scenarios).
