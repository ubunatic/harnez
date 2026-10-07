# 656 — harnez agent send: opt-in messaging across providers to existing interactive and detached sessions

**Status**: Open
**Priority**: P2 (Medium) — start only after 630 (start/resume/wait backgrounding guides) is closed
**Severity**: Moderate
**Category**: Agentic Ergonomics / Feature
**Related**: 630 (session vs machine background), 611 (host polling), 439 (agent chat lifecycle), 477 (hook-driven completion), 483 (root agent form, slash interception), [680 (Codex cross-task messaging probe)](680-probe-codex-cross-task-messaging-for-agent-send.md)

---

## 1. Problem & Motivation

Hosts can start agents but cannot talk to an agent that runs fully detached, and a host
does not reliably know which agents it has started. 630 fixes how hosts run agents in the
session background. This ticket designs a real eventing layer on top: `harnez agent send`.

The target includes existing interactive sessions across providers, including sessions the
sending host did not start. A concrete acceptance case is a Claude Code session sending a
message to an existing Codex interactive session. Detached children are one supported case,
not the scope boundary.

## 2. Scope, in order (each milestone gates the next)

- **M1 Host session agent tracking.** Record, per host session, the number and names of all
  agents it started (start, resume, detached). Keep the host continuously informed of what was
  started (e.g. hook-injected context or the `start`/`resume` output), so it never loses track.
- **M2 Statusline count.** Show the host session's detached agent count in each agent's
  statusline (Claude, AGY, Codex where a statusline exists).
- **M3 Delivery discovery (canary, per agent).** For Claude Code, Codex and AGY, find what
  **actually** works to deliver a message into a running or idle agent, not what docs claim:
  - cover existing interactive sessions as well as detached workers, across providers and
    outside the sender's child-agent tree; prove Claude Code → Codex delivery as an acceptance case;
  - discover and resolve the recipient by stable provider/session ID or an unambiguous name;
    document access requirements and running versus idle behavior;
  - piggyback on the agent's native messaging (e.g. Claude `SendMessage`, AGY/Codex equivalents);
  - hooks (inject the message as context on the next prompt/tool event);
  - just resume with another prompt (`harnez agent resume --name X "<msg>"`);
  - anything else found. Record the recommended way and the proven ways per agent in
    `docs/studies/`, with probe evidence (`docs/Canary.md`).
  - **Codex route:** integrate with `codex_tui.send_message_to_thread(threadId, prompt)`;
    determine how Harnez obtains and selects the target thread ID, and distinguish an accepted
    send from confirmed recipient receipt. Issue 680 tracks the canary and interface probe.
- **M4 `harnez agent send --name <name> -m <msg>`.** Implement with the M3 winners per agent,
  only after M1-M2 work. Support existing interactive and detached sessions across providers.
  Distinguish accepted transport, confirmed receipt, and recipient response; report unsupported
  or failed delivery explicitly. Acceptance alone must not be presented as confirmed receipt.

## 3. Constraints

- Opt-in. Either a user-callable skill (e.g. `/harnez-peering`) or a `harnez apply` toggle;
  M3 decides which fits better. Default off.
- No new external daemon unless M3 proves nothing native works; the agents stay "the system"
  (see 630).
- Re-check live code and 630's outcome before starting.

## 4. M1 Plan Review (host, 2026-10-01)

M1 plan by dev-656 (flash37:low) accepted with these required changes:

- **One source of truth.** `subagent.Session.ParentSessionID` is already written on every start path
  and `session.go` already filters by parent. Derive the per-host-session agent list from it. Do NOT
  add a second `AgentRef` index to `sessionstate`.
- **Resume from another session.** Keep `ParentSessionID` as provenance (unchanged). If the resuming
  host must see the agent too, add one field (e.g. `LastHostSessionID`) set on resume; count an agent
  for a host when either field matches. Test both sessions' views.
- **Host informing = start/resume output only for M1.** One short line, e.g.
  `harnez: this session started 3 agents (1 running): dev-a, rev-b, adv-c`. No sessionTipHook
  reminder in M1 (noise; revisit in M2 with the statusline).
