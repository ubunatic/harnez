# 034 — Hook-Triggered File Seek for Local Token Extraction (AGY & Claude)

**Status**: Open  
**Category**: Architecture / Telemetry  
**Related**: [Issue 023: `harnez usage`](023-usage-command-token-quota-tracking.md), [Issue 030: Missing Local Token Counts](030-agy-codex-missing-local-token-counts.md), [Issue 035: Transparent HTTP Proxy Sidecar](035-transparent-proxy-quota-and-token-sidecar.md), [Study: Agent Telemetry](../docs/studies/2026-08-19-agent-telemetry-hooks-proxies-and-log-extraction.md)  

---

## 1. Problem & Context

Issue 030 identified that `harnez usage --watch` only displays a `TokenBreakdown` (input, output, cache read/write) for Claude Code, because Claude aggregates lifetime stats into `~/.claude/stats-cache.json`. AGY and Codex lack a ready-to-read pre-aggregated file:
- **AGY**: Raw token usage exists in local conversation databases (`~/.gemini/antigravity-cli/conversations/*.db`) as Protobuf BLOBs and in workspace transcript files (`.gemini/antigravity/transcript.jsonl`), but querying these via background polling or crawling is heavy, noisy, and requires reverse-engineering binary formats.
- **Polling disk logs**: Running continuous file watchers (`inotify` or poll loops) across arbitrary project directories introduces background CPU drain and file descriptor leaks.

## 2. Proposed Solution: Hook-Triggered File Seek

Lifecycle hooks in Antigravity (`hooks.json`) and Claude Code (`~/.claude/settings.json`) provide synchronous lifecycle events whenever a turn completes. Although hooks do not pass token numbers directly in their stdin JSON payload, they provide exact contextual pointers:

1. **Antigravity (`hooks.json`)**:
   - `PostInvocation` and `Stop` hooks pass `conversationId`, `stepIdx`, and `transcriptPath` (e.g. `/path/to/project/.gemini/antigravity/transcript.jsonl`).
   - A lightweight hook command (e.g., `harnez ingest-hook --agent agy`) receives this payload, seeks to the specific line/step in `transcript.jsonl`, parses the step's token metadata (or calculates diffs), and appends the usage to the shared cache (`~/.cache/harnez/usage.json`).

2. **Claude Code (`settings.json`)**:
   - `Stop` or `PostToolUse` hooks fire at turn completion with session ID and project context.
   - Claude's per-project JSONL transcript (`~/.claude/projects/<slug>/<session-id>.jsonl`) can be read at the exact trailing offset without scanning entire directory trees.

## 3. Advantages

- **Zero Polling / Zero Crawling**: Runs purely on event dispatch when the LLM generates tokens.
- **Accurate Attribution**: The hook payload provides `conversationId`, workspace path, and step index, allowing metrics to be attributed to specific projects and subagents.
- **Lightweight & Sub-millisecond**: Seeking to a specific line in a JSONL file takes <1ms and does not add noticeable latency to agent execution.

## 4. Scope & Limitations

- **Tokens Only**: This mechanism extracts token consumption (input, output, cache); it cannot extract API quota windows (5h/weekly limits or remaining requests), which are returned solely in HTTP response headers.
- **Codex Limitation**: OpenAI Codex CLI does not provide a standard declarative lifecycle hook mechanism, so Codex requires either the proxy approach ([Issue 035](035-transparent-proxy-quota-and-token-sidecar.md)) or remote API polling.

## 5. Implementation Plan

1. **Subcommand `harnez ingest-hook`**:
   - Add hidden/internal command `harnez ingest-hook --agent [agy|claude]` that reads hook JSON on stdin, extracts `transcriptPath` + `stepIdx`, updates the local shared usage cache, and writes `{}` to stdout.
2. **Hook Template Injection in `harnez apply` / `harnez init`**:
   - For AGY: Inject a `PostInvocation` hook into `.agents/hooks.json` or global config.
   - For Claude: Inject a `Stop` hook into `~/.claude/settings.json`.
