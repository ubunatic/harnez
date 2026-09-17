# Codex Background Job & Async Wait Guidance

Archived rule originally shipped in `~/.codex/AGENTS.md` via `agents_md.agents.codex` (Issue [[149]]) before global root instruction management was deprecated in Issue [[415]]. Preserved here for future refactoring into `docs/practices/AgenticLoop.md` or a copyable Codex practice doc.

## Original Instruction

<!-- harnez:begin Background Job Waiting -->
### Background Job Waiting
Codex has no host-notified subagent/background-job completion callback like
Claude Code's. `codex agents` only lets you browse agent sessions on the shared
local app-server daemon, and `codex queue` only queues a message into an existing
session — neither is an event-driven completion notification. Verified against
codex-cli 0.153.4: no such primitive exists yet.
Do not spin a tight polling loop waiting on a background task or subagent. If you
must wait on one, use a bounded/backoff poll (increasing delay between checks, with
a maximum number of attempts or a maximum wait time) against `codex agents` rather
than a fixed tight loop, and prefer surfacing the wait to the user instead of
blocking silently.
<!-- harnez:end Background Job Waiting -->
