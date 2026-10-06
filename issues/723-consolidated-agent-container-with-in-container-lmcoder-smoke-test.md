# 723 — Consolidated agent container with in-container lmcoder smoke test

**Status**: Closed — make smoke-container passes all 25 checks for Pi 1.0.4 and Codex 0.160.1 on in-pod lmcoder; model download path untested (host disk)
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

## 4. Results
Run with `make smoke-container` (podman; `scripts/agent-canary/run.sh`). Both
images run in one pod and share loopback; lmcoder serves `canary`
(qwen2.5-0.5b-instruct-q4) on CPU at 127.0.0.1:8734, weights in the
`harnez-lmcoder-cache` volume.

2026-10-06, all 25 checks passed: harnez 0.1.24, Pi 1.0.4, codex-cli 0.160.1,
node 22.23.3, git 2.39.5, git-lfs 3.3.0, llama.cpp b10590 CPU. PING PONG: Pi
17 s, Codex 46 s. Pi and Codex each list every Harnez skill (read from Pi's
JSON system prompt and Codex's session file); `harnez status` reports all
skills and hooks ok; Pi fired the Distill hook and Codex the telemetry hooks
(counted in `harnez log`).

Findings:
- The first cloud-agent version could not have passed: the app image failed
  to build (`go.work` points at a sibling repo; fixed with `GOWORK=off`),
  lmcoder was never started or connected, `lmcoder start` detaches, the Codex
  config used invalid keys, and skipped checks still reported success.
- Codex runs config hooks only after the user trusts them; the test uses
  `--dangerously-bypass-hook-trust`. Real installs: #724.
- lmcoder refuses any model download with under 10 GiB free disk. This host
  had 4.6 GiB, so the run used `LMCODER_MODEL_DIR=~/.cache/llama-canary/models`
  (existing weights, mounted read-only); the download path is untested.
- Codex hides the four bundled third-party skills (explicit-only by design),
  so both agents are checked against Harnez's own skill set.
- OpenCode's launcher and templates were removed; restore them from git
  (before commit 29755237) for #721.
