# 104 — AGY quota collector only ever records data when a live process poll coincides with a running AGY instance

**Status**: Closed — resolved in 783cfcc; `CollectAGY` now shells out to `agy -p "/usage"` instead of scanning `/proc` for a live LanguageServer process
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [[103-agy-missing-from-all-usage-aggregate]], `internal/usage/agy.go`, issue 087 (flock+freshness gate generalized to Codex/AGY), issue 033 (original live-fetch cache mechanism)

## Resolution

`CollectAGY` (`internal/usage/agy.go`) no longer scans `/proc` for a running
AGY `LanguageServer` process or calls its Connect-RPC endpoint. It shells
out to `agy -p "/usage"` (canary-verified in this ticket's addendum below
to not itself consume model quota, even against an exhausted account) and
parses the plain-text quota table into `ModelGroups`. This removes the
live-process-poll coincidence entirely: a fresh `agy` invocation is
spawned on demand rather than requiring one to already be listening.

The existing on-disk `harnez-quota-cache.json` freshness gate and
cross-process flock (issue 033/087) are unchanged — only the fetch
mechanism underneath them changed. A failed or empty fetch still falls
back to the cached reading (labeled stale) and never overwrites the
on-disk cache with empty data, addressing this ticket's acceptance
criteria 1 and 2 (`QuotaFetchError` is now always set on a failed/empty
attempt, addressing criterion 3).

Verified live against the real `~/.gemini/antigravity-cli`: `harnez usage
--summary` and `--summary --compact`'s `[a] All Usage` both now show
fresh AGY quota bars (also resolving issue 103's symptom), addressing
criterion 4. `go test ./...`/`make check` pass, including new regression
tests `TestCollectAGYFailedFetchPreservesDiskCache` and
`TestCollectAGYEmptyFetchPreservesDiskCache`. See
`docs/studies/2026-08-28-usage-collector-daemon-architecture.md` §6 for
the broader per-agent quota-source architecture note (why this pattern
applies to AGY but not Claude/Codex).

`findAGYPorts()`'s flagged efficiency/robustness concerns (see Notes
below) are moot now that the function is deleted along with the rest of
the RPC/proc-scan path.

## Problem

Issue 103 documented that AGY's last recorded quota snapshot in
`~/.claude/harnez/usage-history/*.jsonl` is from 2026-08-23 and diagnosed
this as a 7-day staleness-gate cutoff (`fillFromHistoryIfNoQuotaWindows`,
`DefaultDisplayStaleness`) silently discarding otherwise-valid historical
data once it crosses 168h.

New evidence contradicts the implicit assumption that data *would* still
be flowing in if not for that gate: the user's own Antigravity "Resume"
conversation list shows real AGY activity as recent as **1 day ago**
("Synchronizing Harness Template Updates", 728 steps) and **2 days ago**
(two more conversations), out of 92 total conversations. On this machine,
`~/.gemini/antigravity-cli/conversations/*.db`,
`~/.gemini/antigravity-cli/history.jsonl`, and
`~/.gemini/antigravity-cli/log/cli-*.log` all show write activity as
recent as **today** (2026-08-30). AGY is being used continuously. Yet
harnez has recorded **zero** successful quota snapshots since 2026-08-23
— a 7-day gap during genuinely active daily use. The 7-day gate in issue
103 is a downstream symptom; this ticket is the upstream cause of why
there was nothing fresh for that gate to admit in the first place.

## Root cause (file:line)

`CollectAGY` (`internal/usage/agy.go:192-426`) has exactly one source of
quota-window data (`ModelGroups`): a live Connect-RPC call to a listening
AGY `LanguageServer` process, discovered by scanning `/proc` for a
process whose cmdline contains `"agy"` or `"antigravity"` and correlating
its open socket fds against `/proc/net/tcp{,6}` listen entries
(`findAGYPorts`, `agy.go:66-147`, wired in via `findAGYPortsFn` at
`agy.go:60`). If no such process is currently running:

1. `findAGYPortsFn()` returns an empty port list (`agy.go:334`).
2. The `for _, port := range ports` loop (`agy.go:337-382`) never
   executes, so `fetched` stays `false` and `lastErr` stays `nil` — note
   `len(ports) > 0` gates the `QuotaFetchError` assignment
   (`agy.go:383`), so with zero ports there isn't even an error surfaced,
   just silent no-op.
