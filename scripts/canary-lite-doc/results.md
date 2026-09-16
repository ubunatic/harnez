# Lite-doc behavioral canary results

Run via `go run ./scripts/canary-lite-doc <lite-doc> <task-file> <output-filename>`
(originally a bash script, rewritten in Go 2026-09-16 — see the note at the
bottom of this file). Each fixture spawns a genuinely isolated `claude -p`
session (scratch directory outside any harnez-managed project — no local
`AGENTS.md`/`CLAUDE.md` auto-injected) whose only context is the named lite
doc, gives it a coding task exercising that doc's rules, and lints the real
output file with `internal/lint` (the same rules `harnez lint` uses) as the
mechanical judge.

| doc | fixture | result |
|---|---|---|
| `docs/lang/Bash.lite.md` | `fixtures/bash-deploy-check.task.md` | PASS — 0 lint findings |
| `docs/lang/Make.lite.md` | `fixtures/make-widget.task.md` | PASS — 0 lint findings |

2026-09-15: both fixtures passed on first run. Manual spot-check of the
generated files beyond what `harnez lint` covers (directory-scoping via
`git -C`, `mktemp`+`trap` cleanup, `printf` over `echo`, no gawk extensions,
`⚙️`/`🤖` sentinel usage, `check`/`check-fast`/`test` alias relationship,
graceful `sudo` degradation) also confirmed compliance.

This superseded issue 359's manual/reasoning-based canary scoring — no custom
LLM-invocation-and-diff infrastructure was needed; `claude -p` plus the
existing `harnez lint` command was sufficient.

## 2026-09-15b — issue 363 dense dos/don'ts rewrite

Full from-scratch rewrite of both lite docs into terms/rules/code/dos-donts
form (no prose, no grammar), plus 4 new fixtures covering rules the original
2 fixtures never reached. All 6 fixtures pass with 0 lint findings.

| doc | fixture | exercises | result |
|---|---|---|---|
| `Bash.lite.md` | `bash-deploy-check.task.md` | strict mode, `if test`, `git -C`, `mktemp`+`trap` | PASS — 0 findings |
| `Bash.lite.md` | `bash-local-exitcode.task.md` (new) | `local` declare-then-assign, `source` over `.`, `return 0/1`, `command -v` | PASS — 0 findings |
| `Bash.lite.md` | `bash-log-pipeline.task.md` (new) | pipeline/condition alignment continuation, `timeout` wrapping, `do`-line loops | PASS — 0 findings |
| `Make.lite.md` | `make-widget.task.md` | help default goal, build, install degradation, check/check-fast/test | PASS — 0 findings |
| `Make.lite.md` | `make-deploy-parity.task.md` (new) | deploy/run/status/backup parity, `DRY=1` | PASS — 0 findings |
| `Make.lite.md` | `make-phony-help.task.md` (new) | single `.PHONY` sentinel line, `⚙️`/`🤖` managed-vs-manual split, self-scraping help | PASS — 0 findings |

Byte size:

| doc | full | lite (2026-09-15) | lite dense (363) | vs full | vs prev lite |
|---|---|---|---|---|---|
| Bash | 9789 | 6237 | 4767 | −51.3% | −23.6% |
| Make | 5461 | 4724 | 3998 | −26.8% | −15.4% |

No rule needed its original (less dense) wording restored — every fixture
passed on the first run of the dense rewrite. Spot-checked generated output
beyond `harnez lint`: `local result` / `result=$(…)` split emitted correctly,
aligned pipeline continuation emitted correctly, `timeout 5 curl` emitted for
the unpredictable-duration call, all four deployment targets plus `DRY=1`
emitted, single `.PHONY: ⚙️ 🤖` line with per-target sentinel prereqs emitted.

One deliberate content change, not a compression: the awk/mawk portability
appendix is no longer inlined in `Bash.lite.md`. awk is rarely written here,
so the lite doc now carries a one-line on-demand pointer to
`docs/lang/Bash.md` § "Appendix — Awk Portability" instead of paying its token
cost every session. The rules themselves are unchanged in the full doc. No
awk-compliance fixture is in scope for the lite doc for that reason.

See issue 362 for the harness's scope notes and issue 363 for this rewrite.

## 2026-09-16 — harness rewritten from bash to Go

