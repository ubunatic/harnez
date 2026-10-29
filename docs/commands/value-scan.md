---
disable-model-invocation: true
---
# Value Scan

Use `/value-scan <topic>` to help the user choose valuable work within one topic. If the topic is missing, ask for it. This is a conversational, read-only scan, not generic triage or a backlog-grooming operation.

## Workflow

1. **Discover the tracker and scope.** Read the repository's instructions and product context. Use its documented issue tracker and active-status conventions. For a Harnez tracker, use `harnez find -d . issues -a status:active` and topic search. If the tracker is unavailable, report the limitation and stop; do not guess ticket contents from titles.
2. **Find and read candidates.** Separate direct matches from incidental mentions. Read each candidate's current goal, status and acceptance, and follow relevant epic links. For a large result set, bound the scan, explain the selection and disclose omissions; ask the user to narrow it when necessary. If a ticket body cannot be read, say so and exclude unsupported conclusions.
3. **Describe outcomes.** Give two or three brief, concrete outcomes for each ticket. Distinguish implementation from discovery, recommendations or planning. Identify dependencies and overlapping epic/child outcomes rather than counting them twice.
4. **Rank value.** Order the candidates by expected benefit to this repository and its users. State the criteria and relevant uncertainties; do not substitute priority or a universal numeric score for judgment.
5. **Identify quick wins.** Highlight tickets that appear both valuable and small in remaining scope. Mark estimates provisional; before calling one verified easy, check its live code, recent history and remaining acceptance. A completed implementation may still require a substantial live run.
6. **Offer a next step.** Ask which candidate to examine further or implement, or stop. Do not change tickets, priorities, files or code as part of the scan. Do not delegate unless the user requests it or the invocation explicitly authorizes it.

When no suitable tickets are found, report the search scope and ask for a different topic. Do not invent a tracker, candidate or outcome.
