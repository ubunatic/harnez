# 972 — Claude guesses and pushes to implement instead of verifying; add a verify-first instruction

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Bug
**Related**: 969 (compaction limits, the session this came from), 353 (Claude-only instructions via `harnez hook claude-instructions`), docs/Canary.md (probe external mechanisms first)

---

/goal Add a durable instruction that makes Claude verify facts (or propose research) before it proposes
values or an implementation, and stops it from pushing for a "go" on unverified ground; prove it with a
replay of the 969 discussion, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

In the 2026-10-26/27 session on compaction limits (issue 969), Claude (Opus 5.5) repeatedly stated guesses as facts,
proposed concrete values before research came back, and asked the owner to "say go" on a plan built on those
guesses. The owner had to correct it again and again. harnez is meant to ship things that must work, not things
that may work; every parameter needs a verified source or an explicit "unverified, to be researched" mark.

Owner's view: Claude should have asked, started a web search, or proposed a research step, instead of filling gaps
with assumptions to get to development quickly.

## 2. Findings: what went wrong (Claude's self-assessment)

Concrete cases from that session:

1. **Values before evidence.** Proposed "Haiku 100k at 75–80%" while the research agents were still running.
2. **Did not ask which models are in use.** Proposed a Gemini Pro override (160k) from generic pricing data; the
   owner only uses Flash 3.8 and Gemini 4 Argon. One question would have avoided it.
3. **Generalised from one data point.** From one measurement (100k window compacts at ~67k) concluded the buffer is a
   fixed 33k, computed `--autocompact 113000` for an 80k trigger, and said "a separate canary isn't needed".
4. **Stated a number as fact without a source.** Used "33k buffer" as settled; when asked, it turned out to be one
   `/context` reading plus third-party blogs, undocumented and probably model/version dependent.
5. **Derived claims from that guess.** Said Claude's default trigger is "about 84%" assuming a 200k window and a
   fixed buffer.
6. **Filled a mapping table with guesses.** Wrote that harnez would set agy's "own compaction threshold as a
   fraction"; research later showed agy has no documented automatic threshold at all.
7. **Accepted weak research.** Relayed "Codex default not published" from a first pass; a second pass found the 90%
   default and clamp in Codex source. Did not ask the first agent to check source code.
8. **Pushed for approval.** Ended almost every reply with "say go", nudging toward implementation while key facts
   were still open.
9. **Skipped session setup.** Did not read the harnez rules at session start as the repo instruction requires.

What Claude did right (defence):

- Launched research agents early, in parallel and on cheap models, as the owner asked.
- Marked unverified items in relayed reports and did not edit code or config without approval.
- Corrected itself when shown wrong (dropped Gemini Pro, explained the 60% was JEV-only, admitted the 33k is
  measured, not configured).

The defence does not cancel the pattern: the corrections were triggered by the owner, not by Claude checking first.

## 3. Implementation & Verification Plan

- Check existing coverage first (docs/Canary.md, docs/AgenticLoop.md, AGENTS.md, the 353 hook text); extend the right
  place instead of duplicating. Claude-only habit → `harnez hook claude-instructions` (353) or the Claude
  instruction file; shared rule → AGENTS.md/docs.
- Rule content (draft, keep short):
  - Every number, default, flag or behaviour you propose needs a source (doc, code, measurement) or an explicit
    "unverified" mark plus a proposed way to verify it.
  - Missing context about the owner's setup (which models, plans, tools): ask, do not assume.
  - Do not propose concrete values while research you started is still running.
  - One measurement is a hypothesis; propose the canary that would confirm it, do not skip it.
  - Ask for "go" only when the open facts are listed and either verified or explicitly accepted as risk.
  - When a research report is thin, send it back for primary sources (official docs, source code) before relaying.
- Verify: replay the opening of the 969 discussion in a fresh Claude session with the rule active; it must ask which
  models are used, wait for research before proposing values, and mark the buffer as unverified.