3. **Connect to `harnez usage`**:
   - Read token totals from `~/.cache/harnez/usage.json` in `internal/usage/agy.go` to populate the `[T]` token toggle in `harnez usage --watch`.

---

## Implementation Plan (revised after 2026-09-04 audit)

The ticket's §5 plan predates the hook infrastructure that now exists. Current
state:

- **AGY hooks are already wired**: `harnez agy-hooks apply|status|hook`
  (`cmd/harnez/agyhooks.go`) writes and validates a `PreToolUse`/`run_command`
  entry in `~/.gemini/config/hooks.json` under a `"harnez"` namespace. Adding a
  second event is an entry in the same file, not new machinery.
- **Claude hooks are declarative** in `config.yaml:95` (`hooks:` list, currently
  a `Stop` sound hook and the `PreToolUse`/Bash `harnez exec hook`).
  `docs/HookRewritePattern.md` records the hard constraint that **two hooks on
  the same matcher do not compose** — relevant if a token hook is ever put on
  `PreToolUse`; a `Stop` hook is a different matcher and is safe.
- **There is no `~/.cache/harnez/usage.json`.** The ticket invents one.
  `internal/usage/statecache.go` already provides per-agent snapshot
  persistence (`StateDir`, `WriteAgentSnapshot`, `ReadAgentSnapshot`,
  `cacheOrLive`) and is what `harnez usage` / `--watch` actually read.
- **AGY transcripts were not found**: no `.gemini/antigravity/transcript.jsonl`
  under any `~/projects/*` on this machine. The premise that a hook payload
  points at a plaintext transcript is **unverified for AGY** — this is the
  ticket's load-bearing assumption and it must be proved before any code.
- **Codex is no longer blocked** either — see the revised plan on
  [030](030-agy-codex-missing-local-token-counts.md): `~/.codex/sessions/**`
  rollouts now carry plain-JSON `token_count` events, readable with no hook at
  all. Codex should be dropped from this ticket's scope entirely.

### Step 0 — canary before code (gating)

Per `docs/other/Canary.md`, do not implement until a throwaway probe answers:

1. Register a no-op AGY hook on `PostInvocation` (or `Stop`) that dumps its
   stdin JSON to a file. Run one real AGY turn. **Does AGY emit that event at
   all, and does the payload actually contain `conversationId`, `stepIdx`, and
   a `transcriptPath`?** The ticket asserts this; nothing in this repo verifies
   it, and no transcript file exists on disk today.
2. If a `transcriptPath` is produced, check whether the referenced file is
   JSONL with readable token fields, or another protobuf/opaque blob.

**If step 0 fails for AGY** (no such event, or no plaintext transcript), close
this ticket in favour of [035](035-transparent-proxy-quota-and-token-sidecar.md)
and note the negative result — that is a legitimate and valuable outcome, and
is cheaper to reach than implementing against an assumed payload.

### Steps (only if step 0 succeeds)

