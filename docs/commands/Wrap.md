---
name: wrap
description: Close the current work session, preserve its state, and prepare a concise handoff
disable-model-invocation: true
---

# Wrap

Close the current work session and leave the repository and its next steps clear.
Use this at the end of a session or before a context reset or handoff. This is
an explicit, user-invoked workflow; do not commit merely because work happened.

## Session Close Checklist

1. Review the current conversation and repository state. Identify completed
   work, unfinished threads, observed bugs, and durable decisions or learnings.
2. Stop background helpers and subagents started for this session. Confirm
   their state with the available task/session tools; do not stop unrelated
   sessions.
3. Sync issue tracking. Close completed tickets with a reason, update the
   issue index, and file distinct tickets for substantive bugs, broken
   assumptions, or follow-ups that should survive this session. Do not file
   nitpicks.
4. Update relevant evergreen `docs/*.md` when the session established durable
   architecture, design, or operational knowledge. `/evergreen` curates those
   learnings and related issue/index state; `/wrap` closes the session, checks
   lifecycle hygiene, and prepares the repository handoff. Invoke `/evergreen`
   when that broader curation is needed.
5. Preserve repository state in a commit. Review `git status` and the diff;
   commit work you authored and any stray documentation or issue changes after
   a quick sanity check. If work is partial or broken, first file or update a
   handoff issue describing the state and resume point, then commit the state
   with that ticket number in the commit message. Do not imply incomplete work
   is finished. If the tree is already clean, verify that with `git status`.
6. Confirm the final status with `git status` and report the current HEAD,
   completed work, remaining work or handoff ticket, active open issues, and a
   recommended next command or action. Keep the report concise and factual.
