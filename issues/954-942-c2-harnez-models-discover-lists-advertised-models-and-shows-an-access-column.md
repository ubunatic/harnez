# 954 — 942 C2: harnez models discover lists advertised models and shows an access column

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942 (epic, ## Re-cut), 946 (C1), 955 (C3), 941 (loader)

---

/goal `harnez models discover` reads the CLIs' own model listings (`agy models`; `claude -p "/model"` aliases;
codex where a non-TTY listing exists) without model calls, writes `advertised` observations with method, time
and CLI version atomically to `$XDG_STATE_HOME/harnez/models/access.yaml`, and `harnez agent models` shows an
ACCESS column (advertised / accessible / denied / unknown). An incomplete run never removes a model; unknown
stays enabled. Tests use recorded CLI output. Or stop and report on a format choice. Circuit breaker: on a loop
or no progress, clear the goal and write a failure report here.

Canary facts: docs/studies/2026-10-25-canary-model-access-discovery.md (`codex models` needs a TTY).
