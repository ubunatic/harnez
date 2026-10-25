# 925 — spec: allow user overrides of spec values from ~/.config/harnez

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 920 (remote load poll intervals), Epic 921, `docs/Spec.md`

---

/goal Selected spec values (first: `remote_load_poll_interval_attached`/`_detached` from 920) can be overridden per
user from a file under `~/.config/harnez`, validated against the same JSON schema, with the embedded spec as default;
or stop and report on an owner decision (which keys are overridable, file layout). Circuit breaker: on a loop or no
progress, clear the goal and write a failure report here.

## Notes

- Owner (2026-10-25, on 920): "more spec is always good, we may also consider allowing override by ~/.config/harnez
  values".
- Finding (d920): the spec loader reads only the embedded `spec/usage.yaml`; `~/.config/harnez/local.yaml`
  (`internal/usage/localconfig.go`) holds user-local settings but no spec values.
- Open decision: an allow-list of overridable keys vs. any key; merge in the spec loader vs. `local.yaml`.
