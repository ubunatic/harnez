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

install-system: ⚙️ build  # install binary to PREFIX/bin via sudo (system-wide)
	sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall: ⚙️  # remove installed binary from system and user paths
	rm -f $(shell which $(BINARY) 2>/dev/null) $(PREFIX)/bin/$(BINARY)

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
	./$(BINARY) clean -c $(CONFIG) -t $(TARGET)

status: ⚙️ build  # show config summary and applied state
	./$(BINARY) status -c $(CONFIG) -t $(TARGET)

lint: ⚙️  # check commands/*.md files are all registered in config.yaml
	bash scripts/lint.sh

agent-canary-build: ⚙️ build  # build the shared Pi/OpenCode canary container
	bash scripts/agent-canary/run.sh build

agent-canary-static: ⚙️ build  # run deterministic in-container canary checks
	bash scripts/agent-canary/run.sh static

check: ⚙️  # run static analysis and tests
	go vet ./...
	go test ./...

check-fast: ⚙️  # fast local feedback loop
	go test ./...

test: ⚙️ check  # alias for check

format: ⚙️  # format source code
	go fmt ./...

release: check ⚙️  # release the project using uman
	uman release harnez

sync: ⚙️  # push here, pull there, build there, verify
	git push
	ssh $(HOST) "cd $(HOST_DIR) && git pull && make install && make status"
