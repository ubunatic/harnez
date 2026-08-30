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

## Web research (2026-08-30)

External evidence gathered, not local instrumentation — this narrows the
hypothesis but does not confirm or rule it out for this machine.

- `agy`/Antigravity authenticates via interactive Google OAuth only,
  tokens cached in the OS keyring; there's no service-account/API-key
  path for the CLI (only the SDK has one; see
  `google-antigravity/antigravity-cli` issue #78, a still-open feature
  request for headless-environment auth).
- **Confirmed, documented failure mode independent of any bot-detection
  system**: OAuth tokens expire after days–weeks; reusing an interactive
  session for unattended/scheduled runs works until expiry, then the CLI
  hangs or throws a re-login / "Further action is required" dialog. See
  antigravitylab.net, "When the Antigravity CLI Stalls on a 401 During
  Unattended Runs," and multiple Google AI Developer Forum threads
  describing the same login-loop symptom.
- Google does run bot/abuse detection on this OAuth surface: the sibling
  Gemini CLI's own GitHub discussion ("Service update: mitigating abuse
  and prioritizing traffic," google-gemini/gemini-cli#22970) confirms an
  active abuse-detection rollout, with users reporting accounts
  incorrectly throttled as a side effect (gemini-cli#24059). The exact
  triggering heuristic (frequency, non-interactive pattern, etc.) is not
  publicly documented.
- **No public source ties polling cadence specifically (e.g. once every
  15 minutes) to triggering reauth or bot detection.** The link is
  plausible — a scheduled, unattended, periodic invocation on a
  consumer OAuth token is exactly the shape abuse heuristics look for,
  and token-expiry-during-unattended-run reproduces the same *symptom*
  (a login dialog appearing) regardless of any detection system — but
  it is not proven as *this* machine's cause.
- Community mitigation pattern for unattended OAuth-based CLI use:
  redirect stdin so a stuck reauth prompt fails fast instead of hanging;
  treat a 401/interactive-login response as a hard signal to back off
  polling rather than retry; avoid reusing one long-lived token across
  many scheduled invocations if an alternative exists.

This reframes item 5 above: even without a confirmed causal link, "treat
any interactive-login/401 output from `agy -p "/usage"` as a signal to
back off polling rather than retry" is a low-cost, evidence-backed
mitigation worth applying regardless, alongside issue 111's per-agent
cadence work.

## Acceptance Criteria

- A clear determination (with timestamp evidence, not speculation)
  of whether `agy -p "/usage"` polling correlates with, or plausibly
  causes, the observed Google login/reauth dialogs.
- If causal: a follow-up fix ticket filed with the specific mitigation
  chosen.
- If not causal / inconclusive: this ticket documents why, and is
  closed with that reasoning recorded for future reference.
