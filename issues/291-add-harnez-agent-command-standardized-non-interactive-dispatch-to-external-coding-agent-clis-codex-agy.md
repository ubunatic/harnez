# 291 — Add `harnez agent`: standardized non-interactive dispatch to external coding-agent CLIs (codex, agy)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Enhancement
**Category**: Feature
**Related**: [288 `/harnez-agent` skill](288-add-fresh-codex-skill-lean-fresh-handoff-sprint-using-codex-cli-instead-of-a-claude-subagent.md), #479
(consumer of this command — the skill should shell out to `harnez agent`, not hand-construct raw
`codex`/`agy` invocations itself, per the `commands/harnez-sync.md` precedent of skills driving
existing CLI subcommands rather than reimplementing logic inline), `cmd/harnez/exec.go` (closest
existing precedent — a generic subprocess wrapper that proxies stdio and records telemetry; this
is the same shape specialized for known agent CLIs), `internal/codex/hooks.go` (existing Codex
integration, currently scoped to hook-trust wiring, not task dispatch)

---

## 1. Problem

288's live trials (`codex`, `agy`, and Claude Code) surfaced the same class of friction on the
tools, each in a different shape:

- **`codex`**: global flags (`-a`/`--ask-for-approval`, `-s`/`--sandbox`) must precede the `exec`
  subcommand; model ids need a full `gpt-<version>-<codename>` form (a bare codename like `sol`
  fails with an opaque provider-side error); no `--list-models` flag was found, so a caller must
  fall back to `~/.codex/config.toml` or the interactive model-picker to find valid names.
- **`agy`**: `-p`/`--print` is a value-taking flag that silently swallows the next CLI token as its
  prompt unless written as `-p="..."` — `agy -p "<prompt>" --model X` fails with the prompt
  discarded and `--model` consumed as the prompt text instead. Its
  `--dangerously-skip-permissions` flag can additionally be blocked by the *host* session's own
  permission classifier (observed inside a Claude Code session), which is friction specific to
  running one agent CLI from inside another.

Every one of these is a one-off discovery an operator (human or another agent) has to relearn each
time, and the raw commands embed real authorization decisions (sandbox/approval level) as
free-standing shell flags with no record of who chose them or why — unlike `harnez exec`, which
already wraps arbitrary shell commands with stdio proxying, exit-code preservation, and a
`call_type='shell'` telemetry row (issue 118).

Claude Code adds a separate operational finding. `claude --model sonnet
--dangerously-skip-permissions -p "..."` successfully performed a write-capable independent
review and filed issues 289/290, but remained alive for several minutes without producing a final
response. The caller must treat the process handle and repository artifacts as the source of truth,
enforce a bounded timeout, and explicitly tear down a hung process. A review may complete useful
writes before final stdout appears. `--dangerously-skip-permissions` is an explicit authorization
choice and must not be a default.

## 2. Proposal

A new `harnez agent` command family that centralizes per-tool invocation correctness and adds a
telemetry record, mirroring `harnez exec`'s shape but specialized for known coding-agent CLIs
rather than arbitrary shell commands.

### 2.1 `harnez agent run`

```sh
harnez agent run --tool <codex|agy|claude> --sandbox <policy> --approval <policy> [--model <id>] \
  --repo <dir> [--ticket <N>] --prompt-file <path> [--background] [--json-log <path>]

harnez agent run --tool agy [--model <id>] --repo <dir> [--ticket <N>] --prompt-file <path>
```

- `--tool` selects a registered adapter (`codex`, `agy`, and Claude Code as the next observed
  candidate). Each adapter (an
  `internal/<tool>` package, alongside the existing `internal/codex` hooks package but a distinct
  concern — task dispatch, not hook-trust config) owns that tool's exact flag grammar, so the
  friction in §1 is fixed once, in one place, instead of relearned per skill/session.
