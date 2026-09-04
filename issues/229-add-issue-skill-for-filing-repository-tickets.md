# 229 — Add /issue skill for filing repository tickets

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [docs/CommandsPipeline.md](../docs/CommandsPipeline.md), [docs/IssueTracking.md](../docs/IssueTracking.md), [config.yaml](../config.yaml), [commands/evergreen.md](../commands/evergreen.md)

---

## TL;DR

Add `commands/issue.md`, register it under both `commands:` and `skills:` in
`config.yaml`, then verify its installation:

```bash
make lint
make apply
make status
```

The installed `/issue` workflow should tell an agent to file a repository
ticket with this command-first sequence:

```bash
harnez find -d <repo> issues "<search terms>"
harnez find -d <repo> issues next --reserve "<title>"
# Fill the reserved ticket using the repository's issue conventions.
harnez index -d <repo>
git -C <repo> add issues/<ticket>.md issues/README.md
git -C <repo> commit -m "docs(issues): file <ticket-summary>"
```

> **Keep this skill deliberately thin.** The repository's `AGENTS.md` and
> issue-tracking documentation should already be in context. `/issue` should
> provide the invocation trigger and command TL;DR, not duplicate or restate
> the full issue workflow.

## 1. Problem & Motivation

Harnez installs several reusable commands as slash commands and cross-agent
skills, but it has no focused `/issue` workflow for turning a request into a
well-formed repository ticket. Agents must rediscover the issue-numbering,
duplicate-search, metadata, index, and commit conventions from project docs.
That adds friction and makes skipped searches, number races, stale indexes, and
uncommitted tracker changes more likely.

Create a concise installable `/issue` skill that makes "file a ticket" a
direct, repeatable operation. Its opening section must be a TL;DR containing
the commands an agent normally needs to run.

## 2. Technical Specification / Findings

- Use one source file, `commands/issue.md`, following the source-reuse pattern
  in `docs/CommandsPipeline.md`.
- Register `issue` in both `config.yaml` lists:
  - `commands:` so Claude-compatible slash-command surfaces receive `/issue`.
  - `skills:` so Gemini/Antigravity, Codex, Claude Code, and Prime Agent receive
    the installable skill.
- Give the skill a narrow, trigger-shaped description: use it when the user
  asks to file, create, capture, or open a repository issue/ticket. It must not
  trigger merely because an implementation mentions an existing issue.
- Start the body with a short TL;DR showing the normal commands: search for a
  duplicate, atomically reserve the next ticket, edit the reserved file,
  rebuild the index, stage the exact tracker files, and commit them.
- Do not over-specify the workflow or copy issue-tracking policy into the
  skill. Assume applicable repository instructions are already loaded; point
  the agent to them and include only guidance unique to invoking `/issue`.
- Respect repository-local instructions and issue conventions. In a
  harnez-managed repository, prefer `harnez find ... issues` and
  `harnez find ... issues next --reserve` over raw directory scans or manual
  number selection.
- Require the agent to inspect existing issue conventions before drafting,
  choose priority independently from severity, and include concrete problem,
  scope, acceptance criteria, and verification guidance.
- Avoid inventing implementation details when the request is intentionally
  exploratory. Record uncertainties and decisions needed in the ticket.
- Keep tracker mutations atomic: update the index and, where repository rules
  require it, commit the ticket/index immediately without sweeping unrelated
  working-tree changes into the commit.
- Handle non-harnez repositories gracefully: follow their local tracker and
  instructions rather than assuming `issues/`, `harnez`, or an immediate-commit
  policy exists.

## 3. Implementation & Verification Plan

1. Add `commands/issue.md` with the command-first TL;DR and concise workflow.
2. Register the shared file under `commands:` and `skills:` in `config.yaml`.
3. Add or extend pipeline tests to assert the command and skill are generated
   with their respective frontmatter and installed in every configured target.
4. Run `make lint`, relevant Go tests, and `make install` because the change
   affects Go-tested installation behavior/configuration expectations.
5. Run `make apply` and `make status`; confirm `issue` appears in both command
   and skill summaries and that a second apply is idempotent.
6. Exercise `/issue` in a temporary repository with an existing issue and dirty
   unrelated files. Verify it searches first, reserves without collision,
   updates only the tracker/index, and does not stage unrelated changes.

## Acceptance Criteria

- `/issue` is available through every configured command/skill installation
  surface supported by harnez.
- The skill begins with a practical TL;DR listing the commands to run.
- A normal invocation produces a correctly numbered, indexed ticket that
  follows the target repository's metadata and commit rules.
- Duplicate discovery and atomic reservation are explicit parts of the flow.
- The skill stays concise and does not duplicate issue workflow already
  supplied by repository context.
- Verification covers installation, idempotency, and preservation of unrelated
  working-tree changes.
