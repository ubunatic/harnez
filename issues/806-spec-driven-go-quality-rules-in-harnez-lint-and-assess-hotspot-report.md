# 806 — Spec-driven Go quality rules in harnez lint and assess hotspot report

**Status**: Closed — resolved in f9502234; harnez lint --lang go reports 171 warnings, assess --hotspots on harnez, full suite green
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Tooling
**Related**: 276, 057

---

## 1. Problem & Motivation
There is no automated signal for spaghetti growth: 4k-line files and very long functions grow unchecked. harnez is the quality hub other repos adopt.

## 2. Technical Specification / Findings
- Go rules: `go-file-length`, `go-func-length`, `go-params`, `go-import-fanout`, reported as warnings.
- Thresholds live in `spec/lint.yaml` (+ schema), loaded via the spec system; no Go defaults.
- `--check` gets a level (`error` default, `warning` makes warnings fail).
- `harnez assess --hotspots`: largest files, longest funcs, packages without tests.

## 3. Implementation & Verification Plan
Unit tests with small thresholds against fixture sources; run `harnez assess --hotspots` on harnez itself.
