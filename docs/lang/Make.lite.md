<!-- harnez:variant=lite -->
# Make Conventions (Lite)

Default language assumed: Go. Apply to other languages accordingly.

## Structure

```makefile
# - first target is the default goal — always `help` (bare `make` prints usage)
# - all targets declared .PHONY via the ⚙️ sentinel trick (below), never per-target names
# - one blank line between targets
```

## Variables

```makefile
BINARY  := harnez              # output binary name       (:= immediate assignment, most vars)
CONFIG  := config.yaml         # default config file
TARGET  := $(HOME)/.claude     # installation target dir
PROJECT := .                   # project root (passed to tool as -p)
PREFIX  ?= /usr/local          # ?= for env-overridable vars

export MYAPP_SOME_FEATURE=1
# align the = signs for readability
```

## Phony declaration — `⚙️ 🤖` sentinels

```makefile
.PHONY: ⚙️ 🤖  # ⚙️ = manual/once, 🤖 = managed

# A sentinel as prerequisite on every target (`build: ⚙️  # ...`, `help: 🤖  # ...`)
# makes Make treat all targets as phony without listing each name twice.
# 🤖 = actively managed (reconciled/updated) by harnez, like `help`
# ⚙️ = defined/generated manually once (build, test, release); harnez never
#      automatically overwrites these
```

## Self-documenting help target

```makefile
_prim := \033[36m
_rst  := \033[0m

help: 🤖  # show this help
	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'

# every target appearing in help carries a `  # description` comment on its rule
# header line; help scrapes them automatically
```

## Build dependency pattern

```makefile
build: ⚙️  # build the binary
	go build -o $(BINARY) .

apply: ⚙️ build  # apply config.yaml to the Claude Code config directory
	./$(BINARY) apply -c $(CONFIG) -t $(TARGET) -p $(PROJECT)

# - action targets depend on `build` so the binary is always fresh
# - `build` rebuilds only when sources change (Make's normal rules apply)
# - always invoke ./$(BINARY) — the locally-built binary, not $PATH's;
#   the user may override this rule if development is close to his system
```

## Install target (Go)

```makefile
install: ⚙️ build  # install the binary to PREFIX/bin (default: /usr/local/bin)
	go install .
	@sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY) && \
	  echo "✅ Installed for all users" || echo "⚠️ System install failed"

# approach: do local + try global
# - go install           -> $(GOPATH)/bin (user-local)
# - sudo install -m 0755 -> $(PREFIX)/bin (system-wide)
# - || echo …            -> degrades gracefully when sudo is unavailable
```

## Check target

```makefile
check: ⚙️  # run static analysis and tests
	go vet ./...
	go test ./...

check-fast: ⚙️  # fast local feedback loop
	go test ./...

test: ⚙️ check  # alias for check

# - `make check` is the standard verification target used by development-flow docs
# - always run `go vet` before `go test`; vet catches what tests may not exercise
# - keep `make test` as a compatibility alias when a repo already exposes it
# - add `check-fast` when full checks are slow: broad coverage, cheap settings
#   (e.g. trafficsim runs one focused model via MODEL=...)
```

## Deployment target parity

```makefile
# Any project with mutating provisioners (pushes a binary, config, or schedule to a
# remote host) must expose these same four self-documenting targets, so deploy/verify
# is never ad-hoc SSH one-liners. Install the optional `deployment-transparency`
# practice for why run/status must probe the live host instead of inferring success
# from a completed deploy.

deploy: ⚙️ build  # deploy binary, configs, and cron schedules (DRY=1 for dry-run)
	@scripts/deploy.sh $(if $(DRY),--dry-run)

run: ⚙️  # query live deployment health (process, service, or job status)
	@scripts/deploy.sh --status

status: ⚙️ run  # alias for run

backup: ⚙️  # sync state snapshots from the remote host
	@scripts/backup.sh

# - deploy [DRY=1]  ships binary, configs, cron/systemd schedules; DRY=1 must be a
#                   REAL dry-run against the remote host, not a no-op
# - run / status    read-only: query the actual remote process table, systemd units,
#                   or crontab — never local repo state; keep `status` as an alias
#                   when a repo already exposes `run`
# - backup          sync state snapshots (config overlays, data) down from the remote
#                   host before a risky deploy
# - all four query or mutate a real remote host — treat like `make smoke`
#   (@docs/AgenticLoop.md): safe to define, run only when you intend the live effect
```
