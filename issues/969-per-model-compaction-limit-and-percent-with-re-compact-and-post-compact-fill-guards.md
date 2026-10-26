# 969 — Per-model compaction limit and percent, with re-compact and post-compact fill guards

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 180 (no PostCompact hook), 591 (default auto-compact threshold), 592 (runtime token watchdog), 869 (agy per-session flag, Claude learnings), 956 (Claude auto-compact env canary, parked), 968 (context overhead)

---

## 1. Problem & Motivation

Harnez-enabled Claude sessions reach 50k+ tokens quickly; about 33k of that is fixed overhead before any work (see 968).
Measured on 2026-10-26 in an interactive Haiku 5.5 session: `/context` shows a 33k "Autocompact buffer", so with a 100k
window Claude compacts at 100k − 33k = 67k (67%), not at the intended 60%. Haiku input above 100k tokens is reported to
cost 5x more (verified in R2), so compaction must start reliably below that step, at a point set per model.

Three gaps:

1. **No per-model percent.** `spec/agent.yaml` has one token value (`compact_threshold_tokens: 200000`), and
   `~/.harnez/config.yaml` `agent.compact_thresholds` can override it per `provider:model[:tier]`, but only as an
   absolute token count. Needed: a limit and a percent of it, for example Haiku at 100k and 80%, so Haiku compacts at
   80k. Claude's own trigger (window minus buffer) is separate and was not proven by the 956 canary.
2. **No guard against compacting again too soon.** Haiku reaches about 80k and compacts to about 70k; the next trigger
   is then only about 10k away, and each compaction costs a full summary turn for little room gained.
3. **No guard when compaction leaves the context full.** At 70% or more after a compaction, the next turns start near
   the trigger again. An orchestrated agent should stop and warn its orchestrator; a user-driven session should show
   the user a hook message.

## 2. Technical Specification / Findings

- Claude's own trigger: auto-compact window minus autocompact buffer (869 Learnings). `~/.claude/settings.json` sets
  `autoCompactWindow` to 200000; the 2026-10-26 session's 100k window came from the owner's launch flag
  `--autocompact 100000` (owner, 2026-10-27). Full formula: section 2a.
- Harnez's trigger: `compact_threshold_tokens` / `agent.compact_thresholds`, sent as `/compact` by `harnez agent`
  (591, `internal/subagent/compact.go`). Separate from Claude's trigger.
