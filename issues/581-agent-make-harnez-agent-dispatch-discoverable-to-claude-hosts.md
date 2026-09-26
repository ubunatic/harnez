# 581 — Make Harnez agent dispatch discoverable and host-visible

**Status**: Open
**Priority**: P1
**Severity**: Moderate
**Category**: Agentic Ergonomics / CLI / Documentation
**Related**: [#578](578-make-harnez-agent-safely-discoverable-and-usable-via-bash-or-mcp.md) (closed), [#585](585-harnez-init-docs-do-not-explain-harnez-agent-commands.md) (closed), [#580](580-harnez-agent-consistent-session-argument-and-detach-across-lifecycle-verbs.md) (merged), [#589](589-agent-recommend-one-native-background-shell-per-agent-run.md) (merged), `docs/studies/2026-09-25-repo-manager-decisions.md`

---

## Goal

`/goal`: A host can discover, start, observe, and manage Harnez agent runs from its loaded
instructions without probing CLI help or hiding work from the user; lifecycle CLI forms are
consistent across verbs.

## 1. Discoverability problem and proposal (original #581)

In a freshly `harnez init`-ed repository, a Claude host instructed to delegate to `luna:med`
and use `terra:med` for review did not know the model aliases, dispatch command, synchronous
behavior, proper backgrounding, scratchpad logging, role/writer rules, or when to ask a peer
session. It consequently ran writers in parallel and detached work inside a shell, making runs
unmanaged and invisible.

Add a concise delegation section to the managed conventions:

- Delegate with `harnez agent start --name <name> --model <alias:tier> --role <role> -d <repo>`;
  `harnez agent models` lists aliases and roles.
- `start` blocks until completion. Dispatch every run in its own native host background shell
  (Claude Code: `run_in_background`), with several host calls when parallelism is valid.
- Advisors and reviewers may run in parallel; developers write sequentially unless the user
  approves disjoint files.
- Agent output belongs in the session scratchpad, never the repository root.
- Use the host's agent/session listing and ask peer sessions about their own repository before
  reading their code cold.

## 2. CLI lifecycle consistency (merged from #580)

The current lifecycle forms are inconsistent: `wait` takes a positional session while `status`,
`stop`, and `resume` require `--name`; a positional value after `resume` is interpreted as the
prompt. `start` has `--detach`, while `resume` does not.

Make `wait`, `status`, `stop`, and `resume` consistently accept the session positionally and via
`--name`; give `resume` the same `--detach` capability as `start`. Once implemented, remove the
current-quirk caveats from the generated Subagent Policy.

## 3. Host-visible native background dispatch (merged from #589)

The recommended pattern for `harnez agent start` and `resume` is one native background shell per
agent run, not `&`, `nohup`, `( … & )`, `--detach`, or `--async`. This keeps each run, tunnel, or
build visible in the host UI, yields completion notification/output, and permits one-action
process-group stopping. Hosts should stop the background shell rather than rely on Ctrl+C within
an agent to stop its children.

- **Three-Tier Caller Context Detection**:
  1. **Case 1: Interactive Host Session (Orchestrator)**:
     - *Signal*: Agent env var present (`CLAUDE_CODE_SESSION_ID`, `ANTIGRAVITY_CONVERSATION_ID`, etc.) and top-level / orchestrator role.
     - *Behavior on `--detach`/`--async`*: Emit an interactive warning/notice:
       > *"Notice: Running with --detach hides this process from the host UI. In interactive sessions with a user, prefer launching agent runs inside a native host background task/shell (e.g. Antigravity task / Claude run_in_background) so the user can monitor, terminate, and receive exit notifications without blocking chat."*
  2. **Case 2: Direct Human at CLI (Manual Terminal)**:
     - *Signal*: `classifyInvoker() == "human"` (no agent env vars, interactive `stdinIsTTY() == true`).
     - *Behavior on `--detach`/`--async`*: Detach quietly as requested by the user, printing the session info and resume command.
  3. **Case 3: Non-Interactive Subagent / Delegator / Script**:
     - *Signal*: Programmatic runner (e.g. non-interactive `sprinter` / `delegator` subagent or CI/script with no TTY).
     - *Behavior on `--detach`/`--async`*: Emit a concise operational notice reminding the calling agent that the job is detached without an automatic completion notification, so active tracking is required, accompanied by the exact command to wait or reattach:
       > *"Notice: Agent detached; no automatic completion notification will be delivered. Active tracking required. Wait or reattach via: `harnez agent wait <session>` or `harnez agent resume <session> \"<prompt>\"`"*

- **Host Task Integration**:
  - Advise callers/hosts to dispatch agent runs through host background facilities (e.g. Claude `run_in_background`, Antigravity `run_command` with backgrounding) that preserve non-blocking chat while delivering reactive exit notifications.

Document this recommendation in the agent guidance and next to `--detach` help. `--detach` must
plainly state that it hides the run from the host UI.

## 4. Work already completed

- #578 added generated Subagent Policy guidance for MCP-versus-Bash routing, lifecycle discovery,
  and the absence of a `harnez advisor` command.
- #585 added the short Harnez Agent section to the managed conventions block, with MCP-first
  guidance and the current CLI forms.

These completed changes establish the baseline; this ticket extends it with host dispatch
guidance and repairs the remaining CLI inconsistency.

## 5. Acceptance criteria

- A fresh Claude host can find model aliases, select the MCP or CLI route, start a named agent
  with an explicit role, inspect it, wait/resume/stop it, and identify peer sessions without
  running `--help`.
- `wait`, `status`, `stop`, and `resume` accept a session both positionally and through `--name`;
  `resume` supports `--detach`; generated policy no longer documents those quirks.
- Managed guidance recommends one native host background shell per run and prohibits shell-level
  detachment; it preserves sequential developer writes and scratchpad-only output.
- `harnez agent start` and `resume` with `--detach` or `--async` output a short runtime notice explaining the detachment implications and recommending host native background tasks for interactive sessions.
- `harnez agent start --help` and `resume --help` state that `--detach` hides the run from the
  host UI, while the recommended dispatch remains the blocking command in a native background
  shell.

## Evidence

In the neus session of 2026-09-25, tickets 001–003 were dispatched, reviewed by `terra:med`, and
then fixed; `harnez agent list` showed `neus-001` through `neus-003` and `neus-review`. User
feedback preferred the native-background-shell pattern because visible entries made concurrent
work clear and controllable.

### Antigravity (AGY) Host & MCP Observations (2026-09-25)

- **Default Execution Timeout (`HTO`)**: When invoking `harnez_command` / `harnez agent start` in an Antigravity host background task, the command timed out after 60s (`harnez exec: timeout kill after 1m0s; rerun with HTO=0 to lift`, exit code 137) during a standard turn with `luna:med`. `harnez_command` should either document `HTO` or default to a longer/unlimited timeout (`HTO=0`) for agent runs dispatched via background tasks.
- **Background Task Integration**: Antigravity's `run_command` with `WaitMsBeforeAsync` cleanly handles blocking commands formatted by `harnez_command` as host-managed background tasks, providing proper reactive wakeups upon completion without active polling.


## Incident 2026-09-27 (neus `/lean-sprint 19`)

A Claude host started a developer with `harnez agent start --detach --role developer --name neus-019-dev ...`
from a normal (foreground) Bash call. The run did not appear in the user's background-shell list.
- No §3 notice was emitted: the full output was only `Started agent neus-019-dev (<id>)`, even though
  `CLAUDE_CODE_SESSION_ID` was set. The Case 1 warning is not implemented yet.
- The host followed its loaded instructions: the managed "Harnez Agent" block written by `harnez init`
  still says `Start: harnez agent start --detach --name ...`, and `harnez init` reported it "unchanged".
  The generated conventions therefore teach the anti-pattern this ticket forbids.
- Fix priority: change the managed block to "one native background shell per run" (Claude:
  `run_in_background`, no `--detach`), then add the Case 1 notice.

## Fix 2026-09-27

`2824879`: `agent start`/`resume` accept `-p/--prompt`; managed Start line teaches a native background shell without `--detach`; test parses every managed-block `harnez agent` example through Cobra. `harnez apply` done. Awaiting reporter feedback after `harnez init`.
- neus feedback (2026-09-27): since the incident it already runs `start`/`resume` without `--detach` in native background shells and the user sees them; `harnez init` there awaits user approval. Final confirmation follows after its next dispatch.
