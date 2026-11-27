# 1311 — Taia session data as an optional usage source plugin

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: #1309 (taia Claude dir via claude_config_dirs), #1310 (web board), #1177 (public packages for taia)

---

## 1. Problem & Motivation

Since #1309, harnez reads taia's Claude transcripts from the config dir list, so cost and tokens
of taia sessions are counted. taia holds more per session that the board (#1310) could show:
driver, repo, task, response count, its own session id (d957448b is taia session 9210f73a). The
owner wants harnez to detect taia sessions and collect that data through an optional plugin,
without tying taia to harnez or harnez to taia.

## 2. Technical Specification / Findings

- taia sessions run confined; no hooks into harnez (owner, 1309 assessment).
- Detection candidates (unverified): the session's transcript lives under a configured
  `claude_config_dirs` entry that a plugin claims, or a taia marker in the transcript metadata.
- Plugin contract candidates (decide in M1 plan, record under Decision):
  a. external command declared in spec/local.yaml, e.g. `taia usage-export --json`, read as data;
  b. read taia's exported `data.json` schema from a configured path;
  c. a generic "session metadata source" interface in harnez with taia as the first adapter.
  All keep taia optional: harnez works unchanged when the plugin is absent or fails (a `note:`).
- Join key: Claude session id to taia session id; taia data never overrides harnez cost.
- Privacy: import ids, names, counts and timestamps only; never prompts or summaries.
- Config values (plugin enable, command, path) in `spec/` with a schema; taia's path only in
  the user's local.yaml, never in repo docs (as in #1309).

## 3. Implementation & Verification Plan

/goal harnez shows taia metadata (driver, repo, taia session id) for detected taia sessions in
the usage report and the #1310 board through an optional plugin, and runs unchanged without it;
or stop and report when blocked on an owner decision, a denied permission, or no progress
(circuit breaker: clear the goal and write a failure report here).

Check live code and taia's current export first; this ticket may be stale. Reading taia is
allowed; any change in the taia repo needs the owner's OK.

- **M1 (plan, read-only):** pick the contract (a/b/c) and the join key, verified against taia's
  real export schema; record under Decision.
- **M2:** plugin and detection with tests on fake data (plugin absent, failing, malformed,
  matching); SOURCE/metadata columns in `harnez usage report`.
- **M3:** taia fields as filters and labels on the #1310 board.
- Acceptance: live report shows d957448b joined to taia 9210f73a; with the plugin disabled the
  report is byte-identical to before.
