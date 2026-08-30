# 2026-08-28 — Usage-Collector Daemon Architecture, Omarchy Comparison, and Rejected Alternatives

Case study for the sprint that produced issues 082–087 (background collector daemon, self-hiding
agent display, and follow-on fixes). Captures the research and architecture decisions that don't
otherwise have a home outside scattered ticket text.

---

## 1. Prior art: Omarchy 4.0 "Quattro"'s Quickshell Agents widget

Researched as a direct comparison point before designing [[082]]/[[083]]. Omarchy (an Arch/Hyprland
distro) shipped a real, core status-bar widget in its Quickshell-based desktop shell rebuild:

- One icon/panel per detected AI coding subscription (Claude, Codex, Fireworks — no
  Gemini/Antigravity support), self-hiding until a scan finds real usage for that provider.
- Shows plan tier, session/weekly meter with reset countdown, prepaid balance gauge, 7-day
  token-by-day chart, per-model token breakdown.
- Architecture: a QML display layer that only file-watches JSON snapshots in
  `~/.local/state/omarchy/agents/usage/`; separate periodic **shell scripts**
  (`omarchy-agent-usage-<provider>`) do the actual collection and emit the JSON contract. Adding a
  new agent means shipping a new collector script — the widget itself needs no changes. Also
  supports merging usage snapshots across machines via a synced folder (Syncthing/Dropbox-style),
  taking max/union to avoid double-counting.

Findings that shaped harnez's design:
- Omarchy's Claude collector reads the same sources harnez already did (OAuth usage endpoint +
  local stats cache) — validated harnez's existing data-sourcing as correct, not idiosyncratic.
- Its Codex collector has the same gap harnez has: no reliable local token-count source, quota-%
  only — confirms this is an upstream/provider limitation, not something harnez is missing.
- Two ideas were worth adopting, one was explicitly rejected:
  - **Adopted (082)**: collector/display decoupling — a background process writes JSON snapshots,
    the interactive tool just reads them.
  - **Adopted (083)**: self-hiding/auto-discovery — only show a box for an agent once it actually
    has recorded usage.
  - **Rejected**: shell-script collectors. This project's convention (`docs/lang/Go.md`) is
    Go-only, no bash wrappers for anything beyond simple Make recipes — the collectors are the
    existing Go functions (`CollectClaude`/`CollectCodex`/`CollectAGY`), just invoked by a new
    timer-driven daemon subcommand instead of cron'd shell scripts.
  - Cross-device sync (Omarchy's synced-folder merge) was *not* adopted this round — noted as a
    future idea, not filed as a ticket, since harnez's usage tooling is currently single-machine.

## 2. Two-layer caching, and why they don't unify

Issue 082 added a **second**, separate cache layer alongside the one added by issue 033:

| Layer | Scope | Location | Locking | Purpose |
|---|---|---|---|---|
| `quotaCache` (033, in `claude.go`) | Claude only | `~/.claude/harnez-quota-cache.json` | Advisory flock (best-effort; a lock miss still performs the fetch, just skips persisting) + a `MinWatchInterval` (30s) freshness short-circuit | Stop concurrent `harnez` processes from re-hitting Anthropic's live quota endpoint within the same short window |
| `statecache` (082, in `statecache.go`) | All agents | `~/.local/state/harnez/agents/usage/<agent>.json` (XDG) | None — atomic write-then-rename only; the daemon is the sole writer | Let the TUI/CLI read a recent snapshot instead of collecting live at all, independent of whether any terminal is open |

These are deliberately layered, not merged: `CollectAll`'s cache-first read (082's layer) falls
back to the per-agent live collector on a stale/missing snapshot, and *that* live collector for
Claude still goes through 033's flock+freshness gate underneath. The two layers solve different
problems (033: throttle a hot, frequently-polled endpoint; 082: avoid collecting live at all when
a recent daemon snapshot exists) and neither depends on the other.

## 3. Gap this surfaced: Codex/AGY have no live-fetch throttle

033's flock+freshness gate was Claude-only. Codex (`codex.go`) and AGY (`agy.go`) have **no**
internal cache or lock at all — every live collect call hits their endpoints unconditionally. This
predates 082 and isn't made worse by it (082's outer cache still reduces overall call volume
whenever the daemon is running and fresh), but it means the narrow race — the daemon's own tick
and a concurrent `--watch` live-fallback both calling Codex/AGY's live endpoints at once — is
still open for those two agents. Filed as [[087]]. Resolution direction: generalize 033's existing
pattern into a shared helper, not build something new (see next section).

## 4. Rejected: SQLite/DuckDB as a replacement for flock + JSON files

Raised as a question during 087's investigation: since SQLite (or DuckDB) handles multi-process
locking inside the library, would it replace the flock/JSON-file approach more robustly?

**Assessment**: the actual problem is a cross-process mutex plus a last-fetch timestamp over
~3 small records — not a data-modeling or query problem. There's no join/aggregation need today
(even [[084]]'s future aggregate quota view is "combine 3 small structs in Go," not a SQL query
over history).

- **SQLite** would solve the coordination problem correctly (transactional check-then-fetch-then-write
  instead of flock's best-effort), but adds a new dependency (a cgo driver, or a heavier pure-Go
  one like `modernc.org/sqlite`), a schema, and a migration story — machinery sized for a query
  need that doesn't exist yet. Cuts against `docs/lang/Go.md`'s "avoid deps" bias.
