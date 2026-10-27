---
kind: sprint
---

# Two-agent lean sprint: 974, GPT-6.1 Terra onboarding

Date: 2026-10-27. Host: Claude Opus 5.5. Searcher `codex:luna:low` (`search-974`, one turn, 28s, 53k new / 193k
cached). Developer `codex:luna:med` (`dev-974`, three turns: plan 38s, M1 1m55s, commit and close 17s).

## Searcher findings

Entry shape from `codex:sol`; the 715 guard is a test that `terra` resolves as unknown, and resume refuses a session
whose stored model is no longer in the spec. Installed Codex 0.164.0 lists `gpt-6.1-terra`. OpenAI's page matched
the ticket; no independent SWE-bench or tool-calling result, only OpenAI's Terminal-Bench claim and one secondary
report.

## Decisions

Owner: reuse `terra` aliases, roles developer/advisor/reviewer, Terra about half Sol and 10x Luna, near-Sol quality.
Host: cost 15/24/36 (half of Sol per tier; the 10x-Luna anchor gives a different scale). Recorded in the ticket.

## Delivery

- M1 `484e2721`: `codex:terra` spec entry, aliases resolve to `gpt-6.1-terra` (715 test flipped), resume test asserts
  the error names `gpt-5.6-terra`, golden, cost and guidance assertions, Models/ModelResearch/ModelRoles docs.
  `preferred` only breaks ties between bare aliases; Luna and Sol defaults unchanged.
- Closed in `0c8e3fc4`.

## Verification

Host: `make test-q1` exit 0, 58 `ok`, 0 `--- FAIL`; `harnez agent models` lists terra low/med/high at 15/24/36;
live canary `--model terra:low` ran as `codex:gpt-6.1-terra`.

## Friction

- The developer's only full run failed on its own repository-relative link in a copyable doc; it fixed the link but
  could not rerun under the one-run budget. The host ran the full suite.
- The live canary from a developer session was refused by the leaf-role guard (developers cannot run
  `harnez agent`). The host ran it; handoffs that ask a developer for an agent canary should assign it to the host.

## Follow-up

- 983: owner asked that model picks per task class live only in the spec, not in skill prose (filed, under epic 942).
