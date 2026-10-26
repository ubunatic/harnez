# 967 — Verify harnez applies all concepts from the agent harness video (XJuLcDxtTz0)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research
**Related**: 763 (study of another harness video), 619 (Phantom Looper concepts), 914 (Harness2 study)

---

/goal Every concept the video https://www.youtube.com/watch?v=XJuLcDxtTz0 ("What Is an Agent Harness? How AI Coding
Agents Work") teaches is listed, and each is marked as applied in harnez (with the file or command that shows it),
partly applied, or missing; missing items are filed as their own tickets. Done when that table is in this ticket.
Or stop and report when no transcript or description can be read. Circuit breaker: on a loop, a hard blocker or no
progress, clear the goal and write a failure report here.

## 1. Problem & Motivation
The owner wants to know whether harnez covers the ideas in this talk on agent harnesses. Harnez should apply the
concepts it teaches, and any gap is a candidate for a ticket.

## 2. Technical Specification / Findings
- The video page gives only its title. A fetch returned no description and no transcript, so the concept list is not
  known yet.
- Next step: get the transcript or a summary of the talk from the owner (paste it, or confirm a source), then
  list the concepts.
- Check each concept against live code and docs in harnez (`docs/`, `.harnez/rules/`, `spec/`), not against old
  tickets alone.

## 3. Implementation & Verification Plan
1. Obtain the concept list (transcript or owner summary).
2. Map each concept to harnez evidence, with file paths or commands.
3. Record the table in this ticket; file one ticket per missing concept, linked here.
