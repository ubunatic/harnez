# 453 — Clarify Opus:low versus Astra:low cost guidance in sprint documentation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Documentation
**Related**: Sprint and reverse-sprint skills; related future documentation issues

---

## 1. Problem & Motivation

The sprint guidance can lead agents to choose `astra:low` when `opus:low` is
the much cheaper option for advisor work. The relative cost should be explicit
so model selection consistently reflects the intended cost discipline.

## 2. Technical Specification / Findings

Review the sprint skills and other model-selection documentation, including
related future issues, and clearly state that `opus:low` is much cheaper than
`astra:low` where that is the intended cost comparison. Preserve role and
quality guidance so the cheaper model is not presented as universally superior.

## 3. Implementation & Verification Plan

/goal: All applicable sprint skills and related documentation clearly guide
agents to prefer `opus:low` over `astra:low` when lower cost is the deciding
factor, with consistent terminology and no contradictory recommendations.

- Find every relevant model-selection reference in skills and docs.
- Update the guidance and related future issue text as appropriate.
- Search again for contradictory cost claims and verify the documentation diff.
