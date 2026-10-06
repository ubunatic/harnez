# Containerfile.app — harnez plus the local-LLM agents it manages (Pi, Codex).
#
# Build context is the repository root. The smoke test runs this image next to
# Containerfile.lmcoder in one podman pod: make smoke-container (issue 723).
# Layer order follows docs/Containerfile.md: static first, dynamic last.
FROM golang:1.26-bookworm AS harnez-build

ENV GOTOOLCHAIN=auto \
    GOWORK=off
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -buildvcs=false -o /out/harnez ./cmd/harnez

FROM node:22-slim

# System tools for harnez-managed workspaces and the smoke test.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      git \
      git-lfs \
      jq \
      procps \
    && git lfs install --system \
    && rm -rf /var/lib/apt/lists/*

# Latest upstream agents, one layer each for targeted cache reuse.
RUN npm install -g @openai/codex
# Pi needs --ignore-scripts, matching the lmcoder canary image finding.
RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent

COPY --from=harnez-build /out/harnez /usr/local/bin/harnez

COPY scripts/agent-canary/configs/pi-models.json.template /etc/harnez-agent-canary/pi-models.json.template
COPY scripts/agent-canary/bin/pi-launch /usr/local/bin/pi-launch
COPY scripts/agent-canary/bin/codex-launch /usr/local/bin/codex-launch
COPY scripts/agent-canary/smoke.sh /usr/local/bin/smoke-test

RUN chmod 755 /usr/local/bin/harnez \
      /usr/local/bin/pi-launch \
      /usr/local/bin/codex-launch \
      /usr/local/bin/smoke-test \
    && chmod 755 /etc/harnez-agent-canary \
    && chmod 644 /etc/harnez-agent-canary/*.template

# The model server shares the pod's loopback (Containerfile.lmcoder).
ENV HARNEZ_AGENT_CANARY_BASE_URL=http://127.0.0.1:8734/v1 \
    HARNEZ_AGENT_CANARY_CONTEXT_WINDOW=32768 \
    HOME=/home/agent \
    XDG_CONFIG_HOME=/home/agent/.config \
    XDG_DATA_HOME=/home/agent/.local/share \
    XDG_CACHE_HOME=/home/agent/.cache \
    XDG_STATE_HOME=/home/agent/.local/state \
    PI_CODING_AGENT_DIR=/home/agent/.pi/agent \
    PI_CODING_AGENT_SESSION_DIR=/home/agent/.pi/sessions

RUN mkdir -p /home/agent /work && git config --system init.defaultBranch main
WORKDIR /work

CMD ["smoke-test"]
