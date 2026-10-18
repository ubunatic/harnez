# 822 — Assess Gemini 4 Argon (argon:low) quota cost and add agy:argon model and quota-pool support

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `spec/agent.yaml`, `docs/Models.md`, `internal/usage/agy.go`, `internal/usage/statecache.go`, 603, 604, 711

---

## 1. Problem & Motivation

Antigravity (`agy models`) now exposes Gemini 4 Argon in three effort tiers (`gemini-4-argon-low`, `gemini-4-argon-medium`, `gemini-4-argon-high`) backed by a dedicated weekly quota pool (`Gemini 4 Argon` / `Gemini 4 Argon Weekly Limit`, `agymeter` bucket `gemini-argon-wtu-weekly`) with no 5-hour window.

Currently:
1. We needed a clean empirical measurement of how much `argon:low` (`Gemini 4 Argon (Low)`) costs against its weekly quota pool per turn and per token compared to `gemini-3.8-flash` (`agy:flash38`).
2. `spec/agent.yaml` has no `agy:argon` entry, so `harnez agent models` and `harnez agent start` cannot select or price it.
3. `internal/usage/agy.go` (`agyMeterQuotaBuckets`) does not map `gemini-argon-wtu-weekly`, causing `agymeter` fallback snapshots to file it under `"Other AGY Models"` instead of `"Gemini 4 Argon"`.
4. `internal/usage/statecache.go` (`ProviderQuotaAvailabilityFromStore`) checks `strings.Contains(lowerModel, "gemini")` and maps all `gemini*` models to `"Gemini Models"`, so quota-exhaustion checks on `gemini-4-argon` inspect the Flash pool instead of `"Gemini 4 Argon"`.

## 2. Technical Specification / Findings

### Empirical Quota Cost of `argon:low` (`gemini-4-argon-low`, 2026-10-18)

Session `0cb679be-5445-4e4a-aa07-c8ad0b4bf9ba` initiated the 7-day `Gemini 4 Argon Weekly Limit` window from `100.00%` remaining (`0.00%` used) at `2026-10-18T18:06:02Z` (`reset_at: 2026-10-25T18:06:02Z`), giving a zero-baseline, single-session measurement across two checkpoints:

| Checkpoint | Calls | Uncached In | Cache Read | Output | New Tokens (`in + out`) | Total Tokens | Remaining | Used |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| **CP1**: after `/harnez-selftest` (steps 1–15, AGY TUI `/usage`) | 7 | 27,560 | 84,909 | 790 | 28,350 | 113,259 | `99.52%` | **`0.48%`** |
| **CP2**: during issue analysis (steps 1–58, `agy -p "/usage"`) | 19 | 145,082 | 528,907 | 4,637 | 149,719 | 678,626 | `97%` | **`~2.53%–2.88%`** (rounded to `3%`) |

Key cost metrics for `argon:low`:
- **Per-turn drain (short 7-call turn)**: **`0.48%`** of the weekly limit (`~0.069%` per call at ~16k context; `~0.13%` per call at ~36k context).
- **Weekly drain per 100k new tokens**: **`1.693%`** (`0.48% / 0.28350`). Applying this rate to CP2 (`149.7k` new tokens) predicts `2.53%` used (`97.47%` remaining), matching the integer-rounded `97%` from `agy -p "/usage"`.
- **Weekly drain per 1M total tokens (incl. cache reads)**: **`4.24%`** (`0.48% / 0.113259`). Applying this to CP2 (`678.6k` total tokens) predicts `2.88%` used (`97.12%` remaining), also rounding to `97%`.
- **Implied 100% weekly budget for `argon:low`**:
  - **~208** short (7-call) turns, or **~1,100–1,450** individual model calls per week.
  - **~5.91M new tokens** (or **~23.6M total cached+uncached tokens**) per week.

### Comparison Against `gemini-3.8-flash` (`agy:flash38`) on the Same Account

From `~/.harnez/agymeter/usage.jsonl` over the active `Gemini Models` weekly window (`2026-10-17T05:38:34Z` to `2026-10-18T18:13:03Z`, `90%` remaining = **`10.0%` used**):
- `893` Flash calls (`72` low, `598` medium, `223` high) consumed `3.056M` uncached input + `184.0k` output + `188.4k` thoughts = **`3.43M` new tokens** (`100.33M` total tokens including `96.90M` cached) for **`10.0%`** weekly drain.
- **Flash weekly drain**: **`0.292%` per 100k new tokens** (`0.100%` per 1M total tokens; `0.0112%` per call; implied weekly capacity **~34.3M new tokens** / **~1B total tokens**).
- **Relative weekly quota drain (`argon:low` vs `flash38`)**:
  - **`5.8×` more expensive per new token** (`1.693%` vs `0.292%` per 100k new tokens).
  - **`6.1×–11.6×` more expensive per call** (`0.069%–0.13%` vs `0.0112%` per call).
  - **`42.4×` more expensive per total token** when Flash runs at high cache hit ratios (`4.24%` vs `0.100%` per 1M total tokens).
- **`harnez agent models` `COST` placement**:
  - Anchored against `agy:flash38 = 4`, a `5.8×–6.0×` weekly-drain multiple places `agy:argon:low` at **`COST 23–24`** (between `agy:sonnet` at `16` and `agy:opus` at `32`). Even at `:low` effort, `argon` is a premium advisor/reviewer tier and should not be used for routine mechanical loops.

## 3. Implementation & Verification Plan

1. **Add `agy:argon` to `spec/agent.yaml` and document in `docs/Models.md`**:
   - Add `agy:argon` (`aliases: [argon, argon:low]`, `provider: agy`, `name: gemini-4-argon`, `tier: low`, `roles: advisor, reviewer`, `cost: 24`) with the 2026-10-18 measured weekly-quota study in `docs/Models.md`.
2. **Map `gemini-argon-wtu-weekly` in `internal/usage/agy.go`**:
   - Add `{"gemini-argon-wtu-weekly", "Gemini 4 Argon", "Gemini 4 Argon Weekly Limit"}` to `agyMeterQuotaBuckets` so `agymeter` records normalize into the same group/window name as `agy -p "/usage"`.
3. **Route `argon` to `"Gemini 4 Argon"` in `internal/usage/statecache.go`**:
   - In `ProviderQuotaAvailabilityFromStore`, check `strings.Contains(lowerModel, "argon")` before the generic `"gemini"` check and set `wantPool = "Gemini 4 Argon"`.
4. **Verify**:
   - Add unit tests in `internal/usage` and `cmd/harnez` covering `gemini-4-argon` pool selection and `gemini-argon-wtu-weekly` bucket normalization.
   - Run `make test-q1` and `harnez agent models`.
