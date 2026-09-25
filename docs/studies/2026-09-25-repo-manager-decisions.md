# Repo Manager Decisions (2026-09-25)

Decision log of the harnez repo-manager session. Flow per ticket: quick assessment, read-only
research by `codex:luna:med` advisor, findings written into the ticket, `codex:luna:med` developer,
host diff review and commit (lean-sprint rules).

## Triage of 576–579

| Ticket | Assessment | Decision |
|---|---|---|
| 576 empty telemetry DB files | P3, small, likely no code source | Research, then fix or close with finding |
| 577 index titles / close reasons | P2 feature, design choice needed | Research; scope M1 to lint + backfill 115/116 before CLI changes |
| 578 harnez agent discoverability | P2, vague goal, overlaps 575 | Research gaps first; no dev until a concrete list exists |
| 579 lite quota-1 init guidance | P2, clear doc/template bugs | Research, then fix sources |

Order of developer work (one writer at a time): 579, 576, 577, 578.

## Pitfall seen by the host

`harnez agent wait --name res-576` fails with "accepts 1 arg(s), received 0": `wait` takes the
session as a positional argument, although `--name` is shown as a global flag. The host's
background wait loop hid the error and "finished" at once. Correct form:
`harnez agent wait res-576 --timeout 30m`. Input for ticket 578 (discoverability).

## Decisions after research

- 576: no code creates the files; docs-only fix and close (host does it, no developer).
- 577: lint for sha-only close reasons plus backfill of 115/116; `issues close` keeps allowing no
  reason, so agents are never forced to invent one.
- 578: fix lives in the generated Subagent Policy text, not in new commands.
- 579: shared AGENTS.md template names the media gate instead of numbering it, because full and
  lite AgenticLoop number it differently (10 vs 7). `/goal` rule applies to new tickets only.

## 579 review

- Developer reported "no `--- FAIL` found" but its new test failed (fixture dir not a git repo, init
  refused). It grepped its own redirect file; `make test-q1` also prints `Failing tests:` and keeps
  the full log under `.git/harnez/quota-1-logs/`. Host review of that log caught it. Lesson: the
  host always greps the q1 log, never trusts a "no FAIL found" report.
- `harnez agent resume` has no `--detach` while `start` does, and it needs `--name` (a positional
  name is taken as the prompt) while `wait` needs the name positionally. The host ran it as a background shell
  job instead. Input for 578.
- Second round: the developer's claim "make test-q1 passed" was correct; `.git/harnez/quota_1.state`
  shows a third run with `exit:0` before the commit. Passing runs leave no file in
  `quota-1-logs/`, so the host briefly misread the newest *failing* log as the latest result.
  Lesson: check `quota_1.state` (`finished`, `exit`) against the commit time, then the log.
- 579 closed. Note: the shared AGENTS.md text also lives in `config.yaml`; both were changed.
