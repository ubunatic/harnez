# 987 — subagent compaction: adopt owner-verified commands (codex resume with low auto-compact limit, claude/agy -c -p /compact)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 985 (per-turn cost check in agent output), 969 (per-model compaction limit, draft), 869 (agy print-mode compaction skipped), 644 (agy context unknown), 590 / 594 (codex resume compaction), 673 (claude compaction check), `docs/AutoCompaction.md`

---

/goal Every provider harnez drives can be compacted on demand and at a per-model threshold, using the commands the
owner verified, and `harnez agent compact` confirms the context dropped; or stop and report when blocked on an owner
decision or a CLI that does not behave as observed. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## 1. Problem & Motivation

Sprint 983 (2026-10-27) kept one Codex developer (`dev-983`, `codex:gpt-6.1-terra:med`) for 18 turns. Cached
re-read reached 47M tokens and the session hit the Codex usage limit before its last commit (details in 985).
Nothing compacted it: harnez cannot compact a Codex session on demand, and Codex's own trigger sat at the global
200k default.

Owner-verified commands (**measured** by the owner, 2026-10-27; installed versions that day: codex-cli 0.164.0,
Claude Code 2.1.311, agy 1.4.8). No re-test needed; record them in `docs/AutoCompaction.md` as measured:

- `agy -c -p "/compact"` and `claude -c -p "/compact"` compact well.
- `codex exec -c model_auto_compact_token_limit=50000 -C <dir> resume <id> "summarise your known status in one line"`
  forces a compaction (slash commands cannot be sent to `codex exec`) and returns a cheap one-line status as a bonus.

Owner transcript (codex-cli 0.164.0, `gpt-6.1-terra` medium, project `neus`, human-readable output, not `--json`):

```
codex exec -c model_auto_compact_token_limit=50000 -C ~/projects/neus resume 01a17c6c-... "summarise your known status in one line"
user
summarise your known status in one line
hook: PreCompact
hook: PreCompact Completed
context compacted
hook: PostCompact
hook: PostCompact Completed
hook: SessionStart              (x4 started, x4 completed)
codex
Issue 102 M3: changes committed and tests and install passed; pinned replay still misses some evidence, ...
hook: Stop
hook: Stop Completed
tokens used
107.088
```

What it shows:

- Compaction runs before the prompt is answered, in the same `exec` call; the reply is a usable one-line status.
- The marker is the line `context compacted`, between the `PreCompact` and `PostCompact` hooks. The matching
  `--json` event is still to be read from one real run (harnez drives Codex with `--json`).
- That call reported 107,088 tokens used (the "107.088" uses a dot as the thousands separator): the compaction
  turn costs about one full read of the old context.
- `SessionStart` hooks fire four times after the compaction. If harnez's own SessionStart hook re-injects its
  preface each time, the fresh context starts fuller than needed; check during implementation (link 969's
  post-compact fill guard).

## 2. Current state (live code, 2026-10-27)

| Provider | `auto_compact` mode (`spec/agent.yaml:16`) | On-demand `Compact()` | Threshold |
|---|---|---|---|
| claude | manual | `Resume(id, "/compact")` (`internal/subagent/claude.go:85`), drop verified via `compact_boundary` (673) | 200k global, Haiku 80k |
| codex | native | returns "manual /compact is unsupported" (`internal/subagent/codex.go:271`) | `-c model_auto_compact_token_limit=<threshold>` on every run and resume (`codex.go:125,144`), 200k for all Codex models |
| agy | disabled | `Resume(id, "/compact")` (`internal/subagent/agy.go:164`), never triggered automatically | none; agy context count unreliable (644, 869) |

Gaps:

1. Codex has no on-demand compaction, so `harnez agent compact` and the pre-prompt trigger cannot shrink a Codex
   session; only Codex's own limit applies.
2. No per-model Codex threshold: Terra, Sol and Luna all compact at 200k.
3. agy compaction works on demand but nothing triggers it, because harnez has no reliable context count for agy.
4. The Claude path uses `--resume <id>`; the owner's form is `-c` (continue the latest session in the dir). Check
   whether they behave the same in print mode; `--resume <id>` is safer when several sessions share a dir.

## 3. Proposal (careful; other options in §4)

- **No canary for the commands** (owner-measured, §1). The only open check is the shape of Codex's compaction event
  in `--json` output, read from one real run during implementation, so the acknowledgement test uses real values.
