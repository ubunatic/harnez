---
title: Containerfile Build Efficiency
weight: 40
---

<!-- harnez:bundled -->
# Containerfile.md — Fast, Cached, and Incremental Container Builds

Containerfiles (Dockerfiles) build efficiently only when each instruction is placed by how
often its inputs change. Put slow-changing, expensive layers first; put fast-changing layers
last. Get this order wrong and a one-line config edit reinstalls every OS package and every
`npm`/`pip` dependency on the next build.

---

## The ordering rule: static first, dynamic last

Order every Containerfile from slowest-changing to fastest-changing:

1. **Base image & system tools** — `apt-get`/`apk` packages, `ca-certificates`, and core
   developer tools (`git`, `git-lfs`). Changes almost never.
2. **Static package-manager runtimes** — global `npm`, `pip`/`uv`, or binary dependencies
   pulled from public registries. Changes when a dependency version bumps, not on every edit.
3. **Pre-built environments** — virtualenvs or other prebuilt runtimes assembled from public
   wheels/packages, before any project-specific code touches them.
4. **Local / vendored project code** — `COPY` local source or vendored binaries only after all
   static dependencies are locked in. This is the layer that changes on every commit.
5. **Configs and launch wrappers** — `COPY` config files and generate launcher scripts near the
   end, so a config tweak never busts a dependency-install layer.
6. **`ENV` and `WORKDIR`** — place last. Tweaking an environment variable should invalidate at
   most the final layer.

Each `RUN`/`COPY` instruction creates a cache layer keyed on its own inputs plus every layer
before it. A change at step N invalidates steps N through 6, but never 1 through N-1 — so
putting frequently-changed things last maximizes what stays cached.

## Worked example: `scripts/agent-canary/Containerfile`

This repo's own `scripts/agent-canary/Containerfile` follows the rule and is a good template
to copy from:

```dockerfile
FROM golang:1.24-bookworm AS harnez-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -buildvcs=false -o /out/harnez ./cmd/harnez

FROM node:22-slim

# System tools required by harnez-managed workspaces and agent canary probes.
RUN apt-get update && apt-get install -y --no-install-recommends \
      git \
      git-lfs \
      ca-certificates \
    && git lfs install --system \
    && rm -rf /var/lib/apt/lists/*

# Keep agent package installs in separate layers for targeted cache reuse.
RUN npm install -g opencode-ai

# Pi needs --ignore-scripts, matching the lmcoder canary image finding.
RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent

COPY --from=harnez-build /out/harnez /usr/local/bin/harnez

# Runtime templates are expanded by launchers so tests can use a dedicated
# local proxy port without rebuilding the image.
COPY scripts/agent-canary/configs/opencode.json.template /etc/harnez-agent-canary/opencode.json.template
COPY scripts/agent-canary/configs/pi-models.json.template /etc/harnez-agent-canary/pi-models.json.template
COPY scripts/agent-canary/bin/opencode-launch /usr/local/bin/opencode-launch
COPY scripts/agent-canary/bin/pi-launch /usr/local/bin/pi-launch
COPY scripts/agent-canary/bin/harnez-agent-canary-check /usr/local/bin/harnez-agent-canary-check
COPY scripts/agent-canary/bin/harnez-agent-canary-hook-check /usr/local/bin/harnez-agent-canary-hook-check

RUN chmod 755 /usr/local/bin/harnez ... \
    && chmod 755 /etc/harnez-agent-canary \
    && chmod 644 /etc/harnez-agent-canary/*.template

ENV HARNEZ_AGENT_CANARY_PROXY_PORT=8736 \
    HARNEZ_AGENT_CANARY_CONTEXT_WINDOW=8192 \
    HOME=/tmp/harnez-home \
    ...

WORKDIR /work
```

What it does right:

- **Multi-stage build**: a `golang:1.24-bookworm` builder stage compiles `harnez`, and only the
  resulting binary is `COPY --from=harnez-build` into the slim `node:22-slim` runtime stage. The
  Go toolchain, module cache, and full source tree never end up in the shipped image.
- **`go.mod`/`go.sum` copied before the rest of the source**: `RUN go mod download` caches as
  long as dependencies don't change, even though application source changes on every commit.
  Compare with `COPY . .` immediately followed by `go build`, which is placed right after (step
  4 — local project code, so its own cache invalidation on every edit is expected and fine).
- **`apt-get install` before any `npm install`**: system packages rarely change; the `npm`
  layers below them change more often (a new agent CLI, a version bump) but still less often
  than the harnez binary itself.
