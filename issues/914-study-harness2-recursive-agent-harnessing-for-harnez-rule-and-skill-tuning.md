# 914 — Study Harness2 (recursive agent harnessing) for harnez rule and skill tuning

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: 302 (self-modifying agent research), 366 (doc compression with canary eval loop), 904, 878

---

/goal Write a study in `docs/studies/` that says which Harness2 ideas harnez should adopt for tuning its rules,
skills and docs, each with a cited source and a verdict (adopt / adapt / skip), and file one ticket per adopted
idea; or stop and report when the paper or code cannot be read. Circuit breaker: on a loop or no progress,
clear the goal and write a failure report here.

## Source

Video "Google's Harness² Is This the End of Harness Engineering?" (Kai, 2026-10-23, 22 min):
https://www.youtube.com/watch?v=yCgfyQmqhPM. Its description cites:

- Paper: Harness²: Recursive Agent Harnessing for an Open World (Xu et al., 2026), https://arxiv.org/abs/2610.14224
- Project page (owner addendum): https://harness2.github.io/
- Code (Apache 2.0): https://github.com/google-research/harness2, experiments doc `docs/EXPERIMENTS.md`
- Spend-limit request: https://github.com/google-research/harness2/issues/4

Claims from the description, unverified until the paper is read:

- An editor model watches an agent run, proposes two alternative harnesses (prompt, rules, memory, tool notes,
  skills, scripts, subagents), runs both, and keeps only edits that changed the agent's behaviour
  ("Propose, Probe, Compose"); skipping the probe lowered scores.
- No weights or answer key; GLM-5 went from 64.2 to 75.7 on WorkBuddy, from an empty base harness.
- Cost: 4 agent runs per task and about 8x tokens per round, up to 20 runs and 40x; no budget cap.
- The editor itself runs on 4,776 words of hand-written prompts (Appendix H).
- Failure modes: bad picks, overfitting, an agent that downloaded its own rubric.

## Questions

1. Does the probe step (keep an edit only if behaviour changed) fit harnez's canary practice
   (`docs/Canary.md`) and the doc-compression eval loop of 366?
2. Which harnez artifacts are candidates: `.harnez/rules/`, `docs/commands/` skills, copyable docs?
3. What budget cap and rubric isolation would harnez need (quota rules, `harnez exec --quota-1`)?
4. Which of the found edits can be copied by hand today?
</content>
</invoke>
<invoke name="Bash">
<parameter name="command">cd /home/uwe/projects/harnez && harnez issues open 914 --commit "docs(issues): file 914, study Harness2 recursive agent harnessing for rule and skill tuning" 2>&1 | tail -2