- **Codex `Compact()`:** a resume with `-c model_auto_compact_token_limit=<low>` (value in `spec/`) and the one-line
  status prompt. Confirm via Codex's compaction event (594 has the acknowledgement parsing); surface the status line
  to the host as the compaction result. Keep the replay safety from 590 (prompt sent only after the compaction is
  confirmed).
- **Per-model Codex thresholds** in `spec/agent.yaml` `compact_thresholds`, the same mechanism Haiku uses.
- **agy:** enable on-demand compaction through the host (`harnez agent compact`) and from 985's cost check, which
  works without a context count (turn count, cached re-read); keep automatic triggering off until 644/869 give a
  count.

### 3b. Track every compaction through Pre/PostCompact hooks (owner direction, 2026-10-27)

Owner: every supported agent should have pre- and post-compact hooks, so harnez can record each compaction (whether
one ran, why, and what it changed) by session ID, and use that record for all compaction decisions.

- Live state (corrected 2026-10-27 after argon-987): Codex `PreCompact`/`PostCompact` (and SessionStart with
  `compact`) are already wired to `harnez codex-telemetry` (`internal/codex/hooks.go:77-85`, present in
  `~/.codex/config.toml`); the host's first search missed it because its output was cut off. Claude and agy
  have no harnez compaction hook (`~/.claude/settings.json` has none). Telemetry already stores compaction events and economics
  (`internal/telemetry/insert.go` `InsertCompactionEvent`, `InsertCompactionEconomics`), fed from stream parsing
  today, not hooks.
- Hook support per agent: Codex has both (owner transcript above; `docs/AutoCompaction.md`). Claude Code has
  `PreCompact`; whether 2.1.311 has `PostCompact` must be re-checked (180 found none at the time). agy and Pi:
  unknown, check docs and record. Where an agent lacks a hook, fall back to the stream event (Claude
  `compact_boundary`, Codex `context compacted`) and say so in the record.
- **agy finding (owner-supplied research, 2026-10-27, agy 1.4.8, read from the hook docs embedded in the binary):**
  no `PreCompact`/`PostCompact`. The six hook events are `SessionStart`, `PreToolUse`, `PostToolUse`,
  `PreInvocation` (before each model call), `PostInvocation` (after tool calls finish) and `Stop`; none fires at
  compaction. Config: `~/.gemini/config/hooks.json`; each hook gets JSON on stdin with `conversationId`,
  `transcriptPath`, `modelName`. The binary has compaction internals (`OnCompactionArgs`, `CompactionInfo`, a
  "Compacting conversation..." label) but no user-facing hook name for them.
  - Substitute (untested inference): a `PostInvocation` hook reads `transcriptPath`, logs step count and any
    compaction marker; consecutive values reveal a compaction and give before/after. First step for agy: run one
    `/compact` and see how the marker appears in the transcript file before building on it.
