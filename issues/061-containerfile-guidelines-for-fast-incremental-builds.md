# 061 — Containerfile.md: Guidelines for Fast, Cached, and Incremental Container Builds

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Documentation
**Related**: [docs/Go.md](../docs/Go.md), [docs/Make.md](../docs/Make.md), [docs/Bash.md](../docs/Bash.md), `lmcoder/issues/039-add-git-and-git-lfs-to-agent-containers.md`

---

## 1. Problem & Motivation

Containerfiles and Dockerfiles in project repositories and agent environments frequently suffer from suboptimal layer ordering. When package installations, vendored code copying, config files, and environment variables are intermixed or ordered incorrectly, even minor config tweaks or local code edits invalidate heavy caching layers.

This causes:
- Full re-downloads of system packages and package-manager dependencies (apt, npm, pip/uv) on trivial edits.
- Long build times that break fast agentic feedback loops and slow down canary test runs.
- Inconsistent tool availability inside containers (e.g., missing `git` or `git-lfs`, which breaks agent workspace inspection and commit diffing).

Harnez needs a bundled `docs/Containerfile.md` (or `docs/Docker.md`) convention document that dictates container build efficiency, caching discipline, mandatory developer tools, and deferred settings.

## 2. Specification & Core Rules for `docs/Containerfile.md`

A bundled container convention guide should establish the following rules:

### 1. Strict Cache-Optimized Layer Ordering (Static First, Dynamic Last)
Containerfile instructions must be ordered strictly from slowest-changing to fastest-changing:

1. **Base Image & System Tools**:
   Install OS packages (`apt-get`, `apk`), `ca-certificates`, and core developer tools (`git`, `git-lfs`) first.
2. **Static Package-Manager Runtimes**:
   Install public npm global packages, pip wheels, or binary dependencies *before* copying any local project code or vendored sources.
3. **Pre-built Environments**:
   Build standard virtual environments (e.g. Python venvs with standard data/compute libraries) with public wheels before adding project-specific runtimes.
4. **Local / Vendored Project Code**:
   `COPY` local vendored code or project binaries only after all static dependencies are locked in place.
5. **Configs and Launch Wrappers (Late in the build)**:
   `COPY` config files and generate launcher scripts near the end so configuration edits do not trigger dependency reinstallation.
6. **Environment Variables and Working Directory (Latest in the build)**:
   Place `ENV` declarations and `WORKDIR` at the very end of the file so tweaking environment settings never busts cache layers.

### 2. Mandatory Core Tools for Agent Containers
Any container intended for developer or agent interaction must include:
- `git` and `git-lfs` (initialized via `git lfs install --system`).
- Coding agents rely heavily on inspecting git repository state, diffs, log histories, and LFS metadata.

---

## 3. Real-World Case Study: `lmcoder` Container Optimization

In `lmcoder` ([`scripts/agent-canaries/multi/Containerfile`](file:///home/uwe/projects/lmcoder/scripts/agent-canaries/multi/Containerfile) and single-agent containerfiles), container builds were optimized using this pattern:

### Before (Naive Ordering)
- Vendored `prime-agent` was copied and pip packages installed *before* global npm packages (`opencode-ai`, `@openai/codex`, `@earendil-works/pi-coding-agent`).
- `ENV PRIME_AGENT_KERNEL_PYTHON=...` was declared mid-file.
- `git` and `git-lfs` were absent, causing basic `git status` checks in agent sessions to fail.
- Any change to vendored source invalidated npm and Python wheel downloads, taking minutes to rebuild.

### After (Cache-Optimized Ordering)
```dockerfile
FROM node:22-slim

# 1. System packages & git/lfs tooling (heavy, rarely changes)
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/local/bin/
RUN apt-get update && apt-get install -y --no-install-recommends \
      git \
      git-lfs \
      ca-certificates \
      python3 \
      python3-venv \
      python3-pip \
    && git lfs install --system \
    && rm -rf /var/lib/apt/lists/*

# 2. Static third-party npm packages (cached across local code edits)
RUN npm install -g opencode-ai @openai/codex
RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent

# 3. Static Python packages (cached across local vendoring edits)
RUN uv venv /opt/prime-agent-kernel-venv --python python3 \
  && uv pip install --python /opt/prime-agent-kernel-venv/bin/python \
       ipykernel dill requests httpx pyyaml tomli python-dotenv pandas numpy scipy beautifulsoup4 lxml pydantic tyro \
  && chmod -R 755 /opt/prime-agent-kernel-venv

# 4. Local vendored code & runtime
COPY --from=vendor . /opt/prime-agent
RUN printf '#!/bin/sh\nexec node /opt/prime-agent/dist/bundle/cli.js "$@"\n' > /usr/local/bin/prime-agent \
  && chmod +x /usr/local/bin/prime-agent \
  && uv pip install --python /opt/prime-agent-kernel-venv/bin/python /opt/prime-agent/dist/prime-agent-runtime \
  && chmod -R 755 /opt/prime-agent-kernel-venv

# 5. Configurations and launch wrappers (late in the build)
COPY configs/opencode.json          /etc/opencode-agent/opencode.json
COPY configs/pi-models.json         /etc/pi-agent/models.json
COPY configs/prime-agent-models.json /etc/prime-agent/models.json
COPY configs/codex-config.toml      /etc/codex-agent/config.toml
RUN chmod 755 /etc/pi-agent && chmod 644 /etc/pi-agent/models.json \
  && chmod 755 /etc/prime-agent && chmod 644 /etc/prime-agent/models.json \
  && chmod 755 /etc/codex-agent && chmod 644 /etc/codex-agent/config.toml \
  && chmod 755 /etc/opencode-agent && chmod 644 /etc/opencode-agent/opencode.json

RUN printf '...' > /usr/local/bin/pi-launch && chmod +x /usr/local/bin/pi-launch
RUN printf '...' > /usr/local/bin/prime-agent-launch && chmod +x /usr/local/bin/prime-agent-launch
RUN printf '...' > /usr/local/bin/codex-launch && chmod +x /usr/local/bin/codex-launch
RUN printf '...' > /usr/local/bin/opencode-launch && chmod +x /usr/local/bin/opencode-launch

# 6. Environment settings (deferred to end)
ENV PRIME_AGENT_KERNEL_PYTHON=/opt/prime-agent-kernel-venv/bin/python

WORKDIR /work
```

### Result
- **Incremental build time**: Dropped to ~2 seconds when configs or wrapper scripts are updated.
- **Tooling**: `git --version`, `git lfs version`, and `git status` function out-of-the-box in the mounted `/work` directory.

---

## 4. Acceptance Criteria

- [ ] Create `docs/Containerfile.md` (or `docs/Docker.md`) documenting container build efficiency and layer ordering rules.
- [ ] Bundle `Containerfile.md` into harnez default docs distribution and register it in `AGENTS.md` language/pipeline conventions.
- [ ] Document the requirement to include `git` and `git-lfs` for any interactive / agent-facing container image.
- [ ] Provide clear before/after examples demonstrating proper layer caching.
- [ ] Add a linter or test checking container conventions where applicable.