The original `scripts/canary-lite-doc/run.sh` parsed the agent's chat reply
text for the generated file's path (`claude -p ... | tail -1`). This broke
when the model printed trailing commentary (e.g. a missing-`AGENTS.md` note)
after the path instead of before it — a nondeterministic ordering issue,
not a doc content problem. Since every fixture task already names its own
output file explicitly ("write to ./deploy-check.sh"), the fix removes the
text-parsing entirely: the Go rewrite (`scripts/canary-lite-doc/main.go`)
just checks for that expected path directly, and lints it by importing
`internal/lint` (`lint.DefaultLinter().LintBytes(...)`) instead of shelling
out to the `harnez` binary.

Re-ran all 6 existing fixtures against the rewritten harness — all PASS,
0 findings, same results as the bash version's last clean run. Two fixture
filenames were corrected in this pass (the earlier bash run's report used
guessed names `deploy-check.sh` for both Bash fixtures and `version-check.sh`
for one, which happened to still pass because `tail -1` picked up the right
path anyway; the actual task-specified names are `version-probe.sh` and
`log-summary.sh` — see the table below).

| doc | fixture | output filename | result |
|---|---|---|---|
| `Bash.lite.md` | `bash-deploy-check.task.md` | `deploy-check.sh` | PASS |
| `Bash.lite.md` | `bash-local-exitcode.task.md` | `version-probe.sh` | PASS |
| `Bash.lite.md` | `bash-log-pipeline.task.md` | `log-summary.sh` | PASS |
| `Make.lite.md` | `make-widget.task.md` | `Makefile` | PASS |
| `Make.lite.md` | `make-deploy-parity.task.md` | `Makefile` | PASS |
| `Make.lite.md` | `make-phony-help.task.md` | `Makefile` | PASS |

## 2026-09-16b — GoRelease.lite.md and IssueTracking.lite.md (NOT lint-validated)

Two new lite docs were authored in the same dense dos/don'ts style:
`docs/practices/GoRelease.lite.md` and `docs/practices/IssueTracking.lite.md`,
registered via `lite_source:` on the `gorelease:` and `issue-tracking:` entries
in `config.yaml` and added to `TestLiteDocStructuralGate`.

**These two docs are deliberately out of scope for this lint-based harness.**
`internal/lint` only understands Bash/Go/Make/Markdown *syntax* rules. It knows
nothing about release-process conventions (version.yaml, minisign `-W`,
`--continue`, GOWORK=off) or ticket-metadata conventions (Status/Priority/
Severity/Category vocabularies, `harnez find issues next` vs `harnez issues
new`). A fixture for either doc would pass `internal/lint` trivially — a
generated `Makefile`, shell script, or Markdown ticket can satisfy every Bash
and Make rule while ignoring every rule the doc actually teaches. Adding such a
fixture would report PASS without evidence, which is worse than no fixture.

Validation status for these two docs: **gates below passed; the isolated-agent
behavioral check did NOT run.**

Passed, mechanically:

- `go build ./...` — clean.
- `go test ./internal/claude/... -run TestLiteDocStructuralGate -v` — all 5
  pairs pass, including the two new ones (no full-doc `## ` section left with
  zero surviving vocabulary).
- `go test ./...` — full suite green.
- `./scripts/smoke-test.sh` — all apply/diff/clean/idempotency/drift-repair
  checks pass with both new `lite_source:` entries present in `config.yaml`.

Not run, and not claimed:

- The manual isolated-agent check (scratch dir outside any harnez project,
  `claude -p` with only the lite doc as `STYLE.md`, then human inspection of the
  answer against the full doc) planned for both docs — one ticket-writing task
  for `IssueTracking.lite.md`, one pre-release question for
  `GoRelease.lite.md`. The session's shell became unusable partway through
  (every `Bash` call exited 126 with `(eval):1: permission denied: ⚙`, i.e. the
  global `PreToolUse` hook `harnez exec hook` could not execute), so no
  `claude -p` run was possible. No substitute was faked: an in-repo subagent is
  *not* an isolated proxy, because this repo's own `CLAUDE.md` references
  `@docs/IssueTracking.md`, which auto-expands the full doc into the subagent's
  context and destroys exactly the isolation the check depends on.

Follow-up for whoever picks this up: run those two isolated-agent probes once
the shell is healthy and append the honest result here. Byte counts for the two
new lite docs were likewise not measured (`wc -c` unavailable); the full docs
are 12744 B (`GoRelease.md`) and 10563 B (`IssueTracking.md`).
