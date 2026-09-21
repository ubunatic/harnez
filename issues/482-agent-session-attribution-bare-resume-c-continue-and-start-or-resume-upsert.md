# 482 — Agent session attribution: bare resume, -c/--continue and start-or-resume upsert

**Status**: Open

**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #479 (epic, design §2.4), #481, #478, #306, #483

---

## 1. Problem & Motivation

Callers must remember session names to talk to an agent again. For repo-default
agents (for example a ticket-filing agent per repository) they want
`harnez agent -d <repo> -c -p "…"` to just reach the right agent, without ever
sending a prompt to the wrong one.

## 2. Technical Specification

A session is *attributable* to a call when all hold: it is resumable (#478),
not quarantined (#306), its working directory equals the canonical `-d`, and the
caller may manage it (`CanManage` lineage via `HARNEZ_SESSION_ID` /
`AGY_CONVERSATION_ID`).

- `harnez agent resume [-d D] "prompt"` (no name): exactly one attributable
  session is required. Zero: error suggesting `start`. Several: error listing
  the candidates by name and last activity. Never guess.
- `-c, --continue` (any prompt form): the most recently active attributable
  session; if there is none, start a new one with the default model and a
  generated short name.
- `harnez agent --name N "prompt"`: upsert. Resume `N` if it exists, otherwise
  start it. `start --name N` and `resume --name N` stay strict.
- `harnez agent start "prompt"` without `--name`: generated short random name.
- Destructive verbs (`stop`, `delete`) never accept `-c`; they need `--name` or
  a unique attribution and print what they resolved.
- Every resolution is visible in the stream header:
  `[session info: id=… agent=… action=resume resolved=name|dir|continue|new]`.
- Open question: hosts other than agy may export no comparable session id.
  Investigate Codex and Claude Code environment variables; until then
  attribution degrades to directory-only, which is why bare `resume` requires
  uniqueness.

## 3. Implementation & Verification Plan

- `attributable(store, dir, caller)` helper plus table tests: zero, one, many;
  terminal and quarantined sessions excluded; lineage exclusion; `-c` ordering;
  upsert both branches; `-c` refused on `delete`.
- Depends on #481 (`resolveSession`) and #478 (a reliable resumable flag).
- Document the rules in `docs/HarnezAgentArchitecture.md`.
