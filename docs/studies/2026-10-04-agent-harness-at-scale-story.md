# Harnez: Building the Harness While Using It

**Date**: 2026-10-04  
**Scope**: A portfolio review of Harnez's product, recent development, and agent workflow evidence.

Harnez began as a way to keep coding-agent setup in one place. Claude Code, Google Antigravity, OpenAI Codex, and Prime Agent each have their own settings, hooks, commands, and instruction files. Harnez makes a declarative `config.yaml` the source of truth, then applies managed sections while preserving user-owned configuration. Over time it grew into a Go command-line toolkit for agent dispatch, usage monitoring, issue tracking, telemetry, skills, release work, and project setup.

That breadth is visible in the repository: its Go module targets Go 1.26.5 and uses Cobra, YAML, and pure-Go SQLite among its dependencies. The tree includes 255 Go source files and 243 Go test files; tests account for roughly 78% as many lines as production Go code. The project assessment counted 1,579 files and about 310,000 lines across code, tests, documentation, and configuration. Harnez touches Linux desktop and system services, terminal interfaces, local databases, and the files and command interfaces of several coding-agent clients. CPU and GPU telemetry, shell hooks, provider APIs, and local agent services also appear in its documented workflows.

The repository has been unusually active: 2,547 commits landed in the six weeks before this review. That count includes a great deal of issue-tracker and documentation work, so it should not be read as 2,547 independent product features. Recent examples show a tight loop from ticket to implementation to evidence. On October 3, issues 692, 686, and 695 delivered role aliases, prompt-based role resolution, and positional start and fuzzy resume commands (`18a910fe`, `0190c196`, `0a5637e8`). Issue 694 added spec-driven rejection of disallowed Antigravity tools (`226a71d7`). The previous day, a usage-dashboard experiment was reshaped into a simpler table and spec-driven view modes (`f343e8ed`, `a09b39fe`).

One instructive episode came from `harnez agent delete --all`. Users expected cleanup to converge, but Codex sessions could reappear because session discovery merged rollout files without consulting Harnez's deletion archive. Ticket 688 first separated provider failures from ordinary pending sessions and added explicit recovery paths. Live verification then found a subtler case: six records belonged to sessions that had never become Codex threads. The provider returned the same error for an unknown identifier as for a real deletion failure, so its exit code could not distinguish them. The follow-up used local rollout-file existence as the evidence: no rollout meant there was no provider session to delete. Tests covered the missing-file and provider-failure cases, and a live check ran `delete --all --force` twice, with the second run producing no repeated session (`2c4d4ec6`, `e0ccbadf`).

This is a useful example of agents helping with work that spans more than code generation. The ticket preserves the initial failure, implementation constraints, the discovery from the live run, test expectations, and the final behavior. The commits include tests and architecture updates alongside the fix. Elsewhere, a recent cross-repository study records an orchestrated sequence across Harnez and the companion `neus` classifier project: static role handling stays fast, while model inference happens after command submission. That study documents advisory exploration, sequential ticket execution, review gates, and quota-limited verification. In this portfolio review, the repo instructions explicitly prohibit this worker from delegating, so the evidence here comes from committed artifacts rather than a newly run multi-agent workflow.

The strongest agentic-development case is cumulative rather than magical. A single developer could build any one feature, given time; the repository does not prove that agents were necessary. What it does demonstrate is a sustained operating model: 2,547 recent commits, hundreds of tracked tickets, 456 closed tickets in the issue index, extensive tests, and durable records of decisions and failures. The process appears to make parallel research and rapid implementation manageable while keeping the shared code changes sequential and reviewable. Its own open issues also show the cost of that speed: the latest tracker contains follow-ups for duplicate Quota-1 rules, missing tracker discovery, and lockfile hygiene (701–703). The honest story is not that agents remove engineering judgment. It is that a well-instrumented harness lets a small project repeatedly turn agent-assisted investigation into tested, traceable changes—and exposes process defects when the harness itself falls short.

## Facts

| Measure | Evidence |
|---|---|
| Stack | Go 1.26.5, Cobra, YAML, pure-Go SQLite; supporting Bash, Python, TypeScript, and HTML |
| Repository size | 1,579 files; about 310,000 lines across assessed categories |
| Go tests | 243 test files; 55,614 test lines; about 78% test-to-production-code line ratio |
| Activity | 2,547 commits from 2026-08-23 through 2026-10-04 |
| Tickets | 456 closed status entries in the issue index; recent fixes include 688, 692, 694, 695 |
| Recent milestones | Agent delete recovery (2026-10-02); role inference, fuzzy resume, hook enforcement (2026-10-03) |
