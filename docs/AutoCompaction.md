# Auto-Compaction Triggers per Agent CLI

When each agent CLI compacts on its own, which knobs move that point, and how harnez's own trigger relates.
Consult before changing `compact_threshold_tokens`, `agent.compact_thresholds`, the driver args in
`internal/subagent/` (`claude.go`, `codex.go`, `agy.go`), or per-model compaction values. History: issues 591, 592,
869, 956, 969. Replacing the summary itself is a separate topic: [JevCompaction.md](JevCompaction.md).

Each fact is marked **measured** (owner or canary run, with version), **docs** (official source) or **unverified**
(blogs, issue reports). Re-measure after a CLI upgrade before relying on a number.

## Two triggers

- **harnez:** `compact_threshold_tokens` in `spec/agent.yaml` defaults to 200000; spec `compact_thresholds` defaults Haiku to 80000. User `agent.compact_thresholds` overrides model-specific defaults, which take precedence over the user global threshold (`internal/subagent/compact.go`). Before each prompt,
  `harnez agent` compares the last turn's input tokens with it and sends `/compact` (591); the stream watchdog stops
  a turn that grows past it (592). Absolute tokens only, no percent yet (969).
- `auto_compact` in `spec/agent.yaml` selects manual, native, or disabled routing by provider: Claude uses manual,
  Codex uses native, and agy is disabled until reliable context counts are available (644, 869). The same spec sets
  the watchdog's shutdown grace and the legacy growth-telemetry threshold.
- **The CLI's own trigger,** below. Whichever comes first wins, so harnez values only work if the CLI's trigger is
  set at or above them.

## Claude Code (2.1.311)

Formula (**inferred** from 9 interactive status-line and `/context` readings on Haiku, Sonnet and Opus 5.5; issue
969 §2a). It fits every reading within the status line's rounding (`% left = (trigger − used) / trigger`, rounded;
at percent 70 it predicts 0%, the line showed 1%). An equally good fit is `trigger = min(window − buffer,
PCT × (window − 20k))`: no reading yet has the percent active with a changed reply cap, so whether the reserve inside
the percent term follows `CLAUDE_CODE_MAX_OUTPUT_TOKENS` is unverified.

```
reserve = CLAUDE_CODE_MAX_OUTPUT_TOKENS        (default 20k)
usable  = window − reserve
trigger = min(usable − 13k, PCT_OVERRIDE × usable)
```

- `/context` shows "Autocompact buffer" = reserve + 13k: 33k by default, 23k with `CLAUDE_CODE_MAX_OUTPUT_TOKENS=10000`.
  Its "Free space" is measured to that buffer line and ignores the percent.
- The status line "N% until auto-compact" does include the percent, and shows no hint above about 20% left.
- Example: window 100k → usable 80k → trigger at most 67k; percent 80 → 64k; percent 50 → compacts at once at 56k.
- **Print mode (`-p`) honours the window but not the percent** (**measured**, two canaries, Haiku, window 120k, env
  set directly on the `claude` process): with percent 80 a resumed turn at 85.9k did not compact, the next one
  compacted at 89.3k (`compact_boundary`, `trigger: auto`, `pre_tokens: 89280`, `post_tokens: 7186`); with percent 50
  (trigger 50k) a resumed turn at 85.9k did not compact either. Why the percent is dropped is unknown.
- `post_tokens` is not the next request's size: after `post_tokens: 7186` the next request sent 20,428 tokens
  (system prompt and tools added). A fill guard must use the next request's input, or add that overhead.
- **Hypothesis, not tested inside tool loops:** the check runs when a turn starts, so a single `-p` turn never shows
  compaction; resume the session for a second turn (956).