3. The code then falls back to the on-disk `harnez-quota-cache.json`
   (`agy.go:397-414`, "regardless of its age, labeled stale") — **but
   only if that file exists**. On this machine it does not
   (`~/.gemini/antigravity-cli/harnez-quota-cache.json`: no such file),
   confirmed by directly listing the directory during this
   investigation — only `harnez-quota-cache.json.lock` (an empty
   0-byte lock file from `lockLiveFetchCache`) is present, never a
   successful payload write.
4. Result: `usage.ModelGroups` stays `nil`/empty for this call. This is
   exactly the condition `fillFromHistoryIfNoQuotaWindows` (issue 103)
   was built to paper over via `usage-history/*.jsonl` — but since no
   live fetch has *ever* succeeded recently enough to append a fresh
   history entry either, that fallback has nothing recent to serve.

**The deeper issue**: `CollectAGY` never reads AGY's own persistent,
continuously-updated state — `~/.gemini/antigravity-cli/history.jsonl`,
`conversation_summaries.db`, or the per-conversation `.db` files under
`conversations/` — for quota information. Those files *are* read, but
only for `total_conversations` count (`agy.go:291-304`) and for
`Account`/`PlanTier` log-scraping (`agy.go:242-289`); none of that path
recovers `ModelGroups` (the Weekly/Session-equivalent quota windows AGY
reports). Quota data is therefore hostage to whether an AGY process
happens to be listening on a TCP port at the exact moment harnez's
collector tick runs — a coincidence that, per the user's own usage
pattern (AGY invoked per-task rather than kept running as a persistent
background daemon), apparently hasn't lined up even once in 7 days
despite near-daily actual usage.

## Reproduction (as run, 2026-08-30 ~15:00 CEST)

```
$ ps aux | grep -i "agy\|antigravity" | grep -v grep
(no output — no AGY process currently running)

$ ls ~/.gemini/antigravity-cli/harnez-quota-cache.json
ls: cannot access '...harnez-quota-cache.json': No such file or directory

$ ls -la ~/.gemini/antigravity-cli/history.jsonl ~/.gemini/antigravity-cli/conversation_summaries.db
-rw------- 1 uwe uwe 471314 Aug 30 14:58 history.jsonl
-rw-r--r-- 1 uwe uwe  57344 Aug 30 14:58 conversation_summaries.db
# both written TODAY — AGY was demonstrably active very recently

$ harnez usage --summary
...
┌─ [G] Antigravity (AGY) ────────┐
│ u***l@gmail.com · Consumer     │
│ model: Gemini 3.7 Flash (Low)  │
│ updated just now               │   <- no quota bars at all
└────────────────────────────────┘

$ harnez usage --summary --compact
...no AGY row in [a] All Usage at all (matches issue 103)
```

This confirms both issue 103's symptom (still reproducing live) and this
ticket's upstream mechanism: `CollectAGY`'s only quota path
(live-process RPC) found zero candidate ports, and the stale-cache
fallback had nothing to fall back to, despite AGY's own on-disk state
being hours-old, not days-old.

## Acceptance Criteria

1. `harnez usage --summary --compact` must reflect AGY activity that
   occurred within the last 7 days (or whatever the current staleness
   window is) even when no AGY process happens to be running at the
   moment harnez's collector polls — consistent with how Claude
   Code/Codex's usage reflects recent activity without requiring the
   respective CLI to be live at poll time.
