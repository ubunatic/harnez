# 835 — lean-sprint: allow agy:argon:low as architect tier

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `docs/commands/lean-sprint.md` §1.5, issue 770, ubunatic.com issue 080

---

## 1. Goal

/goal The lean-sprint skill lists `agy:argon:low` as a supported architect tier next to `opus:low`, `flash38:low` and `sol:low` (with bare `argon` normalized), so the host does not stop and ask when the user names it; or stop and report when the tier list lives in a spec that needs a user decision.

## 2. Findings (2026-10-20)

In ubunatic.com sprint 080 the user asked for `agy:argon` as architect. The skill's §1.5 rule forced a stop-and-ask round trip; the user answered "anyway". argon:low returned a usable design in 1m56s (milestones, rewrite rules, host pitfalls such as the Plesk nginx bypass) with one stale pre-work note. Rated 4/5.

## 3. Plan

1. Add `agy:argon:low` to the tier list and normalization in `docs/commands/lean-sprint.md` §1.5 (and any spec or config copy, see issue 770).
2. Reinstall the skill and verify the installed copy.