- **Web/binary research (native Claude session `searcher`, 2026-10-27; web search partly failed):**
  - Claude Code: `PreCompact` exists, matcher `manual` (`/compact`) or `auto`; common stdin fields `session_id`,
    `transcript_path`, `cwd`, `permission_mode`, `hook_event_name` (docs snippet,
    https://docs.anthropic.com/en/docs/claude-code/hooks). `PostCompact`: a community report lists `trigger` and
    `compact_summary` and says interactive manual `/compact` fired Pre but not Post, marked fixed 2026-08-16
    (unverified,
    https://claudeissues.com/issue/78760-bug-interactive-mode-manual-compact-fires-precompact-but-never-postcompact-print).
    Blocking or changing compaction from a hook: no source.
  - Codex 0.164.0 binary strings contain hook events `pre_compact`, `post_compact`, `session_start`,
    `session_end` and an exec event `context_compacted` (rollout item `compacted`). Hook config location, stdin
    fields and event fields: unverified (https://github.com/openai/codex/pull/19905).
  - Codex hook config (searcher, read 2026-10-27): `~/.codex/config.toml`, `[[hooks.<Event>]]` groups with a
    `matcher`. Two `[[hooks.SessionStart]]` groups, both matcher `startup|resume|clear|compact`: one runs
    `codebase-memory-mcp hook-augment`, the other `harnez codex-telemetry`. So each SessionStart trigger runs two
    hooks; the owner run showed two such pairs (likely `resume`, then `compact`; unconfirmed). Codex
    SessionStart's `compact` matcher is a second post-compaction signal next to `PostCompact`.
  - Control run (searcher, same session `01a17c6c`, codex-cli 0.164.0, resumed without the `-c` limit): no
    Pre/PostCompact lines and one SessionStart pair (the resume). So in the compacted run the second pair came from
    the compaction: compaction re-triggers SessionStart with `compact` and runs both hooks again (two runs; a
    logging hook naming the trigger would confirm it).
  - Cost: the control turn reported 265,096 tokens used, the compacted turn 107,088, for the same one-line status
    prompt on the same session. The compaction turn was cheaper than a normal turn on the uncompacted context, so
    an on-demand compaction pays for itself in its own turn here.
  - Open, to measure in the first read-only developer turn with a logging hook (`cat > <scratch>/<event>.json`):
    Claude `PostCompact` in print mode and its fields; Codex hook config path and Pre/Post fields; the
    `context_compacted` JSON fields; whether Codex `turn.completed` usage is per turn or cumulative (two
    consecutive values; also needed by 985); why `SessionStart` fires 4× after a Codex compaction.
- The hooks write one event per compaction: session ID, agent, model, trigger (native limit, harnez threshold,
  on-demand `agent compact`, CLI's own auto), context before/after when the payload carries it, timestamp. Pre
  without a matching Post = failed or interrupted compaction.
- Consumers: `agent compact` confirms the drop from the Post event instead of guessing; 985's cost check and 969's
  re-compact and post-compact fill guards read the same records; `harnez agent list` / stats can show compaction
  count per session.
- Installed through `harnez apply` like the other hooks; hook commands and payload fields per agent in `spec/`.

## Architecture (argon-987, agy:argon:low, 2026-10-27)

- **Record:** reuse the telemetry tables (`compaction_events`, `session_boundaries`, `token_snapshots`,
  `compaction_economics`, `internal/telemetry/insert.go`). New: a pair query `LatestCompactionPair(sessionID)`
  joining pre/post rows; fields `Source` (`hook` | `stream_fallback`), before/after context tokens; a small summary
  on `subagent.Session` (compaction count, last before/after, pending Pre without Post).
- **Feeds:** Codex via the existing hooks (`harnez codex-telemetry`, `internal/server/codex_telemetry.go`) with the
  `--json` `context_compacted` event and rollout `compaction` items as fallback. Claude: add `PreCompact` (and
  `PostCompact` if M1 shows it fires in `-p`) through `harnez apply` (`internal/claude/apply_settings.go`), with
  `compact_boundary` as fallback. agy: a `PostInvocation` hook (`internal/agy/hooks.go`) that reads
  `transcriptPath` for a compaction marker or step-count drop and writes synthetic pre/post records.
- **Consumers:** `agent compact` (`compactSession`, `cmd/harnez/agent_run.go:997`) and `VerifyCompaction`
  (`internal/subagent/compact.go:74`) confirm from the Post record or fallback and print before/after plus the
  status line; 985 resets its per-turn baseline at a compaction; 969's guards read the same records.
- **Codex `Compact()`:** `codex exec resume <id> --json ... -c model_auto_compact_token_limit=<spec low limit>
  "<spec status prompt>"`; confirmed only when a Post/`context_compacted` is seen **and** context tokens dropped;
  the reply is the status line. 590 safety: pre-resume auto-compact blocks until confirmed and aborts before the
  caller's prompt otherwise.
- **Decided differently from §1:** drivers keep `--resume <id>`, never `-c` (several workers can share one dir).
- **Deferred:** done-line format and cost thresholds → 985; gap/fill percent math and agy automatic pre-prompt
  compaction → 969 / 869 (agy stays `auto_compact: disabled` until M4 proves counts).

## Decision (host, 2026-10-27)

- Lean sprint, developer `codex:gpt-6.1-terra:med`, a fresh session per milestone (985's finding: one long session
  cost 47M cached re-read); the ticket carries everything between milestones.
- M1 measures on throwaway sessions it creates itself in a scratch dir. Never resume or compact the owner's
  sessions (e.g. Codex `01a17c6c`/`01a17ab0` in `neus`, the Claude session `searcher`).
- M1 must not edit the owner's global hook files (`~/.codex/config.toml`, `~/.claude/settings.json`,
  `~/.gemini/config/hooks.json`): other live sessions read them. Use per-invocation config instead
  (`claude --settings <file>`, `codex -c ...` or a copied `CODEX_HOME`); if a provider allows no per-run hook,
  record that and measure from the stream/transcript only.

### M1 plan review (host, 2026-10-27)

Plan accepted (per-run hooks: `claude --settings`, `codex -c 'hooks.<Event>=[...]'`; agy has none). Changes:

1. agy: resume by ID (`agy --conversation <id> ... -p "/compact"`, as `internal/subagent/agy.go:142` does), not
   `-c`: `-c` may pick the owner's latest conversation instead of the scratch one. Take the ID from the first
   call's `conversation_id`.
2. Codex: a `-c hooks.<Event>=` override replaces the whole event array, which drops `harnez codex-telemetry` for
   that run. Put the logger **next to** the installed harnez entry (copy its command from `~/.codex/config.toml`,
   read-only), so "what codex-telemetry stores today" is measured from the same run.
3. Codex per-turn vs. cumulative needs two normal turns before the compaction: start, one plain resume, then the
   compacting resume (3 Codex calls).
4. The doc section also records the owner's commands from §1 as measured (owner, 2026-10-27, versions from §1).

### M1 delivered (f726353c): measured hooks and events per provider

`docs/AutoCompaction.md` "Compaction probes (issue 987 M1)". Host checked the raw scratch output: Codex
`turn.completed.usage` input 16,307 → 32,786 → 49,322 over start, plain resume, compacting resume.

### M2 plan review (host, 2026-10-27)

Plan accepted: `compact_thresholds` entries for `codex:gpt-6.1-terra`, `codex:gpt-6.1-sol`, `codex:gpt-6-luna`;
`auto_compact.codex_on_demand_limit_tokens: 50000` and `auto_compact.codex_status_prompt` (required in schema),
loaded but unused until M3.

Decision (host, reversible spec value): 100000 for all three. The live Codex window is ~475k (`agent list`
CONTEXT_SIZE for a terra session) and the old 200k let sprint 983 re-read up to 10M cached per turn; one
compaction costs about one context read (M1, §1), so halving the limit trades a few more compactions for cheaper
turns. Revisit per model once 985 shows per-turn numbers.

### Compaction cost (searcher, 2026-10-27; owner asked whether 100k is really cheaper than 200k)

- Prices (third-party, not OpenAI's own pages, rate uncertain): GPT-6.1 Sol input $2.00, cached $0.10, output
  $10.00 per 1M (https://pricepertoken.com/pricing-page/model/openai-gpt-6.1-sol; one listing says $1.00 / $0.05).
  Whether ChatGPT-plan quota counts cached tokens at the same ratio: no source
  (https://inventivehq.com/blog/codex-cli-usage-rate-limits says credits follow API pricing).
- Owner's session `01a17c6c` rollout, per model request: before 49,415 input (42,368 cached); the compacting request
  49,464 input (49,280 cached) + 482 output (summary); after 19,759 input (12,544 cached, so 7,215 uncached).
- One compaction ≈ 482 × $10/M + 7,215 × $2/M ≈ $0.019. Saving per later request ≈ (49,415 − 19,759) × $0.10/M ≈
  $0.003. Break-even = C / (k × S) ≈ 0.64 turns at k = 10 tool calls per turn. The uncached rebuild is only the new
  summary part, not the whole context, because the system prompt prefix stays cached.
- Not priced: quality loss from the summary (the worker forgets details), and plan-quota counting.

### M2 delivered (6697c28a): per-model Codex thresholds

`spec/agent.yaml` has 100000 for terra, sol, luna (astra stays at the global 200000; not in M2's list) and the
on-demand keys `auto_compact.codex_on_demand_limit_tokens` / `codex_status_prompt`. `harnez agent models` gained a
COMPACT column (host checked live: haiku 80000, astra 200000, terra/sol/luna 100000). `make test-q1` passed.

### M3 plan review (host, 2026-10-27)

Plan accepted (Compact in `codex.go`, rollout reader fixed from `compaction` to the measured `compacted` item,
`VerifyCompaction` accepts rollout evidence, refusal removed, 590 abort-before-prompt in `runResume`). Changes:

1. Codex only compacts when the context exceeds the limit. When the session's last known context usage is at or
   below `codex_on_demand_limit_tokens`, `Compact` returns a plain "context usage N is below the on-demand limit M;
   nothing to compact" without calling Codex (no paid turn). Unknown usage: call Codex and rely on the rollout check.
2. Live check context: ~70k tokens of plain text (above the 50k on-demand limit, below the 100k native limit, so the
   start turn does not compact on its own), not a 320 KB hex file (~200k tokens, would compact natively during start).
   Use `codex:luna:low`; the host runs it.
3. Commit message: `feat: Codex on-demand compaction (issue 987 M3)`.

### M3 delivered (de8f8995): Codex on-demand compaction

Host live check (installed binary, throwaway `codex:luna:low` session `m3-live` in `/tmp/harnez-987-m3-A1osab`):
at 18,657 and 35,307 context usage `agent compact` refused without a Codex call ("below the on-demand limit
50000"); at 71,705 it compacted and printed `Context: 71705 -> 17063 tokens` plus the status line. Codex
truncates long command output, so a ~70k context needed three chunked reads.

### Pre-work for M4 (from M3 review)

1. `compactSession` does not store the result: after the compaction `agent list --dead` still shows CONTEXT_USED
   71.7k for `m3-live`. Save the post-compaction context usage (and a compaction count) to the session record;
   test that the stored value drops.
2. Output wording per 985's terms: `Compacted: context usage 71705 -> 17063 tokens` then `Status: <line>`.

### Pre-work for M3 and M4 (from M1 findings)

1. Codex `--json` emits **no** `context_compacted` event, and `turn.completed.usage` stays cumulative even across
   the compaction (49,322 after it). So M3 cannot confirm a drop from the stream: confirm from the PostCompact
   hook record (session ID + turn ID) or the rollout `compacted` item, and read context before/after from the
   rollout `last_token_usage` / `latest_token_usage_record`, not from `usage`.
2. Codex hook payloads carry no token counts; harnez telemetry's pre and post snapshots both held the cumulative
   32,798, so today's `compaction_economics` row is `insufficient_data`. M3 fills before/after from the rollout.
3. A per-run `-c hooks.<Event>=` override ran **in addition to** the installed hooks (two pre and two post rows),
   so the probe doubled records; the installed config alone gives one pair. M4: pair records by session ID + turn
   ID and tolerate duplicates.
4. Claude 2.1.311 fires `PostCompact` in `-p` with `trigger` and `compact_summary`: M4 can use a real Pre/Post pair.
5. agy has no hooks and no plain transcript marker; the stream's `step_update` with `step_type: checkpoint` is the
   signal. M4 for agy uses that stream event from `Compact()`, not a `PostInvocation` hook; record it as
   `stream_fallback`.

## Milestones

- **M1 (measure, read-only for code):** a temporary logging hook (`cat > <scratch>/<event>.json`), one real
  compaction per provider and a two-turn Codex run. Record in `docs/AutoCompaction.md` (measured, with version):
  Claude 2.1.311 `PostCompact` in `-p` and its fields; Codex Pre/Post stdin fields; `context_compacted` fields;
  Codex `turn.completed` per turn vs. cumulative; trigger name per SessionStart pair; agy transcript marker after
  `/compact`; what `harnez codex-telemetry` already stores today for a Codex compaction. Only the doc changes.
- **M2 (spec, independent of M1):** per-model Codex thresholds (terra, sol, luna), on-demand limit and status prompt
  in `spec/agent.yaml` + schema. Visible: `harnez agent models` or a threshold test shows the per-model values.
- **M3 (Codex on-demand compact, needs M1):** `CodexDriver.Compact`, drop the Codex refusal in `compactSession`,
  print before/after and the status line. Visible: `harnez agent compact --name <codex session>`.
- **M4 (Claude and agy hooks plus pair records, needs M1):** Claude `PreCompact`/`PostCompact`, agy
  `PostInvocation` through `harnez apply` (harnez-owned entries only); Pre-without-Post reported. Visible: one
  compaction per agent leaves a pair readable by harnez.

## 4. Other options

- Fresh session per milestone instead of compaction (lean-sprint policy, or a `resume --fresh` command; see 985):
  no summary cost, no stale context; loses the worker's in-session knowledge, which the ticket channel mostly
  replaces.
- Lower Codex's native limit only (no on-demand path): one spec change, but no host control and no confirmation.
- Let 985's check recommend compact vs. fresh by measured economics (`telemetry` already stores compaction
  economics, `internal/telemetry/economics.go`).

## 5. Verification

- The owner's commands recorded as measured per provider in `docs/AutoCompaction.md`.
- Hook support per agent recorded there (has Pre / has Post / stream fallback).
- Live: one compaction per agent (claude, codex, agy) leaves a Pre and a Post record with the same session ID,
  readable by harnez; a killed compaction leaves a Pre without Post and is reported as such.
- Tests: Codex compact args (low limit, status prompt, `-C <dir>`), acknowledgement required before the next prompt,
  per-model threshold lookup, agy on-demand path.
- Live: `harnez agent compact --name <codex session>` prints the before/after context and the status line; a
  resumed Codex session past its model threshold compacts before the prompt.
- `make test-q1`; `make install`.
