# 723 — Consolidated agent container with in-container lmcoder smoke test

**Status**: In Progress — images build; smoke test not yet run
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Infrastructure
**Related**: [#716 Pi skills and hooks](716-install-harnez-skills-and-hooks-into-pi-instances.md), [#721 OpenCode native agent](721-support-opencode-as-a-native-harnez-agent.md), [#722 Claude Code on local model](722-run-claude-code-against-the-local-lmcoder-model-in-the-agent-container.md), [#071 canary architecture](071-agent-canary-container-for-hook-testing.md), [#072 Pi/OpenCode canary](072-agent-canary-local-llm-pi-opencode.md), [#073 credentialed canary](073-agent-canary-cloud-credentialed-claude-agy-codex.md)
**Depends on**: [#716 Pi skills and hooks](716-install-harnez-skills-and-hooks-into-pi-instances.md)

---

## 1. Problem & Motivation
Harnez has two container files: the root `Containerfile` (build proof) and
`scripts/agent-canary/Containerfile` (harnez + Pi + OpenCode, with the model
server on the host per #071). Neither proves end to end that agents installed
next to harnez work and see Harnez's skills and hooks.

/goal One self-contained container setup that installs harnez and every
supported local-LLM agent, starts a tiny local model via lmcoder inside the
container, and verifies each agent: PING PONG, skills, hooks and other installed
components. Stop and report if an agent or lmcoder cannot run this way without
forks or credentials.

## 2. Technical Specification / Findings
- **Files:** consolidate into `Containerfile.app` (harnez + agents) and
  `Containerfile.lmcoder` (lmcoder + model server). Replace the two existing
  files; follow `docs/Containerfile.md`.
- **Model server:** lmcoder runs inside the container, on CPU, with the
  smallest model that reliably answers PING PONG and lists skills. Weights are
  downloaded on first run into a cache volume, not baked into the image. This
  supersedes #071's "lmcoder on the host" split.
- **Agents:** Pi and Codex first, latest upstream releases, no forks. OpenCode
  follows #721, Claude Code follows #722.
- **Assumes** #716 (Pi skills/hooks) and per-agent hook support are done before
  this starts; missing skills or hooks are failures, not known gaps.
- Upstream versions float; the smoke test prints every installed version.

## 3. Implementation & Verification Plan
- Build both images; run the smoke test with one command (Make target).
- Per agent: PING PONG through the local model; agent lists its skills;
  `harnez status` reports skills and hooks installed; a hook visibly fires.
- Other components: harnez version, git/git-lfs, lmcoder server health.
- Record per-agent timings and versions in the ticket.

### Maintainer Execution Command
To run the smoke test locally on a host with container runtime access:

```bash
make smoke-container
```

Or manually:

```bash
docker build -t harnez-app -f Containerfile.app .
docker build -t harnez-lmcoder -f Containerfile.lmcoder .
docker run --rm harnez-app /usr/local/bin/smoke-test
```
