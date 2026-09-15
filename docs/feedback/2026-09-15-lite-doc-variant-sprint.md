# 2026-09-15 — lite-doc-variant sprint (357-360)

Full 5-phase sprint across four sequentially-dependent tickets, shipping the `lite_source`
doc-variant mechanism end to end: schema field + resolver (357), drift-detection marker (358),
the `AgenticLoop.lite.md` pilot + behavioral canary (359), and the CLI switch (360).

## What went well

- The prerequisite chain (357 → 358 → 359 → 360) held up exactly as scoped — no rework needed
  when a later ticket touched an earlier one's code.
- Advisor discovery caught real gaps before implementation started twice: 359's ticket text
  assumed a `harnez docs variant --check` CLI verb and an LLM-invocation canary harness that
  don't exist. Rather than silently building unscoped extra CLI surface or silently skipping
  validation, both gaps were named, a stopgap was shipped for the pilot (a `go test` structural
  gate, manual canary scoring), and two follow-up tickets (361, 362) were filed for the deferred
  automation — consistent with canary-first practice.
- Backward compatibility for `ApplyAll`/`RunInitWithForce` (many dozens of existing test call
  sites) was preserved by adding new `*Variant` functions that the originals now delegate to,
  rather than changing signatures in place. Reviewer confirmed this was the right call, not
  gratuitous indirection, given the blast radius of touching ~35 call sites for no behavioral gain.

## Friction / gaps worth naming

- The lite doc's target size (15-20% of the full doc) was optimistic once "no rule omitted" was
  taken seriously — the shipped pilot landed at ~29%. The ticket's own priority (omission over
  compression) made this an acceptable tradeoff, but a smaller doc with mechanically-precise
  taglines (dropping full-sentence anti-pattern rationale down to true one-liners) could likely
  hit the original target with more editing passes than this sprint had budget for.
- The behavioral canary (359) is currently hand-scored, not automated — issue 362 exists but real
  LLM-invocation-and-diff automation is still a gap. The manual pass (7/7 both variants) is
  reasoning-based, not a real two-session comparison; treat it as a plausibility check, not proof.
- No CI runs this repo's `go test`/`smoke-test.sh` automatically as part of this workflow — every
  verification in this sprint was a manually-triggered local run. Fine for now given the solo/
  hobby repo setup, but worth remembering if the follow-up tickets (361, 362) ever want a gate
  that fires without a human/agent remembering to run it.

## Follow-ups filed

- [361](../../issues/361-harnez-docs-variant-check-cli-verb-for-lite-doc-structural-gate.md) —
  generalize the stopgap structural-gate test into a reusable `harnez docs variant --check` verb.
- [362](../../issues/362-llm-invocation-canary-harness-to-automate-lite-doc-behavioral-scoring.md)
  — build the LLM-invocation harness the manual canary scoring stood in for.
- [231](../../issues/231-local-compact-doc-profile-for-small-local-models.md) — re-scoped (not
  closed): its mechanism-design step is now redundant with 357/360's `SourceFor`/`--variant`;
  only content-authoring work remains, still blocked on 149.
