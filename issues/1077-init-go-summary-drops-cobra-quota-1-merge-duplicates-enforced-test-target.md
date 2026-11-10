# 1077 — init: Go summary drops Cobra; Quota-1 merge duplicates Enforced Test Target

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 701, 364, 884

---

## 1. Problem & Motivation

Plain `harnez init` re-run in loom (2026-11-01, older init state from before 701) produced two regressions:

1. **Cobra dropped from the Go summary.** `config.yaml:899` hint changed from "Modern Go, avoid deps but use Cobra, add tests; ..." to "Modern Go; substantive features and tests in internal/<feature>, thin cmd/<app> ...; avoid deps, use runes and display width". `docs/Go.md` still mandates Cobra for every Go app, so the AGENTS.md one-liner no longer matches its doc.
2. **Quota-1 merge is sloppy (corrected 2026-11-01).** loom's `.harnez/rules/Quota.md` had `# Quota-1 Guardrails` (H1, full rules incl. Clean Tree First) plus a later duplicate `## Quota-1 Guardrails`. Init (plain and `--quota-1`) correctly drops the duplicate and keeps the H1 rules, but: (a) appends a second **Enforced Test Target** bullet (new wording) after a blank line instead of replacing the old one, so the rule appears twice; (b) does not add **Report Untested Edits**, present in harnez's own Quota.md; (c) `--quota-1` prints "AGENTS.md ... Quota-1 Guardrails: added" though AGENTS.md gets no such section (only Quota.md has it). An earlier read of this ticket claimed rules were lost; that was a misread of the diff context.

Open question: is running plain `harnez init` on a pre-701 layout a supported migration, or a misuse that should need e.g. `--quota-1`/an upgrade path? Either way init must not silently delete rules; if the layout is unsupported it should warn/refuse rather than prune.

**Goal**: `/goal` Restore Cobra in the Go summary hint and make init's Quota-1 merge yield each current rule exactly once from older layouts (H1 or H2 heading, duplicated sections), replacing outdated wording in place, and fix the misleading AGENTS.md 'added' report, with a regression test using loom's pre-init Quota.md as fixture; or, if plain init on that state is deemed misuse, make init detect it and warn. Stop and report if blocked on a user decision.

## 2. Evidence

- loom pre-init: `git -C ../loom show fc86921:.harnez/rules/Quota.md`
- loom post-init diff: `-## Quota-1 Guardrails` block removed, `+- **Enforced Test Target**: ...` only.
- Merge code: `internal/claude/init.go` (`quota1SectionName`), tests `internal/claude/quota1_init_test.go`.

## 3. Verification

Focused init test with the loom fixture; re-run init in loom and confirm Quota.md contains each current Quota-1 rule exactly once, and AGENTS.md Go line mentions Cobra.

Starting point (checked 2026-11-10 at af9688e0, after a discarded Jules report with drifted references):
the Go summary hint at `config.yaml:940` still omits Cobra, while `docs/lang/Go.md:70` requires Cobra for
every Go app. The Quota-1 merge half was not re-checked.
