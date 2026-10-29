# Argon refinement draft for 991 (pending owner go)

One-shot `agy:argon:low` advisor turn (argon-991c, 8m08s) with read tools, 2026-10-28. Not yet
written into the ticket. Before adopting: drop the trailing `<!-- GOAL_COMPLETE -->` and `/goal`
line, and drop the model name from the sandbox rules (forgewerk config picks it).

---

# 991 — harnez usage fails with no collector snapshots and cannot show Codex free-plan quota

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 829, 714, 942, 954, 955, 992

---

## Sandbox & Circuit Breaker Rules

This ticket is scoped for a single developer job (`claude:opus:med`) running in a sandbox (git worktree, no forge network access, no `systemd --user` session, no live Codex account, no `git push`).

- **Circuit breaker**: Do not loop on blocked OS/network operations or guess external API behavior. If blocked by sandbox constraints, an architectural conflict, or a failing prerequisite, stop immediately and record a **Blocked / Failure Report** section at the bottom of this ticket (what was completed, what is blocked, why, and the exact resume point for the host).
- **No process killing**: Item 3 must **never** kill or signal processes. Detect and report a stray non-systemd server only.
- **Drop-to-follow-up rule**: Items are ordered by value and risk below. If the job runs short on time or hits unexpected complexity in the agent picks plumbing, **drop Item 4 to follow-up ticket 992** after completing Items 1–3 and record the handoff notes in the ticket.

## Observed (2026-10-28, harnez 0.2.4, found while bootstrapping forgewerk)

- `harnez usage` (also `--compact`, `--agent codex`) prints
  `Error: read usage store: usage store has no collector snapshots yet` and then the full usage
  help. Exit status still 0.
- `harnez server status`: `harnez-agent-collector.service (installed=false, active=false)`, yet a
  `harnez server` process (PID 25109, started 13:37:34) owns the socket. Its uptime was over
  4 minutes and there was still no snapshot.
