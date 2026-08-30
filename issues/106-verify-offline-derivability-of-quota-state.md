# 106 — Verify offline-derivability of quota/limit/reset-time state from current storage model

**Status**: Closed — audit complete, findings below
**Priority**: P2 (Medium) — informational/audit; no fix implied by this ticket itself
**Severity**: Moderate
**Category**: Architecture
**Related**: [[103-agy-missing-from-all-usage-aggregate]],
[[104-agy-quota-collector-requires-live-process-poll-coincidence]],
[[105-surface-per-collector-fetch-status-in-usage-ui]],
[[030-agy-codex-missing-local-token-counts]],
[[086-offline-degraded-cache-snapshot-masks-live-data]],
[[087-generalize-flock-freshness-gate-to-codex-agy]],
[[101-usage-keep-stale-agents-visible-until-7d]],
`internal/usage/history.go`, `internal/usage/statecache.go`, `internal/usage/agy.go`,
`internal/usage/claude.go`, `internal/usage/codex.go`, `internal/usage/livefetchcache.go`,
`internal/usage/types.go`, `internal/usage/usage.go`

## Question this ticket answers

> If harnez had zero network/API/live-process access whatsoever, right now, could it still show
> the user a coherent "last known state" for each agent's quota windows, usage percentages, and
> reset times, sourced entirely from what's already persisted on disk?

This is a full storage-model audit across all three tracked agents (Claude Code, OpenAI Codex,
Antigravity/AGY), prompted by issues 103–105's AGY-specific investigation. No code was changed to
produce this report.

## Storage inventory

Four persistent stores exist under `internal/usage`, each with a distinct purpose and distinct
staleness behavior:

1. **Collector-daemon state snapshot** — `<XDG_STATE_HOME|~/.local/state>/harnez/agents/usage/<agent>.json`
   (`StateDir`/`snapshotPath`, `internal/usage/statecache.go:80-93`). One file per agent, written by
   `harnez agent-collector` (`WriteAgentSnapshot`, `statecache.go:102-120`) via `PersistAgentSnapshot`
   (`statecache.go:149-159`), which refuses to overwrite a richer existing snapshot with an emptier
   live result (`hasQuotaSignal`/`hasQuotaWindowSignal` guards). This is the **primary durable store**
   read by `CollectAll` (`usage.go:51-55`, `cacheOrLive`) before any live collect is attempted, and it
   persists the *entire* `AgentUsage` struct verbatim — `Session`, `Weekly`, `ModelGroups` (with each
   `QuotaWindow.ResetAt`), `Tokens`, `ModelTokens`, `Details`, `Sources`, all of it. `AgentSnapshot.FetchedAt`
   (`statecache.go:54-57`) is the store's own staleness clock, independent of any field inside `Usage`.

2. **Usage-history log** — `~/.claude/harnez/usage-history/<hostname>.jsonl`
   (`HistoryDir`/`AppendHistory`/`ReadHistory`, `internal/usage/history.go:35-40,224-292`). Append-only,
   one line per snapshot, each line a full `HistoryEntry{Hostname, UsageSummary}` — i.e. it durably
   captures the same complete `AgentUsage` (including `ResetAt`) at every recorded point in time, not
   just the latest. `latestHistoryQuotaWindow` (`history.go:301-314`) and `fillFromHistoryIfNoQuotaWindows`
   (`history.go:335-349`) read this back as a **secondary fallback**, gated at `DefaultDisplayStaleness`
   (7 days, `statecache.go:49`) measured from the *history entry's own recorded timestamp*, not from
   read time — this is the exact mechanism issue 103 found silently expiring for AGY.

3. **Per-agent live-fetch cache** — `<agent-dir>/harnez-quota-cache.json`
   (`internal/usage/livefetchcache.go`), one per agent: `~/.claude/harnez-quota-cache.json`,
   `~/.codex/harnez-quota-cache.json`, `~/.gemini/antigravity-cli/harnez-quota-cache.json`. Generalized
   to all three agents in issue 087 (confirmed identical `readLiveFetchCache`/`writeLiveFetchCache`
   call sites in `claude.go:178-296`, `codex.go:226-317`, `agy.go:313-413`) — earlier ticket-104 framing
   that treated this as AGY-only/exceptional is superseded; it's a uniform mechanism. This is a
   **cross-process short-circuit cache** (avoids redundant live RPC/HTTP calls within
   `MinWatchInterval`), not a long-term store — its payload is agent-specific (`claudeQuotaPayload`,
   `codexQuotaPayload`, `agyQuotaPayload`), but always includes `Session`/`Weekly`/`ModelGroups` with
   `ResetAt`. On a stale-or-failed live attempt, all three collectors fall back to this file's contents
   *regardless of age*, relabeling window names via `staleQuotaWindow`/`staleModelGroups`
   (`watch.go:936-951`) — which copies the whole `QuotaWindow` struct and only rewrites `Name`, so
   `ResetAt` survives the stale-relabel untouched.

