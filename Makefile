.PHONY: ⚙️ 🤖  # ⚙️ = manual/once, 🤖 = managed

_prim := \033[36m
_rst  := \033[0m

BINARY   ?= harnez
CONFIG   := config.yaml
TARGET   := $(HOME)/.claude
PROJECT  := .
LANGS    := golang bash make git
PREFIX   ?= /usr/local
HOST     ?= um760
HOST_DIR ?= projects/harnez

help: 🤖  # show this help
	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'

preflight: ⚙️  # check toolchains and dependencies
	@command -v go >/dev/null || (echo "❌ go is not installed" && exit 1)

build: ⚙️  # build the binary
	go build -o $(BINARY) ./cmd/harnez

run: ⚙️ build  # run the application locally
	./$(BINARY)

install: ⚙️ build  # install binary to ~/go/bin (user)
	go install ./cmd/harnez
	@ln -sf harnez $$(go env GOPATH)/bin/⚙ 2>/dev/null || ln -sf harnez $(HOME)/go/bin/⚙


install-system: ⚙️ build  # install binary to PREFIX/bin via sudo (system-wide)
	sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)
	sudo ln -sf $(BINARY) $(PREFIX)/bin/⚙

uninstall: ⚙️  # remove installed binary from system and user paths
	rm -f $(shell which $(BINARY) 2>/dev/null) $(PREFIX)/bin/$(BINARY) $(PREFIX)/bin/⚙ $(HOME)/go/bin/⚙ $$(go env GOPATH)/bin/⚙ 2>/dev/null

apply: ⚙️ build  # apply config to ~/.claude globally
	./$(BINARY) apply -c $(CONFIG) -t $(TARGET)

init: ⚙️ build  # initialize or update this project
	./$(BINARY) init -c $(CONFIG) -d $(PROJECT) -y

init-siblings: ⚙️ build  # run harnez init across all sibling projects
	@for dir in $$(find .. -maxdepth 1 -mindepth 1 -type d | sort); do \
		if test -d "$$dir/.git" || test -f "$$dir/AGENTS.md" || test -f "$$dir/CLAUDE.md"; then \
			if test "$$dir" != "../archive" && test "$$dir" != "../videos"; then \
				echo "=== Updating $$dir ==="; \
				./$(BINARY) init -d "$$dir" -y || true; \
			fi; \
		fi; \
	done

diff: ⚙️ build  # show what apply would change in managed blocks
	./$(BINARY) diff -c $(CONFIG) -t $(TARGET)

clean: ⚙️ build  # remove managed blocks from the Claude Code config directory
	./$(BINARY) revert --managed -c $(CONFIG) -t $(TARGET)

status: ⚙️ build  # show config summary and applied state
	./$(BINARY) status -c $(CONFIG) -t $(TARGET)

lint: ⚙️  # check docs/commands/*.md files are all registered in config.yaml
	bash scripts/lint.sh

agent-canary-build: ⚙️ build  # build the shared Pi/OpenCode canary container
	bash scripts/agent-canary/run.sh build

agent-canary-static: ⚙️ build  # run deterministic in-container canary checks
	bash scripts/agent-canary/run.sh static

install-canary: ⚙️  # verify the latest Codeberg release installs and reports its version
	bash scripts/install-canary.sh

macos-ci: ⚙️  # trigger and watch macos-hello CI workflow on the GitHub mirror
	bash scripts/macos-ci.sh

check: ⚙️  # run static analysis and tests (GOWORK=off: catch go.mod pin drift behind a local workspace override)
	gofmt -l . | awk 'BEGIN {found = 0} {print; found = 1} END {exit found}'
	GOWORK=off go vet ./...
	GOWORK=off go test ./...

check-fast: ⚙️  # fast local feedback loop
	go test ./...

smoke: ⚙️ build  # live smoke: apply/diff/repair + usage bar-alignment against real binary
	bash scripts/smoke-test.sh

test: ⚙️ check  # alias for check

format: ⚙️  # format source code
	go fmt ./...

release: check ⚙️  # release the project using harnez
	./$(BINARY) release

sync: ⚙️  # push here, pull there, build there, verify
	git push
	ssh $(HOST) "cd $(HOST_DIR) && git pull && make install && make status"

test-q1: 🤖  # run tests under Quota-1 enforcement
	harnez exec --quota-1 -- $(MAKE) test
