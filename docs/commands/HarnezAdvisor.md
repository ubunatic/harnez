Use this skill when the user explicitly asks to reuse or resume an earlier advisor session.
There are no portable skill flags. Interpret reuse requests as prose and use the active tool's
supported lifecycle commands.

Ordinary requests such as "ask for help", "get a review", "ask an agent", or "get a quick
second opinion" use a fresh helper with a short prompt. Dispatch directly through the active
subagent method; no advisor-session discovery, compaction, or sprint workflow is needed.
Lean-sprint reviewers and advisors follow that same direct path.

- Use `reviewer` to inspect a diff or proposed change; use `advisor` for a question or read-only investigation.
- Give the helper the question, relevant files or diff, and desired reply. Keep the prompt scoped;
  do not load unrelated documents or restate the helper's base instructions.
- Use native helpers when the active policy permits them. For a user-selected model, use
  `HTO=0 harnez agent start --role <reviewer-or-advisor> --name <name> --model <requested-model> -p "<short prompt>"`
  as a tracked session-background job. Honor the requested model exactly.
- Read the result when the tracked job completes, report the useful answer or findings, and
  clean up the helper using the active tool's lifecycle controls.

For explicit reuse requests:

1. Use the named session or an advisor already assigned to the current task. Do not search
   transcript archives or select an unrelated session merely because it is recent.
2. Confirm that project, task scope, role, model, instructions, and permissions still fit.
   Harnez-managed sessions resume with
   `HTO=0 harnez agent resume --name <name> "<short prompt>"` as a tracked session-background job;
   native helpers use their supported continuation mechanism. A Harnez telemetry ID is not
   a provider resume ID.
3. If the session is unavailable or incompatible, explain briefly and use a fresh helper
   when permitted by the request. Ask only when a missing choice matters to that request.

Compaction is optional for a long reused conversation when context pressure warrants it;
it is not a prerequisite for asking a question. Do not claim token or cost savings without
measurements. Do not store prompts, transcripts, credentials, or session IDs in the repository.
