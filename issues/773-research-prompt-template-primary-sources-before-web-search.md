# 773 — Research prompt template: primary sources before web search

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: lmcoder issue 178

---

## 1. Problem & Motivation
When a host dispatches web-research agents about an external tool, the
prompt usually asks for "find X, cite URLs". In lmcoder issue 178
(2026-10-10, Strata LLM server) a codex:luna:med searcher reported that
Strata "needs 64 GB RAM" from the project's marketing site. The project's
own `docs/MODELS.md` and `setup.sh --check` showed a coder variant that
runs in ~26 GiB. The wrong summary shaped the study and the first
recommendation until the host re-read the repo itself.

## 2. Technical Specification / Findings
- Source quality order that would have avoided it:
  1. the project's own repo docs (README, docs/, model/hardware tables),
  2. the tool's `--help` and any `--check`/dry-run output on the target host,
  3. release notes and issues,
  4. third-party web pages last, labelled as such.
- Researchers should flag claims that come only from tier 4 and list which
  repo docs they did not read.

## 3. Implementation & Verification Plan
- Add a short "research an external tool" prompt template (or a section in
  the dispatch docs / a skill) with the source order and the flagging rule.
- Reference it from the Canary-first doc (probe external mechanisms).
- Verify: rerun the Strata question with the template on a cheap model and
  check the report names `docs/MODELS.md` and the coder variant's RAM need.
