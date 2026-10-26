# 953 — Interactive Codex sessions are treated as Harnez leaf agents

**Status**: Closed — Verified interactive Codex host identity and launcher provenance; host review dispatch and self-test work while leaf guards remain enforced
**Closed-By**: codex:luna:low, session c943
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #486, #581, #665, #470, #965; [OrchestratedAgentFlow](../docs/OrchestratedAgentFlow.md), [ReviewLearnings](../docs/ReviewLearnings.md)

---

## /goal

Make a normal interactive Codex session usable as a Harnez host, with host operations available and leaf-worker restrictions preserved for spawned agents; verify the self-test and agent dispatch from that host, or stop and report if the active host sandbox policy prevents the required behavior.

## 1. Problem & Motivation

This interactive Codex session could not use Harnez as a normal host. Harnez exported `HARNEZ_AGENT_ROLE=advisor` into the interactive session. `harnez agent selftest --step hello` and attempts to start an agent were rejected with “agent role advisor is a leaf worker,” even though the user was chatting with the top-level interactive host and expected it to act as orchestrator.

The same session had related sandbox friction: `harnez rate` could not write its session record under `~/.harnez/sessions`, and adding `/home/uwe/.cache/go-build` to `~/.codex/config.toml` did not change the active managed permission roots. Gokay lint failed package loading with the default Go cache but worked with `GOCACHE=/tmp/...`.

This leaves ordinary Codex + Harnez workflows confusing and unreliable: host operations are blocked as if the host were a leaf, and filesystem restrictions are not explained by the user-level Codex configuration.

## 2. Technical Specification / Findings

- `env` showed `HARNEZ_AGENT_ROLE=advisor` and `CODEX_PERMISSION_PROFILE=:workspace` in the interactive session.
- `harnez agent list` showed the Codex session as active with source `vscode`.
- `harnez agent selftest --step hello` failed immediately with the advisor leaf-worker refusal.
- `harnez agent start` was refused with the same leaf-worker message, even when `--role orchestrator` was supplied.
- `harnez rate` failed to write `/home/uwe/.harnez/sessions/2322f1b00adbf2d2.ticket` because the filesystem was read-only.
- The configured writable root `/home/uwe/.cache/go-build` remained unwritable after resuming; setting `GOCACHE=/tmp/gokay-sandbox-gocache` allowed Gokay lint to complete and report a source finding.

Investigate whether the host role environment is incorrectly inherited or set by the interactive Codex integration, and clarify which filesystem restrictions are controlled by Harnez versus the host-managed Codex permission profile.

## 3. Implementation & Verification Plan

- Confirm how interactive Codex hosts are identified and why this session receives the `advisor` leaf role.
- Ensure ordinary interactive hosts can run Harnez self-tests and dispatch agents, while spawned leaf workers remain unable to manage agents.
- Provide actionable behavior or documentation when the active Codex sandbox makes Harnez session state or configured cache paths read-only.
- Verify the flow from an interactive Codex host without overriding `HARNEZ_AGENT_ROLE` manually.

## 4. Reproduced in Cati interactive host — 2026-10-26

Two authorized host workflows failed in `/home/uwe/projects/cati` with exit 1 and the same refusal:

```text
Error: agent role "advisor" is a leaf worker: it must not start, resume or manage agents; do the work yourself and report back; the host runs live harnez agent checks after your commit
```

1. The owner invoked the story skill, which requires an independent second-vendor review. The host used `harnez_command` to prepare a read-only reviewer launch, then executed the returned `HTO=0 harnez agent start --json --stream stats --model claude:sonnet:high --role reviewer --dir /home/uwe/projects/cati --name cati-story-review-20261026 -- '<read-only review prompt>'`. Startup was rejected before any reviewer ran. The story was committed as review-blocked, and Cati issue 119 tracks the missing review.
2. The owner explicitly invoked the self-test skill. Its required first command, `harnez agent selftest --step hello`, was rejected before returning runtime progression instructions. No later self-test steps could run. This happened again after reinstalling Harnez with `make install` while filing issue 963.

A targeted environment read confirmed `HARNEZ_AGENT_ROLE=advisor` in this interactive host. This reproduction is not dependent on the earlier filesystem failures: the current host has unrestricted filesystem permissions; `harnez rate`, `harnez read`, `harnez find`, build/install, and index checks worked. Both failures were rated. No environment override, sandbox bypass or alternative agent launcher was used to evade the role guard. The diagnostic's direction to let “the host” run the checks is circular when delivered to the interactive host itself.

Extend acceptance verification to both commands: an actual interactive host can launch the explicitly requested reviewer and complete self-test progression, while a genuinely spawned advisor still cannot launch children. Capture host/parent identity and the role's provenance in rejection diagnostics so an inherited or incorrectly resolved role can be distinguished from an intentional leaf restriction. Do not solve this by globally allowing advisors to spawn agents.

References: Cati story commit `f178232`, Cati issue 119, and the subsequent owner-requested self-test in this session. Issue 959 is a separate prompt-based role-inference failure, not a duplicate root-cause claim. Implementation and live verification remain outstanding; this issue stays Open.

## 5. Resolution and verification — 2026-10-26

- Baseline: the active interactive host inherited `HARNEZ_AGENT_ROLE=advisor`,
  `HARNEZ_SESSION_ID=c943`, and `HARNEZ_AGENT_MODEL=codex:luna:low`.
  No stored worker matched c943. `harnez agent selftest --step hello` exited 1
  with the leaf-worker refusal. The actual Codex thread's metadata identified
  source `vscode`; its process ancestry led to the managed Codex app server.
- Decision: verify the exact current thread's bounded first metadata record
  before treating inherited worker variables as stale. The spec lists accepted
  interactive sources (`cli`, `vscode`). Unknown, missing, malformed or exec
  metadata retains the role restriction. `HARNEZ_AGENT_LAUNCHER_PID` binds new
  workers to their live launcher ancestry, including interactive workers;
  matching stored provider IDs protect legacy interactive workers.
- Shared caller-role resolution covers start, resume and the command guard.
  Interactive host tracking and explicit target access use the real thread ID.
  Refusals include environment provenance, thread/worker identity and launcher.
  Architecture documentation explains host-managed sandbox/cache limitations.
- `make test-q1` passed the complete suite, including new host/worker regression
  cases; `make lint` passed (repository size/complexity warnings only).
  `make install` updated the installed binary.
- Live verification without role overrides: self-test hello, background,
  confirm, confirm-running and verify completed successfully for
  `--session issue953-host`. The original advisor environment remained intact.
- Actual review dispatch succeeded: `HTO=0 harnez agent start --name
  issue953-review --model codex:luna:high --role reviewer ...`, provider thread
  `01a17936-00cc-7462-b839-afc5a4e7b466`, parent
  `01a17932-4b30-70c1-a470-248960b5b8b4`. Reviewer inspected the changed files,
  reported no findings, and its live self-test attempt exited 1 with the expected
  reviewer leaf refusal and provenance. The review process exited successfully;
  session status is completed and no worker remains running.
- Code: `6d9ab6b0`; architecture/sandbox explanation: `2ae74526`.
- Session learning: inherited role variables describe a launch context, not
  necessarily the current interactive host. Exact metadata and launcher ancestry
  now encode that distinction in code and regression tests. No new follow-up
  work was identified. Sandbox restrictions were absent in this live host;
  Harnez does not claim to reconfigure another host's permission policy.
