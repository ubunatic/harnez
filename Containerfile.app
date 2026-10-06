# Containerfile.app — Consolidated harnez + agent container (Pi + Codex)
# Follows docs/Containerfile.md guidelines.

FROM golang:1.24-bookworm AS harnez-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -buildvcs=false -o /out/harnez ./cmd/harnez

FROM node:22-slim

# Base system tools required by harnez and developer/agent workflows
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      git \
      git-lfs \
      jq \
      minisign \
      procps \
    && git lfs install --system \
    && rm -rf /var/lib/apt/lists/*

# Keep agent package installs in separate layers for targeted cache reuse.
RUN npm install -g @openai/codex
RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent

COPY --from=harnez-build /out/harnez /usr/local/bin/harnez

COPY scripts/agent-canary/bin/pi-launch /usr/local/bin/pi-launch
COPY scripts/agent-canary/bin/codex-launch /usr/local/bin/codex-launch
COPY scripts/agent-canary/configs/pi-models.json.template /etc/harnez-agent-canary/pi-models.json.template
COPY scripts/agent-canary/configs/codex.toml.template /etc/harnez-agent-canary/codex.toml.template
COPY scripts/agent-canary/smoke.sh /usr/local/bin/smoke-test

RUN chmod 755 /usr/local/bin/harnez \
      /usr/local/bin/pi-launch \
      /usr/local/bin/codex-launch \
      /usr/local/bin/smoke-test \
    && chmod 755 /etc/harnez-agent-canary \
    && chmod 644 /etc/harnez-agent-canary/*.template

ENV HARNEZ_AGENT_CANARY_PROXY_PORT=8735 \
    HARNEZ_AGENT_CANARY_CONTEXT_WINDOW=8192 \
    HOME=/tmp/harnez-home \
    XDG_CONFIG_HOME=/tmp/harnez-home/.config \
    XDG_DATA_HOME=/tmp/harnez-home/.local/share \
    XDG_CACHE_HOME=/tmp/harnez-home/.cache \
    XDG_STATE_HOME=/tmp/harnez-home/.local/state \
    PI_CODING_AGENT_DIR=/tmp/harnez-home/.pi/agent \
    PI_CODING_AGENT_SESSION_DIR=/tmp/harnez-home/.pi/sessions

WORKDIR /work
