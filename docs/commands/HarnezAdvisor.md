Use this skill when asked to reuse an advisor session across supported agent tools.

The interface is prose-first: interpret the request and state the plan before acting. Identify, when supplied, the target agent or harness, session ID, project or repository, advisor task, model, and desired effort. Do not invent missing values.

Reuse a session only through that harness's native resume, continue, or fork mechanism, and only when the agent family and version, model and effort where relevant, project identity, instruction and permission context, advisor role, task scope, and session validity all agree. A Harnez telemetry session ID is for correlation; it is not a provider resume ID. Never choose a provider session by recency or by matching a Harnez ID.

Prefer advisors already working in this session, context, project, or feature. Treat unrelated or random advisor sessions as poor candidates unless the request explicitly identifies them.

Advisor lifecycle is decided by the orchestrator. Track when an advisor was last used and whether its project, feature, role, model, instructions, and permissions still match. Reuse a recent matching advisor directly. If a matching advisor is stale, ask it to compact its context before resuming so later requests can use a smaller summary while retaining relevant memory. Start a fresh advisor when the scope is unrelated or compatibility is uncertain. Staleness is a signal for compaction, not an automatic rejection.

After requesting compaction, confirm that the advisor returned a concise retained-context summary. Where token data is available, compare the next request's input and cached-input counts with the pre-compaction turn; do not claim savings from compaction without that evidence.

If a critical value is missing, mismatched, unsupported, expired, or ambiguous, explain why and use a fresh advisor session when the request permits it. Low reasoning is the default only where the harness supports it; report when it cannot be honored.

Current capability guidance:

- Claude: use the installed CLI's native resume, continue, or fork behavior when exposed.
- Codex: resume by the native session ID; local rollout metadata may provide input, cached-input, output, reasoning, and total token counts.
- AGY and Gemini: use the specific installed harness's continue or resume behavior when available. Do not assume AGY and Gemini have the same session interface.
- Prime: report unsupported and use a fresh fallback while no Prime executable or session API is available; use native reuse once that capability exists.

Label evidence as `measured`, `reported`, `estimated`, or `unknown`. Cached input is not the same as saved tokens or money: claim savings only when a comparable fresh control exists. Do not persist prompts, transcripts, credentials, session IDs, or arbitrary command output in the repository.

There are no portable skill flags. Treat requests such as `/harnez-advisor reuse <plain request>` as prose and pass the interpreted intent to the native harness. A future `harnez advisor reuse --flags` command may provide a machine-readable interface; do not claim it exists yet.