- `--sandbox`/`--approval` (or whatever a given adapter's tool actually calls them) are **required,
  not defaulted to the most permissive value**. `harnez agent run` should refuse to run with no
  explicit sandbox/approval choice rather than silently picking `danger-full-access`/`never` —
  this is the same "ask, don't default to the risky option" rule 288 already states for the skill,
  enforced at the CLI layer so it can't be skipped by a careless prompt.
- `--model` resolution: validate against the tool's own config/model-list mechanism where one
  exists (`~/.codex/config.toml`'s `model`, `agy models`) before dispatch; reject a bare codename
  outright with a clear error pointing at the valid form, rather than letting the subprocess fail
  with a vendor-specific opaque message.
- `--prompt-file` (not an inline string arg) — avoids shell-quoting fragility for multi-paragraph
  task prompts, and gives the telemetry row something durable to reference.
- `--background`: mirrors this repo's own "Responsive Host Orchestrator" invariant — dispatch and
  return a run handle immediately rather than blocking, for callers (skills, sessions) that must
  stay responsive. `harnez agent status <run-id>` (or a `--json-log` file the caller can tail/poll
  itself) covers checking in later. Every adapter must also have a bounded timeout and teardown
  operation for processes that stop producing progress after creating artifacts.
- Records a `call_type='agent_dispatch'` (or similarly named) telemetry row: tool, model, repo,
  ticket (if given), sandbox/approval policy actually used, start/end time, exit status. This is
  the mechanism that turns "record how well this goes" (this session's ad hoc per-trial writeup in
  issue 288) into structured, queryable data over many trials — feeds `harnez gain`/`harnez stats`
  the same way existing `shell` telemetry rows do.

### 2.2 `harnez agent review` (first-class cross-agent review mode)

288 §2b.1 found that an independent review pass from a *different* agent/model than the one that
implemented a change caught a real bug (voxi issue 094) that survived implementation, self-testing,
and a same-session human review. Worth a dedicated, explicitly-read-only mode rather than only a
prompt convention layered onto `agent run`:

```sh
harnez agent review --tool codex --model gpt-5.6-sol --repo <dir> --target <path-or-diff> \
  --prompt-file <path>
```

- Forces the most restrictive sandbox the chosen tool supports (`-s read-only` for `codex`, etc.)
  regardless of what `--sandbox` would otherwise default to for `run` — a review task never needs
  write access, and the adapter should enforce that rather than trust the caller to remember it
  (per 288 §2b.1's own note that this is a real distinction worth making explicit).
- Same telemetry row shape as `run`, tagged so review dispatches are distinguishable from
  implementation dispatches in later analysis. The row should record timeout, termination, and
  artifact-presence outcomes separately from the child process exit status.

### 2.3 `harnez agent list-tools`

Enumerate registered adapters and, where the tool exposes one, its available models (`agy models`,
`~/.codex/config.toml`'s configured default) — a discovery command so an operator/skill doesn't
have to already know a tool's own model-listing incantation.

## 3. Adapter Registry Shape

Mirror `internal/codex/hooks.go`'s clean, single-purpose-package style: one `internal/<tool>`
package per adapter, each implementing a small shared interface (build the argv for
run/review given normalized inputs, parse/validate a model string, locate the tool binary). Keep
the interface minimal — the two tools already show real per-tool divergence (flag placement,
value-vs-boolean flag semantics, model-id shape) that a heavier shared abstraction would likely
paper over incorrectly. Add adapters one at a time, from real trial data (as 288 did for `codex`
and `agy`), not speculatively for tools nobody has actually driven yet.

## 4. Non-Goals

- Not a sandboxing/approval engine of its own — `harnez agent` constructs and validates the
  arguments passed to each tool's own sandbox/approval mechanism; it does not reimplement
  sandboxing.
- Not a general subprocess-execution abstraction — `harnez exec` already covers arbitrary shell
  commands; this is specifically for the small, curated set of known coding-agent CLIs with
  adapters written for them.
- Not required before 288's skill can ship a first version — the skill could initially hand-
  construct the `codex`/`agy` invocations directly (as this session's trials did) and be migrated
  to call `harnez agent` once this lands, if sequencing makes that the faster path to a usable
  skill. Note the dependency either way in 288 so the migration isn't forgotten.

## 5. Verification

- Unit tests per adapter: argv construction from normalized inputs, especially the exact bugs 288
  documented (codex flag-before-subcommand ordering, agy's `-p=` value-flag requirement,
  bare-codename rejection, and Claude Code's model/print/permission flags).
- A live end-to-end check per docs/practices/AgenticLoop.md's "Live/Real-Environment Verification"
  note — unit tests alone would not have caught either tool's real-world quirk in §1; run at least
  one real dispatch against each adapter against a throwaway/read-only task and confirm the
  constructed invocation actually behaves as intended before considering an adapter done. For
  Claude Code, include a bounded timeout/teardown probe and verify that issue/feedback artifacts
  written before termination are retained and reported.

## Epic note (#479)

Model alias, CLI flag and dispatch normalization is now covered by #481 (unified `--name`/`--model`/`-d`, legacy compatibility) and #484 (default model). Keep this open until those land, then close as absorbed.