- **No spec/schema change** unless a real configurable value appears.
- **Docs:** update `harnez agent start --help` / `sessionBackgroundHelp` or `config.yaml` Tools line only
  if the output line needs explaining; no AgenticLoop change for M1.
- **Codex host id:** add a test that the PPID fallback yields the same host session id across two
  separate harnez invocations from the same parent (two `exec_command` calls), or document why not.
- Edge cases from the plan stay (no host id, same name twice, unknown host).

## 5. M1 Review (host, 2026-10-01): delivered in 012aa22d, one regression to fix

**M1 delivered (host session tracking):** `currentHostSession` (HARNEZ_SESSION_ID, AGY, Claude/Codex ids,
Codex-only PPID fallback), `LastHostSessionID` on resume, one stderr line after start/resume.
Host live run from Claude Code: `harnez: this session started 2 agents (0 running): m1-probe, m1-probe2`. OK.

**Pre-Work / Required Refinements (before M2):**

- **Regression: Claude Code hosts lost access to existing agents.** Before M1, Claude Code hosts had an
  empty parent id and acted as root. Now `CLAUDE_CODE_SESSION_ID` scopes them, so on a live Claude host:
  - `harnez agent list` shows only this session's agents (91 rows before, 0 now; `--all-sessions` works);
  - `harnez agent stop --name dev-656` (ParentSessionID empty, started before M1) fails:
    `session "..." is outside caller lineage`. The same applies to `resume`, which also blocks the
    "resume from another session" case that section 4 requires, whatever the unit tests show.
  Required: an explicit `--name`/id target on resume/stop/delete/status/wait/rate from a host session must
  keep working for agents with empty ParentSessionID and for agents of other host sessions (hand-off).
  Keep CanManage's lineage guard for leaf workers (callers whose HARNEZ_SESSION_ID is a harnez worker),
  which is what it exists for. Add a test for each: host resumes a legacy agent, host resumes another
  host's agent (then both views list it), worker is still refused outside its lineage.
- **`list` scoping:** keep the scoped default, but when other sessions have agents, append one stderr line:
  `harnez: N more agents in other sessions; use --all-sessions`.

## 6. M1 Delivered (host, 2026-10-01)

**M1 (host session tracking) closed:** 012aa22d plus regression fix a7d010f4. Host live checks from
Claude Code: start/resume print `harnez: this session started N agents (M running): ...`; `stop --name`
of a pre-M1 agent works again; `list` is scoped and prints
`harnez: 90 more agents in other sessions; use --all-sessions`; a fake worker
(`HARNEZ_SESSION_ID` set) is still refused outside its lineage.

**Pre-Work for M2 (statusline count):**
- Claude workers can't be resumed after a long turn until 673 is fixed; plan M2 as one-turn
  developer handoffs, or fix 673 first.
- Developers must not discard or revert files outside the set named in the handoff. Report
  unexpected changes to the host instead (a dev reverted the user's spec edits in M1).
- Count = this host session's agents with status running (same source as the M1 line).

/goal Hosts always know which agents they started, statuslines show the detached count, and
`harnez agent send` delivers messages to existing interactive and detached sessions across
providers, including Claude Code → Codex sessions outside the sender's child-agent tree,
through methods proven by canary with confirmed recipient receipt; stop and report when
blocked on unavailable session access, a user decision (opt-in form), or a denied permission.

## 7. Rule: tickets carry the work, messages only nudge (user, 2026-10-07)

Agents communicate main decisions and work requests through tickets (in any repository), not
through messages alone; `harnez agent send` is the nudge ("look at ticket N in repo X"). This
keeps a track record between agents without transcribing everything. Not M2 scope for 740; record
it in the agent-send docs/skill (e.g. peer-assistant, AgenticLoop) when `send` ships.

## 8. Progress (host, 2026-10-07)

Claude recipients are done: channel server and `send` (740), allowlisted plugin route without the
development warning (743), `harnez apply` ships the plugin (747), default session names and the
`send --name|--id <message>` form, registry pruning (749), plural command words (748). Open: Codex
route (680). §7's tickets-first rule is now in `docs/practices/AgenticLoop.md` (multi-session peer
assistance, full and lite).
