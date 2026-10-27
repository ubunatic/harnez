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