Knobs (**docs**: https://code.claude.com/docs/en/env-vars, https://code.claude.com/docs/en/model-config,
https://code.claude.com/docs/en/settings-reference, https://code.claude.com/docs/en/hooks):

| Knob | Kind | Effect |
| --- | --- | --- |
| `--autocompact <auto\|tokens>` | flag | window for this launch, 100k–1M, capped at the model window |
| `/autocompact` | command | window, saved per model |
| `autoCompactWindow` | setting | window (owner's `~/.claude/settings.json`: 200000, also in a Sonnet block) |
| `CLAUDE_CODE_AUTO_COMPACT_WINDOW` | env | window; docs say it beats flag and settings |
| `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` | env | percent of usable, 1–100, can only lower the trigger; ignored in `-p` (measured) |
| `CLAUDE_CODE_MAX_OUTPUT_TOKENS` | env | reply cap; also the reserve, so it moves the trigger |
| `autoCompactEnabled`, `DISABLE_AUTO_COMPACT=1` | setting, env | auto-compaction off; `DISABLE_COMPACT=1` also blocks `/compact` |
| `idleCompaction` | setting | idle compaction, on by default |
| `PreCompact` hook | hook | runs before compaction, can block it; `PostCompact` runs after, side-effect only; both fire in `-p` (measured, 987 M1) |

Do not lower `CLAUDE_CODE_MAX_OUTPUT_TOKENS` to compact later: it also shrinks the reply room.

## Codex CLI (**docs**/source; measured behaviour in "Compaction probes" below)

- `model_auto_compact_token_limit` (tokens) and `model_context_window`, in `config.toml`, a `--profile`, or `-c` on
  the command line. No compaction flag or env var. harnez passes `-c model_auto_compact_token_limit=N` (`codex.go`).
- Default and cap: official docs only say "unset uses model defaults"
  (https://learn.chatgpt.com/docs/config-file/config-reference). Codex source on `main` (read 2026-10-27, not pinned
  to the installed version) sets both at 90% of the model window
  (https://github.com/openai/codex/blob/main/codex-rs/protocol/src/openai_models.rs).
- `PreCompact` (stop with `{"continue":false}`) and `PostCompact` hooks.

## Antigravity (agy) (**docs**; measured behaviour in "Compaction probes" below)

- Manual `/compact` is documented; automatic behaviour is unverified. No threshold, flag, env var or compaction hook
  was found in the docs (https://github.com/google-antigravity/antigravity-cli/blob/main/CHANGELOG.md), which does not
  prove there is none.
- **Unverified:** one issue report says agy compacts at about 25% of a 1M window and `AGY_CLI_AUTO_COMPACT_OVERRIDE`
  has no effect (https://github.com/google-antigravity/antigravity-cli/issues/1245).
- harnez's pre-prompt compaction is skipped for agy in print mode (869), so nothing triggers it automatically;
  `harnez agent compact` works on demand (987), but agy reports no context count (644).
- Gemini CLI is a different product (`model.compressionThreshold`, `/compress`, `PreCompress` hook); its settings do
  not apply to agy.

## Model price steps that set limits

| Model | Step | Source |
| --- | --- | --- |
| Claude Haiku 5.5 | input above 100k tokens per request costs 5x | https://platform.claude.com/docs/en/models/haiku-5-5/overview |
| Claude Sonnet/Opus/Fable 5.5 | none found | https://platform.claude.com/docs/en/about-claude/pricing |
| GPT-5.6 Sol | input above 272k costs 2x | https://developers.openai.com/api/docs/models/gpt-5.6 |
| Gemini Flash 3.8, Gemini 4 Argon | none on the Gemini API page | https://ai.google.dev/gemini-api/docs/pricing |

Targets are owner decisions, not derived limits: 200k by default, 80k for Haiku, 100k for Codex terra/sol/luna
(2026-10-27, `spec/agent.yaml` `compact_thresholds`, user override `agent.compact_thresholds`). Codex 100k rationale
(987): one compaction cost about $0.019 and paid back in about 0.64 turns at 10 tool calls per turn, because every
later call re-reads less cached context.
API pricing does not describe subscription quota (for example the AI Pro plan). Each turn's new prompt and tool
results can overshoot a trigger; the room needed above it is still open (969 R1).

Predicted, not yet observed at the point: Claude interactive `--autocompact 120000` with percent 80 → 80k; `-p`
`--autocompact 113000` → 80k via the buffer line (the percent does not work in `-p`); or harnez's own `/compact` at 80k.

## Pitfalls

- Canary filler made of random dictionary words tripped Haiku's safety filter (`api_refusal_category: bio`) and cost
  $0.19; use numbered lines. 4000 lines of `Row 00001: the inventory count is N units.` ≈ 77k tokens.
- Prompts above 100k on Haiku are billed at the higher rate; size canaries to stay below.
- `/context` free space ignores the percent; read the status line, or a `-p` stream's `compact_boundary`.

## Compaction records

Codex and Claude hooks record pre/post events in telemetry; agy records a
`stream_fallback` pair when its compact stream reports a successful checkpoint.
Read the latest pair for a session with `harnez agent status --name <name> --json`
in the `compaction_pair` field. A Pre without a Post shows as `interrupted: true`.

## Compaction probes (issue 987 M1)

All measurements below are dated 2026-10-27. Probes used newly created sessions in a scratch directory; hooks were
added per invocation, with no global hook configuration edits.

- **Measured, owner, Claude Code 2.1.311 and agy 1.4.8:** `claude -c -p "/compact"` and `agy -c -p "/compact"`
  compacted the owner's sessions. **Measured, owner, codex-cli 0.164.0:** `codex exec -c
  model_auto_compact_token_limit=50000 -C <dir> resume <id> "summarise your known status in one line"` compacted
  before answering; the human-readable output said `context compacted`. These `-c` forms select the latest session;
  use explicit IDs when multiple sessions exist.
- **Measured, Claude Code 2.1.311:** `--settings` logging hooks both fired for `--resume <id> -p /compact`.
  `PostCompact` stdin had `session_id`, `transcript_path`, `cwd`, `prompt_id`, `hook_event_name`, `trigger` and
  `compact_summary`; trigger was `manual` and the summary contained the generated summary. `PreCompact` had the
  same common fields and `trigger`, plus `custom_instructions` (null), instead of `compact_summary`.
- **Measured, codex-cli 0.164.0:** both `PreCompact` and `PostCompact` stdin had `session_id`, `turn_id`,
  `transcript_path`, `cwd`, `hook_event_name`, `model` and `trigger`; trigger was `auto`, and neither payload
  included token counts. A limit of 1 on the third invocation triggered compaction of the short scratch session.
- **Measured, codex-cli 0.164.0:** the compacting `--json` stream contained no `context_compacted` event, although
  both hooks fired and the rollout had a `compacted` item. Its payload included `message`, `replacement_history`,
  `replacement_history_metadata`, `retained_context`, window IDs, `compaction_response_id`,
  `latest_token_usage_record` and `resume_metadata`. The JSON event's fields remain unmeasured.
- **Measured, codex-cli 0.164.0:** `turn.completed.usage` was cumulative across normal turns: input 16,307 then
  32,786, output 6 then 12. Rollout `total_token_usage` matched those values, while second-turn `last_token_usage`
  was 16,479 input and 6 output. This is a session cumulative counter in this probe, not a per-turn counter.
- **Measured, codex-cli 0.164.0:** logging `SessionStart` recorded `source` values `startup`, `resume`, `resume`,
  `compact` across the three calls. On the compacting call, both resume and compact fire; multiple configured
  handlers run for each source, explaining the owner's multiple hook lines.
- **Measured, agy 1.4.8:** `--conversation <new id> -p /compact` produced a stream `step_update` with
  `step_type: checkpoint` and success. The scratch conversation's SQLite `steps` row at that index had numeric
  `step_type` 23 and a binary payload containing `<summary>`; no plain JSON transcript marker was established.
  The CLI help exposes no per-invocation hook configuration, so this probe used stream and conversation storage.
- **Measured, codex-cli 0.164.0 and installed harnez:** telemetry recorded pre/post compaction events with session,
  turn, `auto` trigger and model, linked session boundaries, token snapshots and an `insufficient_data` economics
  row. Event token fields were null; pre and post snapshots both contained the prior cumulative 32,798 total tokens,
  so they did not establish a context drop. Two pre and two post rows appeared: the per-run telemetry handler and
  the installed handler both ran; do not interpret this probe as one record per compaction.
