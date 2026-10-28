# Issue Research

Trigger only on explicit `/issue-research` or "issue-research". Invoke as
`/issue-research` or `/issue-research "<filter/query>"`.

## Phase 1: Config Discovery & Cohort Partitioning

Read `.harnez/research.yaml` and use `spec/schemas/research.schema.json` as the
configuration contract. If the file is missing or invalid, explain what is needed
and stop. Use `harnez find issues` with the supplied query to discover candidate
tickets, then read each candidate's problem and motivation. Partition candidates
into batches of 2–4, respecting a smaller configured `batch_size`; process a
remaining single ticket on its own. The schema permits larger batch sizes, but
cap concurrent batches at four to limit rate pressure.

## Phase 2: Subagent Fanout

For each candidate, write a bounded prompt file containing its ID, problem and
motivation, the configured environment, toolchain, disallowed choices, ignored
blockers, and goals ordered by priority (P0 through P3). Ask explicit research
questions for every goal, current source links, evidence dates, limitations, and
a Viable, Infeasible, or Follow-up verdict. Ask the worker to research only and
leave tracker changes to the coordinator.

Dispatch one session per candidate in parallel using the host's session-owned
background runner, with `HTO=0 harnez agent start --name <unique-name> --role
<configured-role> --model <configured-model> -f <prompt-file>`. Use the configured
model and role, defaulting the role to advisor. If no model is configured, use
`--role advisor --model cheap` where available; if the selected model is unavailable,
report the failure and request a usable model. Track each session and its job;
never use shell backgrounding or polling loops. Follow the host's reactive
completion mechanism and clean up all sessions and temporary prompts.

## Phase 3: Reactive Collection & Synthesis

React to completion signals as workers finish. Check findings and cited sources
against each ranked goal and configured constraint, distinguishing evidence from
inference and recording missing or conflicting evidence. Write a comparative
Markdown report in the repository with candidate, goal findings, tradeoffs,
sources, and verdict columns. Mark incomplete research Follow-up rather than
inferring infeasibility.

## Phase 4: Issue Tracker Reconciliation

Update the configured `actions.update_section` (default `2. Technical
Specification / Findings`) in viable tickets with sourced findings, preserving
existing material. Close an infeasible or obsolete ticket only when evidence
supports the verdict and `actions.auto_close_infeasible` is true. Substitute the
evidence-based summary for `{verdict_summary}` in `actions.close_reason_template`
when supplied, and run `harnez issues close <id> "<reason>" --no-code`. Leave
Follow-up tickets open and document unanswered questions. Follow the repository's
tracker synchronization and commit conventions; report the artifact, updates,
closures, and any failures.