- Claude Code has a PreCompact hook (can block, matchers `manual`/`auto`). 180 recorded no PostCompact hook; current
  docs list one, side-effect only (https://code.claude.com/docs/en/hooks#precompact). Unverified on 2.1.311 (R4).
  In `-p` stream-json, the `compact_boundary` event carries `pre_tokens` and `post_tokens` (section 2a), but
  `post_tokens` leaves out system prompt and tools, so the next turn's usage stays the measure.
- Context measure: last-turn full input tokens (as in 591). Confirmed against `/context` in R3.
- Proposed config, per model in `spec/agent.yaml` with a schema (no Go defaults, per `docs/Spec.md`):
  - `limit_tokens`: reference size, for example 100000 for Haiku.
  - `percent`: share of the limit that triggers compaction, for example 80 (80k for Haiku).
  - `min_gap_percent`: minimum room, in percentage points of the limit, between the post-compact size and the
    trigger; percentage points rather than tokens so it scales per model. Starting value 20 (20k room for Haiku).
  - `max_after_fill_percent`: post-compact fill at or above which harnez stops or warns, for example 70.
- How the two guards relate: a compaction leaves size A; the room to the next trigger is `percent − A`. With
  `percent` 80 and `max_after_fill_percent` 70, the post-compact guard already fires whenever the room is 10 points
  or less. The re-compact guard only adds behaviour if `min_gap_percent` is larger than `percent − max_after_fill_percent`,
  and only if its action differs from the post-compact guard's (R1 decides).
- User-driven sessions get a message only, no pause: the user is present and picks the next step (`/clear`,
  `/compact` with a focus, or continue). Which hook carries the message is R4.

## 2a. Findings 2026-10-27 (Claude Code 2.1.311)

Full write-up and sources: `docs/AutoCompaction.md`.

- **Formula** (inferred; fits all 9 owner readings within rounding, interactive Haiku/Sonnet/Opus; an alternative with
  a fixed 20k inside the percent term fits equally well, see the doc):
  `reserve = CLAUDE_CODE_MAX_OUTPUT_TOKENS (default 20k)`, `usable = window − reserve`,
  `trigger = min(usable − 13k, PCT_OVERRIDE × usable)`. `/context` "Autocompact buffer" = reserve + 13k (33k default;
  23k with `CLAUDE_CODE_MAX_OUTPUT_TOKENS=10000`). `/context` free space ignores the percent; only the status line
  ("N% until auto-compact") reflects it, and it shows no hint above about 20% left.
  - Window 100k: percent 100/90/80/70 → 17/17/13/1% left at 56k; 50 → compacted at once.
  - Window 120k: percent 70 → 20% left at 56k (trigger 70k); percent 80 → no hint (predicted 80k, not yet observed).
- **R2 done:** Haiku 5.5 input above 100k tokens costs 5x ($0.10 → $0.50 per MTok, output/cache also 5x), per request
  (https://platform.claude.com/docs/en/models/haiku-5-5/overview). No step found for Sonnet/Opus/Fable 5.5. Owner uses
  Gemini Flash 3.8 and Gemini 4 Argon (AI Pro plan): no step on the Gemini API pricing page; owner decision: 200k.
- **R3, `-p` canary** (`claude -p --model haiku --autocompact 120000`, `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE=80`, four
  resumed turns, cost about $0.27 incl. one refused attempt): turn 2 at 85.9k input did **not** compact (percent
  predicts 80k); turn 3 auto-compacted, `compact_boundary` `trigger: auto`, `pre_tokens: 89280`, `post_tokens: 7186`,
  9.4s. Second canary (review 2026-10-27, sol:med): percent 50, resumed turn at 85.9k, no compaction (trigger would
  be 50k). Conclusion (two runs): `-p` honours `--autocompact` and the buffer line (87k) but **not** the percent
  override. 956's negative result used the env window and one turn.
- **`post_tokens` ≠ next request size:** 7,186 after compaction, next request 20,428 (system prompt, tools). The
  post-compact guard (step 4) must use the next request's input or add that overhead.
- **Haiku setting that follows** (predicted, not observed at the point; targets are owner decisions): interactive: `--autocompact 120000` + percent 80 → 80k (unobserved point);
  `-p`/`harnez agent`: `--autocompact 113000` → 80k via the buffer line, or harnez's own pre-prompt `/compact` at 80k.
- **Other tools** (luna:med web research, unverified locally): Codex `model_auto_compact_token_limit`, default and cap
  90% of window; agy has only manual `/compact`, no documented threshold. Details in the doc.

## 3. Research Tasks

Each task is done when its output is written into this ticket. The guards (section 4, steps 3–5) are not built before
R1–R4 are done.

- **R1. Does `/compact` need headroom?**
  - Method: in a Haiku canary, fill the context to about 95% of the window, run `/compact`, and read the compact turn's
    usage from the transcript. Repeat at about 60% for comparison.
  - Output: whether `/compact` at 95% succeeds or fails, the input tokens the compact turn sends, and whether those
    tokens are billed at the normal input rate.
  - Unblocks: the re-compact guard's action. If the compact turn sends the full context as input (needs headroom, or
    is billed above the price step), skipping past the trigger is unsafe: merge the re-compact guard into the
    post-compact guard (stop or warn). If it does not, the guard skips the compaction and compacts at `limit_tokens`.
- **R2. Per-model limits and price steps.**
  - Method: read Anthropic's current pricing and model pages for Haiku 5.5, Sonnet 5.5 and Opus 5.5.
  - Output: a table with, per model, the context window, the input price step (token count and multiplier, or "none"),
    and the source link for each figure.
  - Unblocks: `limit_tokens` per model: the price step if there is one, else the window. `percent`,
    `min_gap_percent` and `max_after_fill_percent` start at the Haiku values (80, 20, 70) for every model, lowered only
    if R1 shows the compact turn needs more room than `100 − percent` leaves.
- **R3. Which mechanism sets Claude's window and trigger?**
  - Method: run interactive, `-p` and `harnez agent` sessions, each with one of the 956 env settings
    (`CLAUDE_CODE_AUTO_COMPACT_WINDOW`, `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE`), `--autocompact`, or settings only, and compare
    `/context` with last-turn input tokens.
  - Output: per session type, the mechanism that took effect and the observed trigger in tokens; whether last-turn
    input tokens match `/context`.
  - Unblocks: how step 3 passes the trigger to the Claude child, and whether the context measure holds.
- **R4. Which hook can show the post-compact message?**
  - Method: in the R1 canary, register SessionStart (source `compact`) and UserPromptSubmit hooks that log their input
    and print a test message.
  - Output: which event fires after compaction, whether it can read the post-compact size, and whether its output
    reaches the user.
  - Unblocks: step 5. If neither event works, step 5 falls back to the message on the next UserPromptSubmit after the
    first post-compact turn, and the ticket records that delay.

## 4. Implementation & Verification Plan

1. **Values.** Fill the per-model table from R1 and R2. Check: every model has all four fields and a source link.
2. **Spec and schema.** Add the four fields per model to `spec/agent.yaml` and its schema, keep `agent.compact_thresholds`
   working, and pass the trigger to the Claude child by the R3 mechanism. Reject at load a `min_gap_percent` that is not
   larger than `percent − max_after_fill_percent`, since it could never fire. Check: schema tests for valid, missing and
   dead values; `harnez agent` resolves 80000 for Haiku.
3. **Re-compact guard.** Implement the R1 outcome (merged into step 4, or skip and compact at `limit_tokens`). Check, by
   unit test: room below `min_gap_percent` takes that action; room at or above it compacts normally.
4. **Post-compact guard in the agent run loop.** Read the post-compact size from the first turn after `/compact`. Check,
   by unit test: fill at or above `max_after_fill_percent` stops the session and returns a warning to the orchestrator;
   below it, the session continues.
5. **User-driven session.** Show the post-compact warning through the R4 hook. Check: a test feeds a post-compact usage
   at 70% and asserts the hook prints the message and does not block.
6. **Live.** One Haiku harnez session compacts at about 80k (±1 turn), and a forced case (large pinned context) shows
   the orchestrator warning and the user hook message. Check: both recorded in this ticket with token counts.
