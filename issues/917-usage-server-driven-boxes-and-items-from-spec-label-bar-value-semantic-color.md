# 917 — usage: server-driven boxes and items from spec (label, bar, value, semantic color)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 912 (swap row, the hand-coded way), 687 (interactive dashboard), 689, 879 (remote load), docs/Spec.md, docs/TUIDesign.md

---

/goal The usage collector (server) delivers boxes and their items as generic, UI-independent data defined in
`spec/` (with a JSON schema), and the usage UI renders them without per-box code; or stop and report on an
owner design decision. Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Idea (owner, 2026-10-25)

- Today each box (Load, Remote Load, ...) is hand-coded in the UI; adding swap (912) means touching collection,
  wire struct and renderer.
- Instead: `spec/` declares which boxes exist and which item goes into which box. The server sends each item
  generically: label, bar/graph, value or percentage, prefix/suffix, whatever a row needs.
- Colors are semantic, not chosen by the UI: e.g. a level (ok/warn/critical) or thresholds per item
  (green when low, red at 100%), so any UI technology renders the same meaning.
- The server also tells the UI which boxes there are. Hotkeys: boxes 1–9 get the digit keys; a few extra boxes
  keep fixed default keys (Remote Load = `R`).

## Open questions (for refinement)

- Item model: fields and kinds (bar, value, sparkline/graph, text); how thresholds and "unknown" (not 0) are expressed.
- Who computes level/color: server from spec thresholds, or UI from a value plus thresholds.
- Compatibility: older remote binaries and stored caches with the current fixed structs.
- Which existing boxes move first (Load as the pilot is a natural fit after 912).
- Overlap with 687/689 dashboard decisions; read docs/TUIDesign.md before designing.