- Codex is logged in via ChatGPT on the free plan (`codex login status`: "Logged in using
  ChatGPT"). That plan has a small quota, and harnez has no way to show it.
- `harnez agent models --picks`: availability is `unknown` for every model. The default picks
  resolve to codex (luna/terra/sol) even when its quota is tiny or used up, with no fallback.

## Findings (2026-10-28, after `harnez server install`)

- **Install leaves the unit unable to start.** The unit was enabled but failed with
  `status=226/NAMESPACE`: `Failed to set up mount namespacing: /home/alfiz/.cache/harnez: No such
  file or directory`. Every `ReadWritePaths=` entry (`%t/harnez %S/harnez %C/harnez
  %h/.local/share/harnez` in `systemd/harnez-agent-collector.service:21`) must exist when unprefixed,
  and `%C/harnez` did not. Fix: `harnez server install` (`internal/server/systemd.go:110-142`)
  creates the directories, and the unit (`systemd/harnez-agent-collector.service:21`) prefixes the
  entries with `-`. Creating the directory by hand fixed it.
- **Stray server holds the socket.** A `harnez server` that had not been started by systemd (spawned
  detached by `EnsureLocalServer` -> `startDetachedServer` in `internal/usagecli/client.go:99-116,170-187`;
  it had 0 clients and never wrote a snapshot because `runCollectorWithDeps` in
  `internal/usage/collector.go:174-190` skips the initial fetch when `tracker != nil && tracker.Count() == 0`
  and sleeps for `collector_cadence: 5m` from `spec/usage.yaml:16`) owned the socket, so the unit
  could not take over (`claimExisting` in `internal/server/server.go:111-114` returns
  `errAlreadyRunning` when an existing server has the same version and build identity) until that
  process was stopped. Then `systemctl --user restart` worked, and the first snapshot arrived
  within seconds.
- **Codex root cause.** `harnez usage --agent codex --json` shows `"authenticated": false,
  "error": "invalid auth.json format"`. `CodexAuth.Tokens` was `map[string]string`
  (`internal/usage/codex.go:21`), but the current Codex CLI writes a nested object
  `tokens.oauth_provenance {issuer, client_id}` (alongside `id_token`, `access_token`,
  `refresh_token`, `login_owner_id`, `account_id`), so `json.Unmarshal` fails for every ChatGPT
  login. Fix: give the tokens a struct with string fields and ignore unknown nested keys; add a
  test fixture with `oauth_provenance`. Only after that can you tell whether the free plan's
  rate-limit response is parsed correctly (Expected item 2).

## Progress (2026-10-28)

- Done: `CodexTokens` struct replaces the token map (`internal/usage/codex.go:18-31`, commit `2c8f0ef8`,
  plus preflight `codexAuthFile` in `internal/subagent/codex_preflight.go:68-74`, commit `3297720e`);
  regression tests `TestCollectCodexParsesNestedTokenFields` (`internal/usage/codex_test.go:256-293`)
  and `TestCodexAuthFileParsesNestedTokenFields` (`internal/subagent/codex_preflight_test.go:212-222`).
  Live after `make install`: Codex authenticated, `plan_tier: Free`, one window at 0% used,
  resetting 2026-11-26 (~29 days out).
- New: the free plan reports a single ~30-day window, which `buildCodexQuotaWindow`
  (`internal/usage/codex.go:177-206`) labels `Weekly`. Name windows by their real length (e.g.
  `Monthly`), and confirm the free plan never has a 5-hour window.
- Unrelated, already on main: `TestCollectCatchupFetchesOnlyProvidersOlderThanThreshold` has a
  data race under `-race` (`internal/usage/catchup_test.go:18-41`, where `fetched := map[string]int{}`
  is mutated concurrently at lines `22` and `25` without a mutex while read at line `39`), so
  `make test-pkg PKG=./internal/usage/...` fails without this change too. File it separately if no
  ticket covers it (or guard `fetched` with a `sync.Mutex` / `atomic.Int32` if touching `catchup_test.go`).
- Still open: Expected items 1 (empty-store message, unit dirs, stray server) and 3 (picks
  fallback, see 992).

---

## Implementation Scope (Ordered by Value & Risk)

### Item 1: Empty usage store error handling (No help dump, actionable message, non-zero exit)
- **Environment**: **Sandbox-doable** (code + unit tests).
- **Verified Code Locations**:
  - `cmd/harnez/usage_cmd.go:41-222` (`newUsageCmd()`): creates `usageCmd` **without** `SilenceUsage: true` (unlike `newUsageExportCmd()` at `cmd/harnez/usageexport.go:76`). When `RunE` returns an error at lines `100` (`--loom`), `127` (`--watch`), `182` (`--json` / `--raw`), `216` (default / `--compact` / `--minimal` / `--table`), or in `historyRecordCmd` at `usage_cmd.go:304`, Cobra prints the full usage/flags block after `Error: ...`.
  - `cmd/harnez/main.go:129-131`, `cmd/harnez/clilog.go:178-188` (`executeAndRecord`), `cmd/harnez/exitcode.go:47-60` (`exitCodeFromRunError`): maps any non-nil `root.Execute()` error to exit status `1`.
  - `internal/usage/readstore.go:17-48` (`UsageStorePath` and `ReadStoredSummary`):
    - Line `27-29`: `os.Stat(dbPath)` returns a raw `os.ErrNotExist` error when `$XDG_DATA_HOME/harnez/telemetry.sqlite` does not exist yet.
    - Line `35-39`: calls `store.LatestUsageSummary(ctx)` (`internal/usagestore/store.go:138-151`); when `err == sql.ErrNoRows`, line `38` returns `fmt.Errorf("usage store has no collector snapshots yet")`.
  - `internal/usagecli/client.go:48-70` (`LoadLocalUsageWithStatus`):
    - Line `57-61`: when `!offline`, calls `EnsureLocalServer(ctx, cfg)` (`client.go:35-41,99-116`), which starts a detached server (`startDetachedServer` at `client.go:170-187`) if none is listening and returns `status = "server starting"` (printed to stderr by `reportUsageServerStatus` in `cmd/harnez/usage_cmd.go:19-21`).
    - Line `62-65`: currently only translates `os.IsNotExist(err)` when `offline == true` (`"usage store does not exist; start harnez server first (or run usage without --offline)"`), leaving non-offline missing DB file errors as raw `stat .../telemetry.sqlite: no such file or directory`.
  - `cmd/harnez/usage_test.go`: existing CLI tests for `usage` (`TestUsageJSONCommandReadsStoredSummary`, `TestUsageMinimalCommand`, `TestUsageOfflineMissingStoreErrorsClearly`, etc.).
- **Expected Behaviour**:
  1. Set `SilenceUsage: true` on `usageCmd` in `cmd/harnez/usage_cmd.go:41` (and on `historyCmd` / `historyRecordCmd` / `historyExportCmd` at `cmd/harnez/usage_cmd.go:254-338`) so runtime store/collector errors never print the full command help/flags block, while `root.Execute()` returns a non-nil error (exit code `1` via `exitCodeFromRunError` in `cmd/harnez/exitcode.go:47-60`).
  2. In `internal/usage/readstore.go:23-48` and `internal/usagecli/client.go:48-70`, export a sentinel error (e.g. `ErrNoCollectorSnapshots = errors.New("usage store has no collector snapshots yet")`) and ensure both `sql.ErrNoRows` (`readstore.go:37-39`) and non-offline missing DB file (`os.IsNotExist(err)` in `client.go:62-65`) produce an actionable message that includes remediation (`run 'harnez server install' or wait for first collection`) instead of a bare error or raw `stat` path error. Preserve the existing `--offline` message at `internal/usagecli/client.go:62-64` tested by `TestUsageOfflineMissingStoreErrorsClearly` (`cmd/harnez/usage_test.go:194-208`).
- **Acceptance Tests (Sandbox)**:
  - Add table/unit tests in `cmd/harnez/usage_test.go` covering `usage`, `usage --compact`, `usage --agent codex`, and `usage --json` against an empty store (both an initialized empty SQLite DB with 0 snapshots and a missing DB file):
    - Assert `root.Execute()` returns a non-nil error and `exitCodeFromRunError(err) == 1`.
    - Assert error/stderr output contains `"usage store has no collector snapshots yet"` and `"harnez server install"`.
    - Assert stdout + stderr do **not** contain `"Usage:\n"` or `"Flags:\n"`.

---

### Item 2: Codex window duration labeling (`Monthly` vs `Weekly` vs `%d-Hour`) & Free-plan single-window routing
- **Environment**: **Sandbox-doable** for code and unit tests; **Host-canary** required only to verify against a live ChatGPT Free account.
- **Verified Code Locations**:
  - `internal/usage/codex.go:18-31`: `CodexAuth` and `CodexTokens` (`IDToken`, `AccessToken`, `AccountID`) are **already implemented** on this checkout (commit `2c8f0ef8`), with regression test `TestCollectCodexParsesNestedTokenFields` in `internal/usage/codex_test.go:256-293`. Verify they remain intact; do not rewrite them.
  - `internal/usage/codex.go:64-69` (`CodexRateWindow`): fields are `UsedPercent float64` (`json:"used_percent"`), `LimitWindowSeconds int64` (`json:"limit_window_seconds"`), `ResetAfterSeconds int64` (`json:"reset_after_seconds"`), and `ResetAt int64` (`json:"reset_at"`).
  - `internal/usage/codex.go:175-206` (`buildCodexQuotaWindow(pw *CodexRateWindow, now time.Time) QuotaWindow`):
    - Currently sets `windowName := "Weekly"` (`line 178`) and only overrides it when `pw.LimitWindowSeconds > 0 && pw.LimitWindowSeconds < 86400` (`line 179-181`) to `fmt.Sprintf("%d-Hour", pw.LimitWindowSeconds/3600)` (e.g. `"5-Hour"`, tested in `internal/usage/codex_test.go:21`).
    - Anything with `LimitWindowSeconds >= 86400` (such as a 30-day window of ~`2592000` seconds) or `LimitWindowSeconds == 0` with a reset ~29–30 days out falls through to `"Weekly"`.
  - **Downstream window routing and rendering (CRITICAL — update together so `"Monthly"` is not misrouted to `Session` / `"5h"`)**:
    1. `internal/usage/codex.go:368-380` (`CollectCodex`):
       ```go
       qw := buildCodexQuotaWindow(pw, now)
       if qw.Name == "Weekly" {
           usage.Weekly = &qw
       } else if usage.Session == nil {
           usage.Session = &qw
       }
       ```
       If `qw.Name == "Monthly"` is not matched alongside `"Weekly"`, `CollectCodex` will place the Free-plan monthly window into `usage.Session`! Route both `"Weekly"` and `"Monthly"` (e.g. `qw.Name == "Weekly" || qw.Name == "Monthly"`) to `usage.Weekly` (the long-horizon quota slot on `AgentUsage` in `internal/usage/usage.go:30-31`), leaving `usage.Session == nil` when the Free plan has no 5-hour window.
    2. `internal/usagestore/store.go:1103-1137` (`NormalizeWindowKey`): maps `"week"` / `"7-day"` to `"weekly"`; for `"Monthly"` it currently falls through to the alphanumeric normalizer (`"monthly"`), or can explicitly map `strings.Contains(name, "month") || strings.Contains(name, "30-day")` to `"monthly"`.
    3. `internal/usage/store.go:600-607` (`applyStoredWindows`):
       ```go
       name := strings.ToLower(r.Name)
       if r.Key == "weekly" || strings.Contains(name, "week") || strings.Contains(name, "7-day") {
           agent.Weekly = &w
       } else if agent.Session == nil {
           agent.Session = &w
       }
       ```
       Include `r.Key == "monthly" || strings.Contains(name, "month") || strings.Contains(name, "30-day")` in the `agent.Weekly` branch so reading a single stored `"Monthly"` window back from SQLite does not move it into `agent.Session`.
    4. `internal/usage/usagetable.go:151-195` (`usageTableWindows`, `quotaWindowKind`, `quotaWindowKindOrder`):
       - At lines `153-155`, `usageTableWindows` currently hardcodes `out = append(out, namedWindow{label: "weekly", w: *agent.Weekly})`. If `agent.Weekly.Name` is `"Monthly"` (or `quotaWindowKind(agent.Weekly.Name) == "monthly"`), use `"monthly"` (or preserve `"weekly"` for weekly windows) and support `"monthly"` in `quotaWindowKind` (`usagetable.go:174-183`) and `quotaWindowKindOrder` (`usagetable.go:187-195`).
    5. `internal/usage/watch_usage_box.go:287,339,515` (`firstAllUsageWindow`, `formatAllUsageTableLineWithMidWidth`, `formatCompactGroupLineWithLabelWidth`): checks `strings.Contains(name, "week") || strings.Contains(name, "7-day")` when ordering/selecting windows; include `strings.Contains(name, "month")` so compact/watch views treat `"Monthly"` as a long window.
- **Expected Behaviour**:
  - In `buildCodexQuotaWindow` (`internal/usage/codex.go:177-206`):
    - Short windows (`LimitWindowSeconds > 0 && LimitWindowSeconds < 86400`): keep `fmt.Sprintf("%d-Hour", pw.LimitWindowSeconds/3600)` (e.g. `"5-Hour"`).
    - Monthly windows (`LimitWindowSeconds >= 14*86400`, or `LimitWindowSeconds <= 0` when computed `durationLeft > 14*24*time.Hour`): label `"Monthly"`.
    - Weekly windows (all other multi-day windows, e.g. ~7 days): label `"Weekly"`.
  - When a Free-plan response provides only a single ~30-day window (`primary_window` ~30d, `secondary_window: nil`), `CollectCodex` sets `usage.Weekly` to that `"Monthly"` window and leaves `usage.Session == nil` (no synthetic 5-hour window), and round-tripping through `PersistUsageSummary` / `ReadStoredSummary` preserves `usage.Weekly.Name == "Monthly"` and `usage.Session == nil`.
  - If a plan returns no usable windows or an unparseable rate-limit payload, preserve explicit error/status reporting on `AgentUsage` without failing the overall store write.
- **Acceptance Tests (Sandbox)**:
  - In `internal/usage/codex_test.go`:
    1. `TestBuildCodexQuotaWindowLabelsByDuration`: table cases covering 5-hour (`LimitWindowSeconds: 18000` -> `"5-Hour"`), 7-day (`LimitWindowSeconds: 604800` -> `"Weekly"`), 30-day (`LimitWindowSeconds: 2592000` -> `"Monthly"`), and `LimitWindowSeconds: 0` with `ResetAt` ~29 days out -> `"Monthly"`.
    2. `TestCollectCodexFreePlanSingleMonthlyWindow`: mock `/backend-api/wham/usage` response with `plan_type: "free"`, `primary_window` at `0%` used resetting ~29 days out (`limit_window_seconds: 2592000`), and `secondary_window: null`. Assert `Authenticated == true`, `PlanTier == "Free"`, `usage.Session == nil`, `usage.Weekly != nil`, `usage.Weekly.Name == "Monthly"`, and store round-trip (`PersistUsageSummary` + `ReadStoredSummary`) preserves `Weekly.Name == "Monthly"` and `Session == nil`.
  - In `internal/usage/usagetable_test.go`:
    3. Test that a Codex `AgentUsage` with a single `Monthly` window renders the `monthly` label and does not emit a `5h` row.
- **Host Canary Verification**:
  - Run `make install` and `harnez usage --agent codex` / `harnez usage --agent codex --json` on the host logged into a live ChatGPT Free account; confirm the single ~29-day window is labeled `Monthly` (or `monthly` in the table) and no 5-hour window is emitted.

---

### Item 3: Systemd `ReadWritePaths`, collector startup snapshot when store is empty, and non-destructive stray server detection
- **Environment**: **Sandbox-doable** for unit file, directory creation, collector startup logic, stray server reporting, and unit tests; **Host-canary** required for live `systemctl --user` start and socket check.
- **Verified Code Locations**:
  1. **Systemd unit & install directory creation**:
     - `systemd/harnez-agent-collector.service:21` (embedded via `embed.go`, configured in `spec/server.yaml:16-17` as `systemd_unit_source: systemd/harnez-agent-collector.service`):
       Currently `ReadWritePaths=%t/harnez %S/harnez %C/harnez %h/.local/share/harnez`. Because entries are not prefixed with `-`, systemd fails with `status=226/NAMESPACE` if any path (such as `%C/harnez` = `~/.cache/harnez`) does not exist yet.
     - `internal/server/systemd.go:110-142` (`SystemdService.Install`):
       Currently only calls `os.MkdirAll(s.UnitDir, 0755)` (`line 128`). It does not ensure the XDG directories referenced by `ReadWritePaths=` exist (`xdgpath.CacheHome()` at `internal/xdgpath/xdgpath.go:34-39`, `xdgpath.DataHome()` at `internal/xdgpath/xdgpath.go:25-30`, plus `~/.local/state/harnez` / `$XDG_STATE_HOME/harnez` and `$XDG_RUNTIME_DIR/harnez` when set).
     - `internal/server/systemd_test.go:46-93` (`TestSystemdServiceInstallRendersUnitAndCallsSystemctl`): line `79` checks for `"ReadWritePaths=%t/harnez"` in the rendered unit file and must be updated if `ReadWritePaths=` entries are prefixed with `-`.
  2. **Why a running server had 0 snapshots after 4+ minutes**:
     - `cmd/harnez/server_cmd.go:523-537` (`runHarnezServer`) creates `tracker := usage.NewClientTracker(clientTimeout)` and passes it to `usage.RunCollectorWithRequestsAndTracker` (`cmd/harnez/server_cmd.go:37-39`).
     - In `internal/usage/collector.go:170-191` (`runCollectorWithDeps`):
       ```go
       if initial {
           initial = false
           // If a tracker is present and has no active clients at startup,
           // skip the immediate initial fetch per issue 714 M3.
           if tracker != nil && tracker.Count() == 0 {
               timer := time.NewTimer(interval)
               ...
       ```
     - Only `harnez usage --watch` (`internal/usage/watch.go:780`, `StartClientHeartbeat`) sends `/usage/heartbeat` requests. One-shot `harnez usage` never sends a heartbeat, so `tracker.Count()` is `0` at server startup, causing the collector to sleep for `interval` (`collector_cadence: 5m` in `spec/usage.yaml:16`) even when `dbPath` has **zero** snapshots in `usage_current_summaries`!
     - Note existing test `TestCollectorSkipsInitialFetchWhenNoClients` in `internal/usage/catchup_test.go:71-105`: it passes an empty `dbPath` (`filepath.Join(home, "usage.sqlite")`) with `tracker.Count() == 0` and asserts `expected 0 initial fetches without clients`.
     - Note pre-existing race in `TestCollectCatchupFetchesOnlyProvidersOlderThanThreshold` (`internal/usage/catchup_test.go:18-41`): `fetched := map[string]int{}` is mutated concurrently by `CollectCatchup` provider goroutines (`internal/usage/registry.go`) without synchronization.
  3. **Stray server detection & reporting (MUST NOT kill processes)**:
     - `internal/server/server.go:111-114` (`claimExisting`): when a new `harnez server` starts and the socket is already owned by a `harnez server` with the same version and build identity, `claimExisting` returns `errAlreadyRunning`, so the new server exits `0` immediately while the old process keeps the socket.
     - `cmd/harnez/server_cmd.go:95-112` (`newServerInstallCmd`): calls `server.NewSystemdService(cfg).Install(cmd.Context())` and unconditionally prints `installed and started harnez-agent-collector.service` without checking `queryLocalServerStatus` (`server_cmd.go:470-488`) or `SystemdService.Status` (`internal/server/systemd.go:219-239`) to see if a stray non-systemd PID (`SocketPID > 0 && SocketPID != MainPID`) holds the socket.
     - `cmd/harnez/server_cmd.go:133-157,412-443` (`newServerStatusCmd` and `printServerSystemdStatus`):
       - When `status.Installed && status.SocketPID > 0 && status.SocketPID != status.MainPID` (`lines 436-441`), it currently suggests only `systemctl --user restart harnez-agent-collector.service` (which cannot take over the socket if the stray server has the same build identity; `harnez server stop` must be run first).
       - When `!status.Installed && status.SocketPID > 0`, `printServerSystemdStatus` does not warn that an unmanaged/stray server process (`PID <SocketPID>`) owns the socket.
- **Expected Behaviour**:
  1. **Systemd unit & XDG directories**:
     - Prefix every path in `systemd/harnez-agent-collector.service:21` with `-` (`ReadWritePaths=-%t/harnez -%S/harnez -%C/harnez -%h/.local/share/harnez`) so missing directories never abort mount namespace setup.
     - In `SystemdService.Install` (`internal/server/systemd.go:110-142`), also create the harnez cache, data, and state directories (`xdgpath.CacheHome()`, `xdgpath.DataHome()`, and `$XDG_STATE_HOME/harnez` / `~/.local/state/harnez`) with `os.MkdirAll(..., 0755)`.
  2. **Initial snapshot when store has no snapshots**:
     - Ensure a newly started server writes an initial snapshot immediately when the usage store at `dbPath` has no existing collector snapshot (`_, err := ReadStoredSummary(ctx, dbPath); err != nil` or `dbPath == ""`), while still skipping the immediate startup fetch when `tracker != nil && tracker.Count() == 0` and a stored snapshot already exists (per issue 714 M3).
     - Update `TestCollectorSkipsInitialFetchWhenNoClients` in `internal/usage/catchup_test.go:71-105` to seed an existing snapshot in `dbPath` (via `PersistUsageSummary`) before testing the no-client skip, and add a new test verifying that an empty store **does** perform the initial collection at startup even when `tracker.Count() == 0`.
     - Synchronize the `fetched` map in `TestCollectCatchupFetchesOnlyProvidersOlderThanThreshold` (`internal/usage/catchup_test.go:18-41`) with a `sync.Mutex` so `go test -race ./internal/usage/...` passes cleanly.
  3. **Detect and report stray server (NO process killing)**:
     - Do **not** call `os.Process.Kill`, `syscall.Kill`, or stop the server automatically during `install` or `status`.
     - In `printServerSystemdStatus` (`cmd/harnez/server_cmd.go:412-443`) and `newServerInstallCmd` (`cmd/harnez/server_cmd.go:95-112`), when `SocketPID > 0 && SocketPID != MainPID` (including when `Installed == false` and `SocketPID > 0`, or after `Install` when an existing non-systemd PID still owns the socket), clearly report the stray server PID (`socket owner: stray process (PID <pid>)`) and instruct the user to run `harnez server stop` before `systemctl --user restart harnez-agent-collector.service`.
- **Acceptance Tests (Sandbox)**:
  - `internal/server/systemd_test.go`: verify `Install` renders `ReadWritePaths=-%t/harnez -%S/harnez -%C/harnez -%h/.local/share/harnez` and creates the XDG cache/data/state directories under temporary `HOME`/`XDG_*` roots.
  - `internal/usage/catchup_test.go` / `internal/usage/collector_test.go`:
    - `TestCollectorInitialFetchWhenStoreEmptyEvenWithoutClients`: when `dbPath` has no snapshot and `tracker.Count() == 0`, collector performs 1 immediate initial fetch and persists a snapshot.
    - `TestCollectorSkipsInitialFetchWhenNoClients`: when `dbPath` already has a stored snapshot and `tracker.Count() == 0`, collector skips the immediate startup fetch.
    - `go test -race ./internal/usage/...` passes with 0 data races.
  - `cmd/harnez/server_service_test.go`: test `harnez server status` and `harnez server install` output when `SocketPID != MainPID` (stray process holds the socket), asserting the stray PID and `harnez server stop` remediation are reported and no process is killed.
- **Host Canary Verification**:
  - Run `make install`, `harnez server install`, and `harnez server status` on the host; verify `harnez-agent-collector.service` starts cleanly without `226/NAMESPACE` and writes an initial snapshot within seconds.

---

### Item 4: Connect stored usage/auth state to `harnez agent models --picks` & fallback (Drop to 992 if short)
- **Environment**: **Sandbox-doable** (code + unit tests). **Drop-if-short rule applies**: if Items 1–3 consume the job budget or this item hits policy ambiguity with 992, drop Item 4 to ticket 992 and document in the ticket.
- **Boundary with Tickets 942/954/955 and 992**:
  - **942 / 954 / 955** own `$XDG_STATE_HOME/harnez/models/access.yaml` (`access denied` probing and persistence, already wired in `internal/subagent/driver.go:484-493,507-509`).
  - **992** owns `$XDG_CONFIG_HOME/harnez/models.yaml` per-provider plan preferences (`plan: own | free | shared`, deprioritizing `free` plans in role chains and skipping `free` providers when quota is low or `unknown`). Do **not** implement `models.yaml` `plan:` parsing in 991.
  - **991 Item 4** is strictly the usage-store availability bridge so `providerAvailability` reflects when a provider in the latest collector snapshot is unauthenticated or quota-exhausted.
- **Verified Code Locations**:
  - `cmd/harnez/agent_cmds.go:177,231-235` (`cachedModelPicks` and `writeModelPicks`):
    - `cachedModelPicks()` calls `subagent.ModelPicksWithAvailability` (`internal/subagent/agentspec.go:169-178`) with:
      ```go
      func(m subagent.Model) bool {
          return providerAvailability(m.Provider, m.Name).State == "exhausted"
      }
      ```
    - `writeModelPicks` (`cmd/harnez/agent_cmds.go:177`) prints `providerAvailability(pick.Selection.Model.Provider, pick.Selection.Model.Name).State` in the `AVAILABILITY` column.
  - `cmd/harnez/agent_run.go:198-200,215-231` (`providerAvailability` and `selectStartSelection`):
    - Calls `usage.CachedProviderQuotaAvailability(provider, model, time.Now())` (`internal/usage/statecache.go:66-79`), which delegates to `ProviderQuotaAvailabilitiesFromStore(ctx, dbPath, provider, models, now)` (`internal/usage/statecache.go:89-163`).
  - `internal/usage/statecache.go:89-163` (`ProviderQuotaAvailabilitiesFromStore`):
    - Queries `windows, err := store.CurrentFor(ctx, provider)` (`line 99`).
    - Marks `state.Exhausted = true` and `state.State = "exhausted"` when a matched window has `window.UsedFraction >= 1 && window.ResetAt != nil && window.ResetAt.After(now)` (`line 139`).
    - **Bug 1**: If a window has `window.UsedFraction >= 1` (`100%` used) and is fresh (`now.Sub(window.ObservedAt) <= AgentQuotaAvailabilityMaxAge()`), but `window.ResetAt == nil`, line `139` fails to mark it `"exhausted"` and instead marks it `"available"` at line `156`!
    - **Bug 2**: When a provider's latest snapshot in `store.LatestUsageSummary(ctx)` (`internal/usagestore/store.go:138-151`, persisted by `PersistUsageSummary` in `internal/usage/collector_store.go:15-54`) has `Installed == true && Authenticated == false` (e.g. logged out or broken `auth.json`), `importAgent` (`internal/usage/store.go:812-849`) writes 0 quota windows, so `store.CurrentFor(ctx, provider)` returns 0 rows and `ProviderQuotaAvailabilitiesFromStore` returns `State: "unknown"`. Because `State` is `"unknown"`, `cachedModelPicks()` and `selectStartSelection()` still pick `codex` first instead of falling back to the next candidate in the role chain!
  - `cmd/harnez/agent_picks_test.go:13-79` and `cmd/harnez/testdata/agent-picks.golden`:
    - `TestModelPicksGolden` (`lines 13-30`) and `TestModelPicksCommand` (`lines 32-79`) test `harnez agent models --picks` when no usage store exists (`State == "unknown"`), expecting `developer/smart` -> `codex:sol:med` with `AVAILABILITY == "unknown"`. Keep `State == "unknown"` when the store is missing or has no snapshot so `TestModelPicksCommand` and `agent-picks.golden` continue to pass!
- **Expected Behaviour**:
  - In `ProviderQuotaAvailabilitiesFromStore` (`internal/usage/statecache.go:89-163`):
    1. Treat a matched quota window with `window.UsedFraction >= 1` as exhausted when either `window.ResetAt != nil && window.ResetAt.After(now)` **or** `window.ResetAt == nil && now.Sub(window.ObservedAt) <= AgentQuotaAvailabilityMaxAge()`.
    2. When `seen == false` (no current quota windows for `provider`), check `store.LatestUsageSummary(ctx)`: if the snapshot is fresh (`now.Sub(record.ObservedAt) <= AgentQuotaAvailabilityMaxAge()`) and the matching `AgentUsage` entry for `provider` has ` !agent.Installed` or `!agent.Authenticated`, mark `state.Exhausted = true` and `state.State = "exhausted"` (or an unavailable state that `cachedModelPicks` and `selectStartSelection` treat as unavailable so `ResolveModelWithAvailability` falls back to the next candidate in the chain).
    3. When the usage store does not exist or has no snapshots, continue returning `State: "unknown"` (`Exhausted: false`).
- **Acceptance Tests (Sandbox)**:
  - In `internal/usage/statecache_test.go` (or `cmd/harnez/agent_fallback_test.go`):
    1. Store a snapshot via `PersistUsageSummary` where `codex` has a single `Monthly` window at `100%` used (`Remaining: 0`, `UsedPercent: 100`): assert `ProviderQuotaAvailabilityFromStore` returns `State == "exhausted"` and `ResolveModelWithAvailability` / `cachedModelPicks` skips `codex` for `developer/smart` (falling back to `claude:opus:high`) with fallback reason `"quota exhausted"`.
    2. Store a snapshot via `PersistUsageSummary` where `codex` has `Installed: true, Authenticated: false`: assert `ProviderQuotaAvailabilityFromStore` marks `codex` unavailable/exhausted and `cachedModelPicks` / `selectStartSelection` falls back from `codex` to `claude`.
    3. Verify `go test ./cmd/harnez -run 'TestModelPicks|TestSelectStartSelection'` passes, keeping `cmd/harnez/testdata/agent-picks.golden` intact.

---

Make sure our goals in issues/991-harnez-usage-fails-with-no-collector-snapshots-and-cannot-show-codex-free-plan-quota.md are actually completed (`harnez usage` exits non-zero without dumping help when the store has no snapshots; Codex free-plan single ~30-day window is labeled `Monthly` with no phantom 5-hour window; `harnez server install` prefixes `ReadWritePaths=` with `-` and creates XDG dirs, collector writes an initial snapshot when the store is empty, and stray non-systemd servers are detected and reported without killing processes; and if not dropped to 992, exhausted/unauthenticated providers in the usage store trigger `--picks` fallback). Only proceed from one phase to the next once confirmed. If you get blocked by an error or anything that makes it impossible to reach the goal, stop and record a Failure Report in the ticket instead of guessing.

<!-- GOAL_COMPLETE -->