2. Investigate whether AGY exposes quota/limit information anywhere in
   its persistent on-disk state (`history.jsonl`,
   `conversation_summaries.db`, per-conversation `.db` files, or log
   lines) that `CollectAGY` could parse as a secondary source when no
   live process is reachable — analogous to how Claude Code's collector
   presumably doesn't require `claude` to be actively running to report
   recent quota. If no such on-disk quota data exists in AGY's own
   files, document that constraint explicitly and consider whether
   harnez should instead prompt/detect a periodic background poll
   window (e.g. only while the user's editor/AGY extension is loaded)
   rather than silently going dark.
3. `QuotaFetchError` should be surfaced (not silently swallowed) even
   when `len(ports) == 0` — i.e. "no running AGY process found" should be
   a distinguishable, loggable state from "RPC call failed" or "RPC call
   succeeded", so future debugging doesn't require re-deriving this via
   manual `/proc` inspection.
4. Once fixed, verify with a real `harnez usage --summary --compact` run
   that AGY reappears in `[a] All Usage` using data that is provably
   fresher than the 2026-08-23 snapshot, without requiring the user to
   manually keep an AGY process running in the foreground.

## Notes for whoever picks this up

- This ticket and issue 103 are related but not duplicates: 103 is about
  the *display/fallback* layer discarding data past a 7-day cutoff; 104
  (this ticket) is about the *collection* layer never having fresher data
  to offer in the first place, because its only quota source requires a
  live process-poll coincidence. Fixing 103's staleness-gate alone would
  not fix this ticket — historical data would still stop advancing past
  whatever the last lucky live-fetch happened to be.
- `findAGYPorts()` (`agy.go:66-147`) also has efficiency/robustness
  concerns worth a look while in this code (iterating all of `/proc`,
  re-reading `/proc/net/tcp{,6}` per candidate process) but that's
  out of scope here — flagged for visibility only.

## Addendum (2026-08-30) — `agy -p "/usage"` as an alternate collection path

Inbox note (`issues/inbox/simple-agy-usage-fetch.txt`) proposed shelling
out to the `agy` CLI's own `/usage` slash command instead of the
live-RPC/`/proc`-scan path. Output is plain, fixed-column text, one quota
window per line:

```
$ agy -p "/usage"
Gemini Models          Weekly Limit Remaining     1%   2026-08-31T16:27:56Z
Gemini Models          Five Hour Limit Remaining  91%  2026-08-30T19:33:52Z
Claude and GPT models  Weekly Limit Remaining     31%  2026-09-04T10:25:29Z
Claude and GPT models  Five Hour Limit Remaining  0%   2026-08-30T19:47:02Z
```

This sidesteps the coincidence problem entirely: instead of requiring an
AGY process to already be listening on a port at poll time, harnez would
spawn its own short-lived `agy` invocation and read the answer from
stdout — the same `exec.Command`/`exec.CommandContext` shell-out pattern
already used for `ssh`, `stty`, `ps`, and `lspci` elsewhere in
`internal/usage` (`remote.go`, `watch.go`, `load.go`). No `/proc`
scanning, no socket-inode correlation, no Connect-RPC payload shape, and
no dependency on the live-fetch cache/lock (issue 033/087) as a
coincidence-mitigation layer.

**Canary probe run this session** (per `docs/Canary.md`, probe before
building): the biggest open risk — whether polling `/usage` would itself
consume model quota, which would make periodic `--watch` polling
self-defeating — is resolved. With the account's Five-Hour quota for
"Claude and GPT models" already at 0%, `agy -p "say hi"` (a real model
turn) failed immediately: `Error: Individual quota reached... Resets in
1h54m48s.` Run back-to-back, `agy -p "/usage"` still succeeded and
returned fresh data. `/usage` is answered locally/out-of-band from the
model-quota-metered path, so polling it does not self-consume.

**Still unverified before adopting this as the fix**:
- Cold-start latency of spawning a full `agy` process per poll (relevant
  for `--watch`'s polling cadence; default `--print-timeout` is 5m,
  suggesting startup isn't instant, though `/usage` itself answered
  quickly in the ad-hoc test above — not yet measured precisely).
- Output format stability across `agy` versions/locales — this is text
  scraping, not a versioned API, same fragility class as the existing
  `Account`/`PlanTier` log-scraping at `agy.go:242-289`.
- Whether `agy` needs to be on `PATH` in every context harnez's collector
  runs in, including a future background collector daemon (issue 082).

Recommendation: prototype `CollectAGY` against this path behind the
existing `findAGYPortsFn`-style test seam (a package-level func var) so
tests can stub the exec call, and time a handful of real polls before
committing — but this looks like the strongest candidate fix for this
ticket's acceptance criteria 1 and 2.