4. **Agent-native on-disk state** (not written by harnez, only read) — e.g. AGY's
   `~/.gemini/antigravity-cli/{settings.json, antigravity-oauth-token, log/*.log, conversations/*.db,
   history.jsonl, conversation_summaries.db}`; Claude's `~/.claude/{settings.json, stats-cache.json,
   ...}`; Codex's `~/.codex/{config.toml, auth.json}`. These feed `Account`/`PlanTier`/`ActiveModel`/
   `Details` (conversation/session/message counts) and, for Claude, local `Tokens`/`ModelTokens` — but
   per issue 104's investigation, **none of AGY's native files carry quota/limit/reset data**; they
   only carry conversation/session metadata. This is a hard ceiling on what any future code fix could
   recover for AGY without an active RPC/API call.

## Per-agent breakdown

### Claude Code (`internal/usage/claude.go`)

- **Live collector produces**: `Tokens`/`ModelTokens` (from local `stats-cache.json` — no network
  needed), `Session`/`Weekly` `QuotaWindow`s with `ResetAt` (from Claude's own quota API/cache).
- **Durably stored**: everything — `Tokens`, `ModelTokens`, `Session`, `Weekly`, all with `ResetAt`,
  in both the state snapshot (confirmed live on this machine: `~/.local/state/harnez/agents/usage/claude.json`,
  fetched 2026-08-29 14:58, carries full `session`/`weekly` blocks with `reset_at` timestamps) and
  `usage-history/*.jsonl`.
- **Live-only / lost on process exit**: nothing structural — Claude's collector doesn't depend on a
  running `claude` process at all (its data sources are local cache files plus a stateless quota API
  call), so there's no "live process must be running" failure mode analogous to AGY's.
- **Offline, right now**: `harnez usage --summary` would render Claude's full quota picture — bars,
  percentages, and reset times — sourced from the ~1-day-old state snapshot, correctly labeled via
  `LastRefreshed`/`Sources` as cached.

### OpenAI Codex (`internal/usage/codex.go`)

- **Live collector produces**: `Session`/`Weekly` `QuotaWindow`s with `ResetAt` (from
  `chatgpt.com/backend-api/wham/usage`, per `sources` in the current snapshot), `Details`
  (`reasoning_effort`, `token_expired`).
- **Durably stored**: `Session`/`Weekly` with `ResetAt` — confirmed live
  (`~/.local/state/harnez/agents/usage/codex.json`, fetched 2026-08-29 14:58, full `session`/`weekly`
  blocks with `reset_at`).
- **Live-only / lost on process exit**: per issue 030 (open, unrelated to this audit but directly
  relevant), Codex has **no local token-count source** at all — `Tokens`/`ModelTokens` are simply never
  populated for Codex, live or cached; that's a pre-existing, separately-tracked gap, not something
  this audit newly discovered.
- **Offline, right now**: same shape as Claude — full quota picture with reset times available from
  the ~1-day-old state snapshot.

### Antigravity / AGY (`internal/usage/agy.go`)

- **Live collector produces**: `ModelGroups` (Gemini Models / Claude+GPT Models pools) with per-window
  `ResetAt`, sourced *exclusively* from a live Connect-RPC call to a running AGY `LanguageServer`
  process (`agy.go:337-382`); plus `Account`/`PlanTier`/`ActiveModel`/`Details.total_conversations` from
  local file scraping (`settings.json`, `antigravity-oauth-token`, `log/*.log`, `conversations/*.db` —
  none of which requires network or a live process).
- **Durably stored**: `Account`/`PlanTier`/`ActiveModel`/`Details` always persist regardless of live
  RPC success, since they come from local files. `ModelGroups` (the actual quota/reset data) is
  persisted to the state snapshot and history log **only on the rare tick where a live RPC happened to
  succeed** (`PersistAgentSnapshot`'s guard doesn't create data, it only avoids destroying it).
- **Live-only / lost on process exit — confirmed live on this machine**: the current state snapshot
  (`~/.local/state/harnez/agents/usage/agy.json`, fetched 2026-08-29 14:58) has **no `session`,
  `weekly`, or `model_groups` key at all** — only `account`/`plan_tier`/`active_model`/`details`/
  `sources`. The usage-history log's last entry with real `ModelGroups` data is 2026-08-23 — as of this
  audit (2026-08-30), that's already past `DefaultDisplayStaleness` (7 days), so
  `fillFromHistoryIfNoQuotaWindows` no-ops (issue 103's mechanism). AGY's own live-fetch cache
  (`~/.gemini/antigravity-cli/harnez-quota-cache.json`) does not exist on this machine at all — it was
  never successfully written (confirmed: only a stray 0-byte `.lock` file). AGY's native on-disk state
  (`history.jsonl`, `conversation_summaries.db`, per-conversation `.db` files, log lines — per issue
  104's file-by-file check) carries **no quota/limit/reset fields whatsoever**; it only ever encodes
  conversation/session activity, never remaining-quota fractions or reset timestamps. There is
  therefore no fallback path — present or hypothetically addable without an RPC/API — that could
  recover AGY quota data purely from AGY's own persistent files.
- **Offline, right now**: `harnez usage --summary --compact` shows AGY's account/plan/model, but
  **zero quota bars and no reset time**, exactly as issues 103/104 describe. This is not a display bug
  on top of good data — the data genuinely isn't on disk anywhere. AGY's gap is total-loss for the
  quota/reset dimension specifically, not partial/stale like Claude/Codex would be if their live paths
  also failed.

## Cross-cutting nuance found during this audit (not previously ticketed)

`QuotaWindow.DurationLeft` (`internal/usage/types.go:13`) is computed **once**, at collection time,
as `resetTime.Sub(now)` (`claude.go:232,252`, `codex.go:106,111`, `agy.go:373`), and then stored
verbatim in every persistence layer above. Every renderer (`usage.go:155,173,202`;
`watch.go:706,1074,1106,1110`) prints the *stored* `DurationLeft` directly (`"in %s"`,
`FormatCompactDuration(w.DurationLeft)`) rather than recomputing `ResetAt.Sub(time.Now())` at render
time. `ResetAt` itself is an absolute timestamp and stays correct indefinitely once persisted — but a
`DurationLeft` served from a stale cache/history/snapshot is a frozen countdown from whenever it was
fetched, not a live one: e.g. a snapshot fetched "2h left" 90 minutes ago will still say "in 2h" when
served now, understating how close the actual reset is (or, once genuinely past reset, will keep
showing a stale positive value since nothing re-derives it from `ResetAt`). This doesn't affect *this*
ticket's core question — `ResetAt` is what durably answers "when does it reset," and it survives fine
— but it's a concrete rendering gap worth fixing alongside any future stale-data-surfacing work
(105-adjacent).

## Summary: durable vs. lost, per agent

| Agent  | Session/Weekly/ModelGroups durable? | ResetAt durable? | Tokens durable? | Offline "last known state" today |
|--------|---|---|---|---|
| Claude Code | Yes (state snapshot + history) | Yes | Yes | Full quota picture, ~1d stale, correctly labeled |
| OpenAI Codex | Yes (state snapshot + history) | Yes | No — never collected at all (issue 030) | Full quota picture, ~1d stale, correctly labeled |
| Antigravity (AGY) | No — only durable when a live RPC happened to succeed on some past tick; none has since 2026-08-23 | No — same caveat; when it was captured, yes | N/A (AGY has no token collection either — issue 030) | Account/plan/model only; **zero quota bars, zero reset time** |

## Recommended follow-up

Documented here for whoever files the next dev ticket (not filed by this audit per the user's
explicit instruction — they will dictate that ticket separately):

1. **AGY quota collection needs a second source or a materially longer live-attempt cadence.**
   Confirmed by this audit and issue 104: there is no on-disk AGY file that carries quota/reset data,
   so a purely-offline fix is structurally impossible for AGY; the only lever is making the live RPC
   attempt succeed more often (e.g. prompting/detecting an AGY-editor-open window, a background poll
   trigger, or accepting a longer-tail RPC retry) — issue 104 ACs already scope this.
2. **Decouple/extend the 7-day display-staleness gate from the aggregate and per-agent panels**
   (issue 103) so historical `ModelGroups`/`Session`/`Weekly` — for any agent, not just AGY — keeps
   feeding the UI past 168h with a visible age/staleness label, rather than silently reverting to
   "nothing to show."
3. **Recompute `DurationLeft` from `ResetAt` at render time** instead of trusting the stored,
   fetch-time-frozen value, in every renderer that prints a "(in %s)" countdown
   (`usage.go:155,173,202`; `watch.go:706,1074,1106,1110`). Small, mechanical, and independent of 1/2.
4. **Surface collector/fetch-provenance status** (issue 105) so a user can tell "genuinely fresh,"
   "stale fallback with an age," and "no data source ever succeeded" apart at a glance — this audit's
   findings are exactly the kind of gap 105 would have made visible without manual disk archaeology.
5. Re-confirm issue 030 (Codex/AGY token counts) is still accurately scoped — this audit reconfirms
   both agents have zero durable `Tokens`/`ModelTokens` today, unrelated to the quota/reset-time
   question this ticket targeted.
