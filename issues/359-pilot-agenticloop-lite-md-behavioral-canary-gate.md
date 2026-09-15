# 359 — Pilot: AgenticLoop.lite.md + behavioral canary gate

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Feature
**Category**: Templates / Docs / Token Efficiency
**Related**: [Issue 357](357-config-yaml-lite-source-variant-field-on-copyable-doc-entries.md) (prerequisite: variant field), [Issue 358](358-self-describing-variant-marker-so-drift-detection-tolerates-lite-docs.md) (prerequisite: drift detection), [Issue 356](356-extract-anti-patterns-section-from-agenticloop-md-into-its-own-copyable-doc.md) (likely unnecessary if this lands), [docs/other/Canary.md](../docs/other/Canary.md) (canary-first practice this follows)

---

## 1. Problem & Motivation

Given the schema/plumbing from issues 357-358, this ticket authors and validates the
first real lite doc. Pilot selection, measured 2026-09-15:

| Doc | bytes | default |
|---|---|---|
| `docs/practices/AgenticLoop.md` | 26,844 | true |
| `docs/practices/GoRelease.md` | 12,744 | auto |
| `docs/practices/IssueTracking.md` | 10,563 | true |
| `docs/lang/Bash.md` | 9,789 | auto |
| `docs/practices/ConciseMode.md` | 4,466 | false |

**Pilot: `AgenticLoop.md`.** It's 2.1x the next largest, `default: true` (inlined into
essentially every managed project), and its Anti-Patterns section alone is 7,944 bytes
(30% of the doc) — the section most amenable to taglining, since each anti-pattern
already has a bolded name a frontier model can expand correctly.

Do **not** pilot `Bash.md` — its value is verbatim syntax rules (`if test`, the
directory-flag table) that taglining would destroy, not explanatory prose that
compresses well.

The key risk: a lite doc silently dropping a rule (not paraphrasing it wrong, but
omitting it) is undetectable by inspection alone — a model that "mostly" respects a
tagline gives no error when it drops an edge case. Trusting the taglining process
without a gate is not acceptable.

## 2. Proposed Fix

Author `docs/practices/AgenticLoop.lite.md`, target 4.0-5.4 KB (15-20% of the 26,844-byte
full doc), preserving every `###`/bolded-name heading from the full doc as at least a
one-line tagline — the failure mode to guard against is omission, not compression.

Two-gate validation, per the repo's own canary-first practice:

1. **Structural gate** (cheap, automatable): `harnez docs variant --check <name>`
   asserts every bolded rule-name / `###` heading present in the full doc has a
   corresponding entry in the lite doc. Catches omission. Cannot catch wrongness.
2. **Behavioral canary**: a fixture set of 6-10 prompts under `scripts/`, each targeting
   one rule with a mechanically checkable violation — e.g. "write a shell conditional"
   must use `if test`; "wait for a CI run to finish" must not emit a blocking `sleep`;
   "the fix regressed a gate, discard the attempt" must commit before reverting;
   "dispatch two ticket-filing subagents" must not run them in parallel. Run each
   fixture against full-doc context and lite-doc context, score pass/fail per rule.
   Ship the lite doc only if it scores >= the full doc on the fixture set.

Lite docs are hand-authored and reviewed, never generated on the fly at apply/init time.

## 3. Acceptance Criteria

- [ ] `docs/practices/AgenticLoop.lite.md` exists, marked with the issue 358 variant
      marker, registered via `lite_source:` on the `agentic-loop` config.yaml entry.
- [ ] Structural gate passes: no heading/rule-name present in full but absent from lite.
- [ ] Behavioral canary fixture set (6-10 prompts) implemented and run; lite doc's score
      recorded and >= full doc's score before merge.
- [ ] `go test ./...` and `scripts/smoke-test.sh` pass; `harnez apply`/`init --docs
      agentic-loop --variant lite` (pending issue 360's flag, or a manual `lite_source`
      toggle if 360 hasn't landed yet) installs correctly.
- [ ] Cross-link to issue 356: if this pilot's Anti-Patterns taglining is judged
      sufficient, close 356 as superseded rather than doing both.