- **One `RUN npm install -g` per package group, not combined**: `opencode-ai` and
  `@earendil-works/pi-coding-agent` are separate `RUN` layers "for targeted cache reuse" (see
  the comment in the file). If one package needs a version bump or a different install flag
  (`--ignore-scripts` for Pi), only that layer rebuilds — not both.
- **Configs and launch scripts copied after the binary**: editing
  `scripts/agent-canary/bin/pi-launch` never triggers a re-run of `npm install` or `go build`.
- **`ENV` and `WORKDIR` last**: adjusting `HARNEZ_AGENT_CANARY_CONTEXT_WINDOW` invalidates only
  the final layer.
- **`apt-get update && ... && rm -rf /var/lib/apt/lists/*` in one `RUN`**: keeps the apt cache
  out of the image layer instead of leaving it as dead weight in an earlier, cached layer that a
  later `RUN rm` can't shrink (deleting in a later layer doesn't reduce the image size — the
  data still exists in the earlier layer).

## Mandatory tools for agent/developer-facing containers

Any container meant for interactive developer or coding-agent use must include `git` and
`git-lfs`, initialized with `git lfs install --system` in the same `RUN` that installs them
(step 1, base image & system tools). Coding agents routinely run `git status`, `git diff`, and
`git log` to inspect workspace state — omitting these tools breaks that inspection with a
runtime error instead of a build-time signal, which is harder to diagnose. See
`scripts/agent-canary/Containerfile` above for the exact pattern.

## Other cache- and speed-relevant practices

- **Pin base image tags.** `FROM node:22-slim` and `FROM golang:1.24-bookworm` pin a major
  version, not `latest` — `latest` silently changes the base layer's contents between builds,
  which both breaks reproducibility and defeats caching (a new `latest` digest invalidates every
  layer after `FROM`).
- **Minimize build context with `.dockerignore`.** The build context (everything under the
  directory passed to `podman build`/`docker build`) is hashed and sent to the daemon before any
  instruction runs; a `.dockerignore` excluding `.git/`, build artifacts, and `node_modules/`
  keeps context transfer and cache-key computation fast, especially for `COPY . .` steps.
- **Order `RUN` steps by volatility, not by topic.** Grouping "all npm installs" into a single
  `RUN` with `&&` looks tidy but forces every package in that group to reinstall whenever any one
  of them changes. Split `RUN` per install group when the packages version-bump independently
  (as `scripts/agent-canary/Containerfile` does for `opencode-ai` vs.
  `@earendil-works/pi-coding-agent`); combine only steps that always change together (e.g.
  `apt-get update && apt-get install && rm -rf /var/lib/apt/lists/*`, which must stay one layer
  to avoid leaving apt's package lists in the image).
- **Copy dependency manifests before source.** `COPY go.mod go.sum ./` (or `package.json` /
  `package-lock.json`, `pyproject.toml` / `uv.lock`) followed by the install step, then
  `COPY . .` for the rest of the source, is the general form of the pattern used above for Go
  modules — it applies the same way to `npm ci` or `uv pip install -r requirements.txt`.
- **Use multi-stage builds to keep the shipped image small.** A build-only stage (compiler
  toolchain, source tree, intermediate artifacts) never needs to exist in the final image;
  `COPY --from=<stage>` pulls across only the built artifact.
- **Balance layer count against cache granularity.** Fewer layers can mean a smaller image, but
  each `&&`-joined `RUN` becomes one atomic cache unit — merge only steps whose inputs always
  change together; keep independent installs in separate `RUN` instructions so an edit to one
  doesn't force a rebuild of all of them.

## Checklist for a new Containerfile

- [ ] Base image tag is pinned (no `:latest`).
- [ ] `git` and `git-lfs` installed (with `git lfs install --system`) if the container is
      developer/agent-facing.
- [ ] Dependency manifests (`go.mod`/`go.sum`, `package.json`/lockfile, etc.) copied and
      installed before the rest of the source is copied in.
- [ ] Local/vendored source `COPY`'d only after static dependency layers.
- [ ] Config files and launcher scripts `COPY`'d after dependency installs, not before.
- [ ] `ENV` and `WORKDIR` placed at the end of the file.
- [ ] Multi-stage build used if a compiler toolchain or build-only tooling is needed.
- [ ] `.dockerignore` present and excludes `.git/`, build artifacts, and other large
      build-context noise.
- [ ] Related independent package-install groups kept in separate `RUN` layers; only
      always-together steps (e.g. `apt-get update && install && cleanup`) are combined.
