# 1079 — harnez read guard: allow native Read with offset/limit; harnez read -L range failure from haiku worker

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 696

---

## 1. Problem & Motivation
1. The PreToolUse read guard blocks native Read of files over 100 lines and points to `harnez read`. An opus reviewer then fell back to `cat -n`: friction with no token saving.
2. `harnez read` with a line range failed once for a claude haiku:med worker (s3-dev); it was rated via `harnez rate` (details in the rate log).

## 2. Technical Specification / Findings
- A ranged native Read (offset/limit) is already bounded ingestion; blocking it only pushes agents to unguarded shell reads.
- Option: allow Read when offset/limit is set and the range is under the threshold; consider relaxing for the reviewer role.
- Pull the s3-dev failure from the `harnez rate` log to get the exact `-L` argument; may overlap 696 (malformed -L hints).

Reported by the taia session from horg worker runs (epic horg#6), 2026-11-01. Source log: codeberg harnez/horg docs/studies/2026-11-01-agent-performance-log.md, section "Tool Experience".

## 2b. Searcher Findings (lean sprint, 2026-11-01)
- Ranged Read is already allowed: `evaluateReadToolViolation` (`cmd/harnez/hook.go:639-684`) denies a ranged Read only when the range is >= 100 lines; whole-file Read of a >= 100-line file is denied.
- Deny text `readingDisciplineDenyReason` (`hook.go:453`) points only to `harnez read`, never says a Read with offset/limit under 100 lines works. Likely why the reviewer used `cat -n`.
- Threshold 100 is hard-coded at `hook.go:453, 646, 666, 676, 680, 684, 793`; not in `spec/`.
- Mode: `readEnforcementEnabledFor` (`hook.go:204`); tests in `cmd/harnez/hook_test.go` (560, 687, 805-908), `guard_hook_test.go`.
- s3-dev `-L` failure: not in telemetry/rate log; source study gives no exact argument.

## Decision
- Drop the reviewer-role exception: ranged Read already passes, so the fix is the message.
- Drop the s3-dev reproduction: not reproducible without the argument; malformed `-L` hints stay with 696.

## 3. Implementation & Verification Plan
### M1 (clear deny message)
- Deny text names both options: `harnez read -L <range>` and native Read with offset/limit spanning fewer than <threshold> lines.
- Tests assert the new wording for both the whole-file and the too-large-range deny cases.

### M2 (threshold in spec)
- Move the 100-line threshold into a spec file (e.g. `spec/edit_guard.yaml` or a new reading-discipline spec) with its JSON schema under `spec/schemas/`, per `docs/Spec.md`; every hook.go site and the message use the one value.
- Tests: a threshold value comes from spec; boundary cases 99/100 lines for range and whole file stay as today.
