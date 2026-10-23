# 710 — AGY usage quota rows remain gray while AGY is active

**Status**: Closed — gray rows fixed by 714 staleness grace; agy statusline drops were quota-less renders (11 accepted, 9 without quota live), counter renamed passive_no_quota
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 714, [649](649-harnez-usage-compact-no-longer-shows-agy-gemini-quota-rows.md), [101](archive/101-usage-keep-stale-agents-visible-until-7d.md)

---

## 1. Problem & Motivation
User report (2026-10-05): `Agy-Gemini` and `Agy-Claude` usage often remain grayed out in the `harnez usage` TUI even while AGY is active. The user cannot tell whether the displayed quota data is fresh.

/goal Investigate and resolve persistent gray AGY quota rows while AGY is active, and make the freshness of displayed data clear; verify the resulting behavior, or stop and report when blocked on a user decision, required authentication action, or denied permission.

## 2. Technical Specification / Findings
The cause is unconfirmed. Check whether quota refresh is failing, stale fallback is being shown correctly, or the TUI incorrectly marks fresh data as stale. AGY process activity alone does not establish quota freshness. Related closed issues addressed missing rows and old meter readings; recheck live code and recent commits before implementing.

## 3. Implementation & Verification Plan
- Reproduce with active AGY and inspect the actual quota timestamps and refresh outcome for both model groups.
- Correct any confirmed refresh or stale-rendering defect and add meaningful regression coverage.
- Ensure the TUI communicates when quota data was last refreshed and why stale fallback remains, so color alone does not leave freshness ambiguous.
- Verify fresh and stale states for both rows; record the cause and any remaining upstream limitation.

## 4. Recurrence (2026-10-22)

User reports rows "often" turning gray in the live `harnez usage` view, together with the
`unavailable (signal: killed)` errors tracked in 714. Likely the same root cause (collector
killed → stale snapshot → gray); fix 714 first and re-check this one.

## Live check (2026-10-22, lean sprint): not reproducible today

`harnez usage --raw` showed fresh Claude, Codex and both AGY pools. Gray comes from either a `QuotaFetchError`
(`internal/usage/types.go:160`) or snapshot age (`internal/usage/freshness.go:5`); the display does not say
which. Way forward: show the reason on a gray row (fetch error text or age), keep the last good quota on a
failed refresh, test fresh/stale/failed rendering. Then park until the next live occurrence, which the reason
text will explain.

## Live data (2026-10-23, after 714 M5)

`harnez server status --json` shows `passive_dropped.agy: 24` (since server start) while the user's AGY
sessions display quota in their status line: `45k/1049k (95% cached) · 5h 31% left · argon-wtu-weekly 43.3%
left · weekly 19.6% left`.

Correction (same day): harnez renders that text itself (`spec/statusline.yaml`, `.Quotas` from
`quotaText`, `internal/statusline/statusline.go:162`), from the payload's `quota.<name>.remaining_fraction`,
the same field and JSON tag `passiveWindows` reads (`internal/usage/passive.go:25-29,108`). So payloads that
show quota should be accepted; the drops (40 by 14:15) are likely renders without a `quota` field. There is no
accepted counter, so this is unproven. Next: count accepted observations next to `passive_dropped`, and
capture one dropped payload (env-gated stdin dump) to see what it carries.

## Passive-path findings (2026-10-23)

- No Harnez quota parsing defect reproduced. `cmd/harnez/statusline.go:46` observes the
  original stdin bytes with provider `agy`. `internal/usage/passive.go:63` posts
  `{"dropped_provider":"agy"}` only when there are zero usable quota windows;
  `internal/usage/passive_endpoint.go:52` is the sole drop-counter increment.
  Transport/HTTP errors return `UpdateDropped` locally but do not increment that counter.
  Passive parsing has no active-model filter; the renderer does. Source precedence can
  also keep API readings visible after accepted statusline observations.
- A realistic Gemini payload with `gemini-5h`, `gemini-weekly`, and
  `gemini-argon-wtu-weekly` fractions renders the reported text and forwards three
  windows: zero drops, one accepted observation. The live cause remains unproven;
  capture is needed to establish what the dropped renders contain.
- Commits: `e496aa17` adds opt-in capture and `passive_accepted` per-provider counts
  beside `passive_dropped`; `2da01455` adds the command-to-Unix-HTTP AGY regression.
  Acceptance is counted once per provider/request after successful collector completion,
  including deduplicated successful writes; both counters reset at server start.
- Capture: export `HARNEZ_CAPTURE_PASSIVE_DROPS=1` before launching AGY, or prefix its
  configured command with `HARNEZ_CAPTURE_PASSIVE_DROPS=1 harnez statusline --agent agy`.
  The variable belongs in the **statusline process** environment. The latest drop is
  atomically saved with mode 0600 to `$XDG_STATE_HOME/harnez/passive-last-dropped.json`
  (fallback `~/.local/state/harnez/passive-last-dropped.json`). Fields: `provider`,
  `reason`, original stdin as the JSON string `payload`, and `captured_at`.
  Parse, missing/invalid quota, configuration, transport, and HTTP errors are captured.
  Unset the variable to stop capture; accepted renders leave the last drop untouched.
- Verification: `go test ./internal/usage/... ./internal/statusline/... ./cmd/harnez/...
  ./internal/server/...` passed; `make install` passed; installed-binary capture canary
  passed with an isolated missing socket. No live daemon restart performed: the host
  must restart onto the installed build for acceptance counts and verify live AGY data.
  Ticket remains open.

## Live capture (2026-10-23 14:39, after e496aa17)

After a server restart and one fresh agy session with one prompt: `passive_accepted.agy: 11`,
`passive_dropped.agy: 9`. The captured dropped payload (agy 1.4.6, `agent_state: "idle"`) has no `quota` key
at all; reason "no usable quota windows". So agy sends quota on some renders only, harnez accepts those, and
the drops are quota-less renders: harmless. Capture switch removed from the agy settings again.
