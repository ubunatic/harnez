# 288 — Add a `/fresh-codex` skill: lean fresh-handoff sprint that dispatches to an external agent CLI instead of a same-vendor subagent

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Enhancement
**Category**: Feature
**Related**: `commands/fresh-sprint.md` (structural precedent — the lean fresh-handoff pattern this
skill adapts), `commands/harnez-sync.md` (precedent for a skill that drives an existing external
CLI rather than reimplementing logic inline), `docs/AgenticLoop.md` (5-phase/lean-handoff practice
reference)

---

## 1. Problem

`/fresh-sprint` always hands the implementation step to a same-vendor subagent (a fresh Claude
Code agent). Harnez already treats multiple coding-agent CLIs as first-class (see
`internal/codex`'s Codex hook integration), and a user may want the lean fresh-handoff pattern —
clean goal handoff, autonomous execution, confidence-gated inline review, fast teardown — but with
a **different** underlying agent CLI doing the actual implementation work, for comparison,
cost/quality tradeoffs, or simply because that CLI is already open in another terminal.

**Important framing note**: this is not "a Claude-specific skill that also happens to shell out to
Codex." Skills in this repo are agent-agnostic instructions — any coding agent capable of running
shell commands and following a markdown playbook can execute `/fresh-codex`, not only Claude Code.
The skill's job is to orchestrate: dispatch the implementation subtask to the `codex` CLI as a
non-interactive, one-shot subprocess, then apply the same self-verification and inline-review
discipline `/fresh-sprint` already uses, regardless of which agent is running the orchestration
itself.

## 2. Prior Art / Experiment (2026-09-09, voxi repo)

A live trial was run in `~/projects/voxi` implementing issue 093 (a small, well-scoped ASR
post-processing fix) via:

```sh
codex -a never -s danger-full-access exec "<self-contained task prompt>"
```

Findings worth carrying into the skill:

- **Invocation shape**: global flags (`-a`/`--ask-for-approval`, `-s`/`--sandbox`) go *before* the
  `exec` subcommand. `-a never` means fully autonomous (no approval prompts); `-s
  danger-full-access` disables the sandbox entirely. Both are real authorization decisions the
  *user* must make explicitly per invocation — never default a skill to `danger-full-access`
  without the user having chosen it for that run.
- **Model naming is easy to get wrong**: model ids on this Codex account carry a vendor-style
  generation+codename form, e.g. `gpt-5.6-luna`, `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-6-astra`,
  `gpt-5.5`. A bare codename (e.g. `sol`) is **not** a valid `-m` value and fails with "Model
  metadata not found" / "not supported when using Codex with a ChatGPT account" — the full
  `gpt-<version>-<codename>` string is required. `codex` has no `--list-models` flag discovered so
  far; the reliable way to enumerate valid names is triggering the interactive model-select menu
  (seen once, unprompted, mid-session) or reading `~/.codex/config.toml`'s existing `model =`
  value. The skill should tell the operator to confirm the exact model string with the user (or
  read it from `~/.codex/config.toml`'s `model`/`model_reasoning_effort` defaults) rather than
  guessing a codename.
- **`--json` streams JSONL events** (`thread.started`, `item.completed`, `turn.completed`, etc.) —
  useful for a background-dispatch pattern (redirect to a file, poll/notify on `turn.completed`)
  mirroring how this repo already handles Claude subagent dispatch.
- **Self-report vs. verification**: the codex run's own final summary claimed tests passed; the
  orchestrating session re-ran `go vet ./...` and `go test ./...` independently rather than trusting
  the self-report, per this repo's own "Silent Verification" anti-pattern in `docs/AgenticLoop.md`.
  This must not be skipped just because the subprocess is a different vendor's agent — if anything
  it's more important, since there is no shared reasoning trace to sanity-check.
- **Result quality**: for this one small, well-scoped ticket, the diff was accurate, correctly
  scoped, matched the ticket's own anchoring/false-positive-avoidance requirement, and included
  reasonable positive/negative unit tests. One data point, not a broad claim about Codex's general
  reliability — the skill's review step should stay mandatory regardless.

## 3. Proposed Skill Shape

Adapt `commands/fresh-sprint.md`'s five steps, replacing step 1-2 (dispatch to a fresh subagent)
with a `codex exec` subprocess call:

1. **Clean Goal Handoff** — same as `/fresh-sprint`: a self-contained prompt (the target CLI has no
   shared context) naming the ticket, target files, acceptance criteria, and explicit
   verification commands to run before reporting done. Must also tell codex which repo
   conventions doc to read (this repo's `AGENTS.md`/`CLAUDE.md` equivalent) since it has no access
   to the orchestrating agent's system prompt.
2. **Non-interactive dispatch** — construct the `codex -a <policy> -s <sandbox> exec -C <repo>
   "<prompt>"` invocation. Never default `-a`/`-s` to the least-restrictive values
   (`never`/`danger-full-access`) without the user having explicitly chosen them for this
   invocation — ask if not already established in the current conversation. Resolve the model
   string against `~/.codex/config.toml` or an explicit user-given value; never guess a bare
   codename.
3. **Autonomous execution** — run in the background (mirroring "Stay Responsive" in
   `/fresh-sprint`); do not block the host turn on a long-running `codex exec` call unless the
   user asked to wait.
4. **Confidence-gated inline review** — mandatory independent re-verification (repo-native test/
   build commands) regardless of what codex's own final message claims. Read the actual diff.
   Escalate to a full reviewer pass on cross-subsystem blast radius or codex-reported uncertainty,
   same thresholds as `/fresh-sprint`.
5. **Fast teardown & status sync** — same as `/fresh-sprint`: update ticket status, run
   `harnez index`, no lingering background processes.

## 4. Non-Goals

- Not a general-purpose "call any external agent CLI" abstraction — scope this to Codex CLI
  specifically for now; generalize only if a second CLI integration is actually needed later.
- Not a replacement for `/fresh-sprint` — both should coexist as alternative lean-handoff targets.
