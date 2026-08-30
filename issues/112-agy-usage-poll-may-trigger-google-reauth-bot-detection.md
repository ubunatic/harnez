# 112 — Investigate whether `agy -p "/usage"` polling triggers Google reauth / bot-detection dialogs

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [[104-agy-quota-collector-requires-live-process-poll-coincidence]], [[111-per-agent-collector-pipelines-independent-cadence-and-timeout]], `internal/usage/agy.go`

## Problem

The user has observed several Google account login/reauth dialogs
appearing recently, and suspects harnez's periodic `agy -p "/usage"`
polling (the fetch mechanism adopted in issue 104 to read AGY quota
without requiring a live listening process) may be triggering a
reauthentication flow or Google's bot/automation detection on
Antigravity's backend — e.g. from being invoked repeatedly and
unattended by a background collector, in a pattern that doesn't look
like normal interactive CLI usage.

This is not yet confirmed — it's an observed correlation the user wants
investigated before it's treated as a real causal effect.

## What to investigate

1. Correlate timestamps: when did the Google login dialogs appear versus
   when `harnez agent-collector` / `harnez usage --watch` actually ran
   `agy -p "/usage"` (collector-daemon logs, `~/.claude/harnez/` state
   dir timestamps, shell history/process accounting if available).
2. Check whether `agy -p "/usage"`'s own logs
   (`~/.gemini/antigravity-cli/log/cli-*.log`) show any auth-related
   activity, token refresh, or warnings coinciding with those polls.
3. Determine current polling cadence/volume in practice: default
   collector interval (`DefaultCollectorInterval` = 900s) plus any
   foreground `--watch` sessions running concurrently — is this within
   normal single-user CLI usage patterns, or does the *aggregate* poll
   rate (multiple watchers/daemons, if more than one has been running)
   look automation-like from Google's side?
4. Check issue 104's addendum note: the canary probe there confirmed
   `/usage` doesn't consume model quota, but did not check for any
   auth-token or session-related side effects of repeated invocation —
   revisit with that specific lens.
5. If a real link is found, evaluate mitigations: lower polling
   frequency for AGY specifically (see issue 111's per-agent cadence
   work — AGY could poll less often), add a minimum spacing guard
   across concurrent harnez processes (daemon + `--watch` foreground)
   so `agy` isn't invoked back-to-back from two sources, or drop back to
   passive on-disk state reads (issue 104's investigated-but-unresolved
   alternative) if Google's backend penalizes active CLI polling.
6. If no correlation is found, document that explicitly here and close.

## Acceptance Criteria

- A clear determination (with timestamp evidence, not speculation)
  of whether `agy -p "/usage"` polling correlates with, or plausibly
  causes, the observed Google login/reauth dialogs.
- If causal: a follow-up fix ticket filed with the specific mitigation
  chosen.
- If not causal / inconclusive: this ticket documents why, and is
  closed with that reasoning recorded for future reference.