- **DuckDB** is the wrong tool for *this* problem specifically — it's an OLAP engine optimized for
  analytical queries over larger datasets, with a weaker multi-process write-concurrency story
  than SQLite, not a lightweight mutex/cache store. It could be worth reconsidering later for
  querying accumulated `usage-history` data (a genuinely different problem: historical analysis,
  not live-call gating) if [[084]]'s aggregate-window work ever needs real query capability over a
  large history — but not for this.
- **Decision**: generalize 033's existing flock+freshness pattern into a shared helper applied to
  all three agents ([[087]]), rather than introducing an embedded database. No new dependency; the
  pattern is already implemented and tested for Claude.

## 5. Bug found while investigating a "no numbers in --watch" report

While assessing whether 082's daemon rollout coordinated correctly with the TUI's live-fallback
mode, a live repro of a user report ("I don't see any numbers in `--watch`") turned up a real,
separate bug: a `harnez agent-collector --once --offline` smoke-test run (done during 082's own
development) had left a snapshot in the shared state dir with token totals but no quota/session
window data (since `--offline` skips the live API call). That snapshot's timestamp was still
within `DefaultCacheStaleness` (30 min), so `CollectAll` served it as fresh — silently omitting
the quota bars a live collect would have shown, without any indication the data was incomplete.
Confirmed by clearing the cache directory: full quota-bar display returned via live fallback.

**Lesson**: a cache-freshness check (age) does not imply a cache-*completeness* check (was this
snapshot produced by a full/successful collect, or a partial/offline one?). Filed as [[086]] —
fix direction is to either not persist offline/partial collects to the shared cache at all, or tag
snapshots with an `offline`/`partial` flag so the freshness gate can treat them differently from a
full collect.

## 6. Per-agent quota source: durable local files vs. CLI exec (`-p "/usage"`)

Added 2026-08-30 while implementing [[104]]. `CollectClaude`/`CollectCodex`/`CollectAGY` (all in
`internal/usage`) do not all read quota from the same *kind* of source, and that's a deliberate,
per-agent choice rather than an inconsistency to clean up:

| Agent | Quota source | Why |
|---|---|---|
| Claude Code | Local files (`~/.claude/stats-cache.json` etc.), no process needed | Durably persists session/weekly windows and reset times to disk regardless of whether `claude` is running (confirmed in the [[106]] audit) |
| Codex | Local files (`~/.codex/...`), no process needed | Same durability story as Claude, confirmed in the same audit |
| AGY | `exec.Command("agy", "-p", "/usage")`, parsing plain text output | AGY has **no** durable on-disk quota state at all ([[106]]) — its own files never carry quota/reset fields, so a live source is the only option |

AGY's collector originally used a live Connect-RPC call to a currently-running AGY
`LanguageServer` process, discovered by scanning `/proc` ([[104]]'s root cause: that RPC/proc-scan
mechanism required a running process to coincide with harnez's poll, which per real usage logs
hadn't happened once in 7 days of near-daily AGY use). Replaced with shelling out to `agy -p
"/usage"` — AGY's own CLI answering its own `/usage` slash command as plain text — which spawns
its own process on demand instead of gambling on one already listening on a port.

**Why this isn't also applied to Claude/Codex**: it was tested — `claude -p "/usage"` and
(untested but architecturally identical) `codex`'s equivalent also work as a CLI-exec source — but
would be a strict downgrade for those two: `CollectClaude`/`CollectCodex` already read durable
local files directly, which is cheaper (no process spawn per poll) and more robust (a stable file
format vs. scraping CLI prose that could change between versions) than shelling out. The `-p
"/usage"` pattern earns its keep specifically where no durable on-disk source exists — that was
AGY's unique gap, not a general technique to roll out everywhere.

**Token-cost concern, checked for both agents**: polling a live source on every `harnez` tick would
be self-defeating if the poll itself consumed the quota it's trying to report. Canary-tested for
AGY ([[104]]'s addendum): with the account's real model quota fully exhausted, `agy -p "say hi"`
(an actual model turn) failed immediately with the quota-exceeded error, while `agy -p "/usage"`
still succeeded right after — `/usage` is answered locally/out-of-band from the model-metered path.
Reasoned but not independently canary-tested for `claude -p "/usage"`: since `CollectClaude` already
reads Claude Code's quota from local files with no live call, `/usage` almost certainly renders
from that same local state rather than invoking the model — consistent with AGY's confirmed
behavior — but this wasn't run against an exhausted-quota session to verify directly, since doing
so would have cost real remaining quota on a session already low. Moot for harnez's own collection
either way, since Claude already has a better (file-based) source than exec.

## Related issues

[[082-agent-usage-collector-daemon]], [[083-usage-tui-self-hiding-auto-discovery]],
[[084-aggregate-quota-window-box-assessment]], [[085-watch-tui-show-collector-daemon-status]],
[[086-offline-degraded-cache-snapshot-masks-live-data]],
[[087-generalize-flock-freshness-gate-to-codex-agy]],
[[103-agy-missing-from-all-usage-aggregate]],
[[104-agy-quota-collector-requires-live-process-poll-coincidence]],
[[106-verify-offline-derivability-of-quota-state]]
