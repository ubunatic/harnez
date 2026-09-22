# Containerfile to proof building, testing, and running harnez in Podman/Docker.

FROM golang:1.24-bookworm AS builder

ENV GOTOOLCHAIN=auto

# 1. Install system dependencies & setup tools
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential \
      ca-certificates \
      git \
      git-lfs \
      minisign \
    && git lfs install --system \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# 2. Copy dependency manifests first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# 3. Copy full source tree and setup scripts
COPY . .

# 4. Run setup script to verify environment automation
RUN bash scripts/setup-env.sh

# 5. Build harnez CLI
RUN make build

# 6. Run full static analysis and unit test suite
RUN make check

# 7. Proof CLI execution
RUN ./harnez status && ./harnez --help

FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      git \
      git-lfs \
      minisign \
    && git lfs install --system \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /src/harnez /usr/local/bin/harnez

ENTRYPOINT ["/usr/local/bin/harnez"]
CMD ["status"]
