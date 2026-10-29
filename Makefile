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
ENGINE   ?= $(shell command -v podman || command -v docker)
TEST_IMG ?= harnez-test
TEST_OUT ?= /tmp/harnez-container-test
TESTFLAGS ?=
TEST_PARALLELISM ?= $(shell awk '$$1 == "go_test_parallelism:" {print $$2}' spec/testing.yaml)
TEST_TIMEOUT ?= $(shell awk '$$1 == "go_test_timeout:" {print $$2}' spec/testing.yaml)
COMMIT_COUNT := $(shell git rev-list --count HEAD 2>/dev/null)
VERSION_LDFLAGS := -X ubunatic.com/harnez.CommitCount=$(COMMIT_COUNT)
# rootless podman: map the caller's UID onto the image's tester user.
USERNS   := $(if $(findstring podman,$(ENGINE)),--userns=keep-id)

help: 🤖  # show this help
	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'

preflight: ⚙️  # check toolchains and dependencies
	@command -v go >/dev/null || (echo "❌ go is not installed" && exit 1)

build: ⚙️  # build the binary
	go build -ldflags '$(VERSION_LDFLAGS)' -o $(BINARY) ./cmd/harnez

run: ⚙️ build  # run the application locally
	./$(BINARY)

man: ⚙️ build  # generate roff man pages
	./$(BINARY) man --dir .

install: ⚙️ build  # install binary to ~/go/bin (user)
	go install -ldflags '$(VERSION_LDFLAGS)' ./cmd/harnez
	./$(BINARY) man --install
	@ln -sf harnez $$(go env GOPATH)/bin/⚙ 2>/dev/null || ln -sf harnez $(HOME)/go/bin/⚙


install-system: ⚙️ build  # install binary to PREFIX/bin via sudo (system-wide)
	sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)
	sudo ./$(BINARY) man --install --dir $(PREFIX)/share/man/man1
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

revert-managed: ⚙️ build  # remove managed blocks from the Claude Code config directory
	./$(BINARY) revert --managed -c $(CONFIG) -t $(TARGET)

status: ⚙️ build  # show config summary and applied state
	./$(BINARY) status -c $(CONFIG) -t $(TARGET)

lint: ⚙️ build  # gofmt, go vet, harnez lint --check on tracked files, docs/commands registration
	gofmt -l . | awk 'BEGIN {found = 0} {print; found = 1} END {exit found}'
	GOWORK=off go vet ./...
	./$(BINARY) lint --check
	bash scripts/lint.sh

install-canary: ⚙️  # verify the latest Codeberg release installs and reports its version
	bash scripts/install-canary.sh

container-test: ⚙️  # run make lint check offline in Containerfile.test (ENGINE=podman|docker; log in TEST_OUT)
	@test -n "$(ENGINE)" || (echo "❌ neither podman nor docker is installed" && exit 1)
	$(ENGINE) build -f Containerfile.test -t $(TEST_IMG) \
	   --build-arg GO_VERSION=$$(awk '$$1 == "go" { print $$2; exit }' go.mod) \
	   --build-arg TEST_UID=$$(id -u) .
	mkdir -p $(TEST_OUT)
	$(ENGINE) run --rm --init --network none $(USERNS) -v $(TEST_OUT):/out:Z $(TEST_IMG) \
	   bash -o pipefail -c 'make lint check 2>&1 | tee /out/test.log'

test-ubuntu: ⚙️ container-test  # alias for container-test (Ubuntu 24.04, git 2.43)

macos-ci: ⚙️  # trigger and watch macos-hello CI workflow on the GitHub mirror
	bash scripts/macos-ci.sh

test-pi: ⚙️  # Pi background-task extension tests (node 22.18+; fails when node is missing)
	bash scripts/test-pi.sh

check: ⚙️ test-pi  # run static analysis and tests (GOWORK=off: catch go.mod pin drift behind a local workspace override)
	gofmt -l . | awk 'BEGIN {found = 0} {print; found = 1} END {exit found}'
	GOWORK=off go vet ./...
	GOWORK=off go test $(TESTFLAGS) -p $(TEST_PARALLELISM) -timeout $(TEST_TIMEOUT) ./...

check-fast: ⚙️  # fast local feedback loop
	go test ./...

test-short: ⚙️ test-pi  # fast unit tier: go test -short (skips PTY/media/integration tests) and the Pi extension tests
	GOWORK=off go test -short $(TESTFLAGS) -p $(TEST_PARALLELISM) -timeout $(TEST_TIMEOUT) ./...

test-pkg: ⚙️  # one package with -race, uncached: make test-pkg PKG=./internal/lint/...
	@test -n "$(PKG)" || (echo "❌ usage: make test-pkg PKG=./path/..." && exit 1)
	GOWORK=off go test -race -count=1 $(TESTFLAGS) -p $(TEST_PARALLELISM) -timeout $(TEST_TIMEOUT) $(PKG)

smoke: ⚙️ build  # live smoke: apply/diff/repair + usage bar-alignment against real binary
	bash scripts/smoke-test.sh

smoke-container: ⚙️  # build both agent images and run the in-pod Pi/Codex smoke test (LMCODER_MODEL_DIR=reuse host models, LMCODER_CACHE_DIR=download to a host dir)
	bash scripts/agent-canary/run.sh

test: ⚙️ check  # full tier: alias for check (gofmt, vet, all tests)

test-dot8: ⚙️  # run tests with dot8 build tag
	GOWORK=off go test -tags dot8 -p $(TEST_PARALLELISM) -timeout $(TEST_TIMEOUT) ./...


format: ⚙️  # format source code
	go fmt ./...

release: check ⚙️  # release the project using harnez
	./$(BINARY) release

sync: ⚙️  # push here, pull there, build there, verify
	git push
	ssh $(HOST) "cd $(HOST_DIR) && git pull && make install && make status"

test-q1: 🤖  # run tests under Quota-1 enforcement
	harnez exec --quota-1 -- $(MAKE) test