1. **`cmd/harnez/agyhooks.go`** — add an `ingest` subcommand under the existing
   `agy-hooks` group (not a new top-level `harnez ingest-hook`; keep the
   agent-specific hook surface where the other agy hook code already lives, per
   `docs/CLIDesign.md`'s scope-separation bias). It reads the hook JSON on
   stdin, seeks the transcript at the given step, extracts token counts, updates
   the agy snapshot, and writes a pass-through `{}` on stdout.
   Hard requirements: never block, never error the agent's turn — any failure
   logs and exits 0 with `{}`; hold a lock for the read-modify-write of the
   snapshot (the same flock discipline the existing caches use).
2. **`cmd/harnez/agyhooks.go` apply path** — extend the installed `hooks.json`
   entry with the `PostInvocation` (or `Stop`) event alongside the existing
   `PreToolUse` one, and extend `agy-hooks status` to report drift for both.
   Keep both under the single `"harnez"` namespace key.
3. **Snapshot sink** — write an incremental `TokenBreakdown` into the agy
   `AgentSnapshot` via `statecache.go`, adding an accumulator field rather than
   overwriting the live-collected quota data. Decide explicitly whether the
   counter is lifetime-cumulative (like Claude's `stats-cache.json`) or
   windowed; lifetime is simpler and matches what the `[T]` toggle already shows
   for Claude.
4. **`internal/usage/agy.go`** — in `CollectAGY`, populate `usage.Tokens` from
   that accumulator when present, leaving the existing percentage-only quota
   path untouched when it is absent.
5. **Claude**: skip. Claude already has `~/.claude/stats-cache.json`
   (`internal/usage/claude.go:110`), which is a pre-aggregated, cheaper, and
   more complete source than a per-turn hook could produce. Adding a Claude
   `Stop` token hook would duplicate an existing working path and risk
   double-counting. Remove Claude from this ticket's title/scope.
6. **Tests** — hook-handler unit tests in the style of
   `cmd/harnez/agyhooks_test.go`: feed a synthetic hook payload plus a fixture
   transcript, assert the exact resulting `TokenBreakdown` and that stdout is
   `{}`; assert a missing/garbage transcript path yields `{}`, exit 0, and an
   unchanged snapshot.

### Design decisions / tradeoffs

- **Reuse `statecache.go`, do not create `~/.cache/harnez/usage.json`.** Two
  usage caches would drift; the ticket's path predates the existing one.
- **`agy-hooks ingest`, not `harnez ingest-hook --agent`.** The `--agent` switch
  buys nothing once Claude and Codex are out of scope, and the agy hook surface
  already exists as its own command group.
- **Fail-open, always.** A telemetry hook that can stall or fail an agent turn
  is worse than missing token counts.
- **Scope reduction is the main output of this plan**: AGY only, gated on a
  canary; Claude covered by `stats-cache.json`; Codex covered by 030.

### Risks / open questions

- **Primary risk**: the whole design rests on an unverified AGY hook payload
  shape. Step 0 exists precisely to avoid building on it.
- Per-turn hook writes are frequent; the snapshot read-modify-write must be
  cheap and lock-correct or it becomes a contention point during fast turns.
- Attribution to project/subagent is claimed as an advantage but adds schema to
  the snapshot; defer it — get a single global counter working first.
- Double-counting on session resume or hook re-fire needs an idempotency key
  (`conversationId` + `stepIdx`) persisted alongside the accumulator.

### Scope

**Small** for step 0 (a probe and a written-up answer).
**Medium** for the full AGY path, conditional on step 0.

## Findings 2026-09-24 (host, loom agy sessions)

- The agy hooks already store per-call data in `~/.harnez/tool_catalog.sqlite` `tool_calls`: the
  post-tool hook writes `output_bytes` and estimated `actual_tokens` (readcard counter) per result.
  Session 35ecec0a: 348 rows, 580 KB, ~155k estimated result tokens. No command lists them per call.
- Every shimmed shell command has two rows: the hook's `run_command` row stays empty (106 of 348),
  the metrics land on the `harnez exec` row because it is the "latest". Pairing works by order only.
- `input_tokens`/`output_tokens`/`reasoning_tokens` stay empty for agy; transcripts have no usage.
- Calibration against agy `/context` (session 0cbaad85, Gemini 3.7 Flash Low, 142 steps): agy
  counts 99.2k = 18.7k fixed (system prompt 5.5k, tools 11.9k, skills+subagents 1.3k) + 80.4k
  conversation. Transcript bytes/4 gave 48k, so factor ~1.7 (≈2.4 bytes/token). The fixed part is
  resent every step (142 × 19k ≈ 2.7M input tokens before caching).
- Estimate per step: input ≈ 19k + 1.7 × (transcript bytes so far)/4; sum per prompt. Cache
  discount unknown, so relative cost only.
- Proposed next step: `harnez stats --session <id> --calls` per-call/per-prompt listing, and merge
  hook and exec rows. See 035 for the network path to real counts.
