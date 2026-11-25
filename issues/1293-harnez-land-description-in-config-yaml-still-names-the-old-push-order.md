# 1293 — harnez-land description in config.yaml still names the old push order

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: docs/commands/HarnezLand.md, issues/1292-flaky-internal-server-runner-launch-tests-fail-in-full-test-q1-runs.md

---

## 0. Goal
`/goal` Make the `harnez-land` skill description in `config.yaml` (the `harnez-land` entry, about line 507) match the push order in `docs/commands/HarnezLand.md`, then regenerate the generated copies. Done when the description reads direct push to main when allowed, then AGit, then a topic branch and PR, and `harnez init -d .` leaves no drift. Stop and report if the generated skill list needs a design decision.

## 1. Problem & Motivation
Commit `ab01b0da` changed the push order in `docs/commands/HarnezLand.md` to: direct push to `main` when allowed, else AGit, else a topic branch and PR. The `config.yaml` description still says "direct to main when push is allowed and confident, else via AGit/PR", so it does not mention the topic-branch step. The description is what the agent reads when it chooses the skill, so the old wording can lead it to skip the topic-branch fallback.

## 2. Technical Specification / Findings
- The description lives in `config.yaml` under the `harnez-land` skill entry (`description:`), and the skill body is `docs/commands/HarnezLand.md`.
- The same wording may appear in generated skill lists; `harnez init -d .` regenerates them, and drift is caught by the tests that compare generated skill output.
- No code change is expected: the fix is wording in `config.yaml`, then regeneration.

## 3. Implementation & Verification Plan
1. Edit the `harnez-land` description in `config.yaml` to name the three steps in the same order as `docs/commands/HarnezLand.md` section 2.
2. Run `make install`, then `harnez init -d .`, and `git checkout --` any unrelated file it rewrites.
3. Verify: `make test-q1` passes; `git grep "else via AGit/PR"` finds nothing.
