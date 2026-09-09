# 288 — Add a `/fresh-codex` skill: lean fresh-handoff sprint that dispatches to an external agent CLI instead of a same-vendor subagent (also cover `agy`)

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

## 2b. Second CLI in scope: `agy`

The same day, two follow-on trials were run against a second external agent CLI, `agy` (a
multi-model agent — its `agy models` subcommand lists Gemini, Claude, and GPT-OSS variants). This
confirms the skill should not be Codex-specific in its core mechanism (dispatch + mandatory
independent verification), even though the concrete CLI-flag details differ per tool and each
needs its own documented invocation recipe.

### 2b.1 Review-only trial: `codex -m gpt-5.6-sol`

Before the `agy` trial, a second Codex run used the actual full model id `gpt-5.6-sol` (see §2's
model-naming note — `sol` alone fails) for a **review-only** task (no edits) against the
`CollapseRepeatedTrailingClause` function issue 093 had just landed in voxi:

```sh
codex -a never -s read-only exec -m gpt-5.6-sol "<review-only prompt, no edits requested>"
```

Note the sandbox choice: `-s read-only` instead of `-s danger-full-access`, since a review task
needs no write access — the skill should pick the least-privileged sandbox the task actually
needs, not default to the most permissive one used in a prior implementation-task trial.

This run found **two real, concrete bugs** in code an earlier Codex run (a different model,
`gpt-5.6-luna`) had written and a human had already reviewed and merged:

- **False positive**: `"Was your answer no? No"` → wrongly collapsed to `"Was your answer no?"`,
  deleting a legitimate one-word answer because it happened to match the question's final word.
  This directly violates the ticket's own core requirement (never delete non-duplicate legitimate
  content) and was missed by both the implementing agent, its own tests, and the human reviewer's
  inline check.
- **False negative (multiline call site)**: in `CleanWhisperTranscript`'s Strategy 2 (line-by-line
  filtering), each line is collapsed independently *before* being joined, so a duplicate split
  across two lines (`"Let's get started.\nget started"`) is never caught.
- Plus several lower-severity punctuation/Unicode edge cases (repeated `!`/`?` clusters, no-space
  boundaries, full-width Chinese punctuation, curly apostrophes) reported as minor false negatives,
  not correctness bugs.

**Implication for the skill**: a second, independent review pass (different agent/model than the
one that implemented the change) caught a real correctness bug that survived implementation,
self-testing, and a same-session human review. The skill's "Confidence-Gated Inline Review" step
should explicitly offer — not just allow — a cross-agent review pass as a cheap way to catch this
class of miss, especially for pattern-matching/heuristic code where false positives are easy to
construct but easy to miss when writing the positive-case tests yourself.

### 2b.2 Small task trial: `agy`

Task: check whether voxi's `issues/README.md` index was current, via `harnez index -d . --check`.

Call-signature findings:

```sh
agy --model gemini-3.8-flash-medium -p="<prompt text>"
```

- **`-p`/`--print` is a value-taking flag, not a boolean** — `agy -p "<prompt>" --model X` fails
  silently-ish: `-p` greedily consumes the very next token (`--model` in one failed attempt) as its
  prompt value, leaving the real prompt as an ignored positional argument, and `agy` errors with
  "`-p` took `--model` as its prompt" rather than running anything useful. The working form uses
  `=`-assignment (`-p="<prompt>"` or `--print="<prompt>"`) with `--model` placed *before* it on the
  command line.
- **`--dangerously-skip-permissions` was blocked outright** by the orchestrating Claude Code
  session's own auto-mode permission classifier — not an `agy`-side failure, but a real
  cross-agent friction point worth documenting: a skill that shells out to `agy` from inside a
  Claude Code session cannot rely on that flag to guarantee non-interactive execution, and should
  either omit it (accepting that `agy` may then require interactive approval for tool calls it
  wants to make) or warn the operator this flag may itself be denied depending on the host
  session's permission mode.
- No `-C`/`--cd`-equivalent flag was found in `agy --help`; the working invocation ran with the
  shell already `cd`'d into the target repo rather than relying on an `agy`-native flag.
- Result quality: correct — reported "up to date," matching an independent `harnez index -d .
  --check` run in the same repo immediately after. One trivial, low-ambiguity task; not a strong
  signal either way on `agy`'s capability for larger implementation work.

## 3. Proposed Skill Shape

Adapt `commands/fresh-sprint.md`'s five steps, replacing step 1-2 (dispatch to a fresh subagent)
with a subprocess call to whichever external CLI the operator selects (`codex` or `agy` to start):

1. **Clean Goal Handoff** — same as `/fresh-sprint`: a self-contained prompt (the target CLI has no
   shared context) naming the ticket, target files, acceptance criteria, and explicit
   verification commands to run before reporting done. Must also tell the target CLI which repo
   conventions doc to read (this repo's `AGENTS.md`/`CLAUDE.md` equivalent) since it has no access
   to the orchestrating agent's system prompt.
2. **Non-interactive dispatch** — construct the invocation for whichever CLI was chosen, using its
   own documented flag recipe (see §2 for `codex`, §2b.2 for `agy`) rather than a shared abstraction
   — the two tools' non-interactive-mode flags are not analogous (`codex`'s `-a`/`-s` sandbox/
   approval pair vs. `agy`'s `-p=`/`--print=` value-flag quirk and blocked
   `--dangerously-skip-permissions`). Never default either tool to its most permissive
   sandbox/approval mode without the user having explicitly chosen that for this invocation — ask
   if not already established in the current conversation. Resolve the model string against the
   tool's own config/model-list command (`~/.codex/config.toml`, `agy models`) or an explicit
   user-given value; never guess a bare codename (see §2's `sol` vs `gpt-5.6-sol` case).
3. **Autonomous execution** — run in the background (mirroring "Stay Responsive" in
   `/fresh-sprint`); do not block the host turn on a long-running dispatch unless the user asked to
   wait.
4. **Confidence-gated inline review** — mandatory independent re-verification (repo-native test/
   build commands) regardless of what the dispatched CLI's own final message claims. Read the
   actual diff. Per §2b.1's finding, actively *offer* a second review pass from a different
   agent/model than the one that implemented the change, not only a same-agent inline read —
   this caught a real false-positive bug a same-session human review missed. Escalate to a full
   reviewer pass on cross-subsystem blast radius or dispatched-CLI-reported uncertainty, same
   thresholds as `/fresh-sprint`.
5. **Fast teardown & status sync** — same as `/fresh-sprint`: update ticket status, run
   `harnez index`, no lingering background processes.

## 4. Non-Goals

- Not a fully generic "call any external agent CLI" abstraction on day one — cover `codex` and
  `agy` specifically (both have live trial data as of this ticket), each with its own documented
  invocation recipe; generalize the dispatch mechanism only if a third CLI integration is actually
  needed later and the two existing recipes turn out to share real structure worth factoring.
- Not a replacement for `/fresh-sprint` — all of `/fresh-sprint`, `/fresh-codex` (Codex), and an
  `agy` target should coexist as alternative lean-handoff destinations, selected explicitly by the
  operator or the user, never auto-chosen based on availability alone.
