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
# rootless podman: map the caller's UID onto the image's tester user.
USERNS   := $(if $(findstring podman,$(ENGINE)),--userns=keep-id)

help: 🤖  # show this help
	@grep -E '^[a-zA-Z_-]+:.*[⚙🤖].*#+' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*#+ "}; {printf "    $(_prim)%-15s$(_rst) %s\n", $$1, $$2}'

preflight: ⚙️  # check toolchains and dependencies
	@command -v go >/dev/null || (echo "❌ go is not installed" && exit 1)

build: ⚙️  # build the binary
	go build -o $(BINARY) ./cmd/harnez

run: ⚙️ build  # run the application locally
	./$(BINARY)

man: ⚙️ build  # generate roff man pages
	./$(BINARY) man --dir .

install: ⚙️ build  # install binary to ~/go/bin (user)
	go install ./cmd/harnez
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

lint: ⚙️  # check docs/commands/*.md files are all registered in config.yaml
	bash scripts/lint.sh

install-canary: ⚙️  # verify the latest Codeberg release installs and reports its version
	bash scripts/install-canary.sh

container-test: ⚙️  # run make check offline in Containerfile.test (ENGINE=podman|docker; log in TEST_OUT)
	@test -n "$(ENGINE)" || (echo "❌ neither podman nor docker is installed" && exit 1)
	$(ENGINE) build -f Containerfile.test -t $(TEST_IMG) \
	   --build-arg GO_VERSION=$$(awk '$$1 == "go" { print $$2; exit }' go.mod) \
	   --build-arg TEST_UID=$$(id -u) .
	mkdir -p $(TEST_OUT)
	$(ENGINE) run --rm --init --network none $(USERNS) -v $(TEST_OUT):/out:Z $(TEST_IMG) \
	   bash -o pipefail -c 'make check 2>&1 | tee /out/test.log'

test-ubuntu: ⚙️ container-test  # alias for container-test (Ubuntu 24.04, git 2.43)

macos-ci: ⚙️  # trigger and watch macos-hello CI workflow on the GitHub mirror
	bash scripts/macos-ci.sh

check: ⚙️  # run static analysis and tests (GOWORK=off: catch go.mod pin drift behind a local workspace override)
	gofmt -l . | awk 'BEGIN {found = 0} {print; found = 1} END {exit found}'
	GOWORK=off go vet ./...
	GOWORK=off go test $(TESTFLAGS) ./...

check-fast: ⚙️  # fast local feedback loop
	go test ./...

smoke: ⚙️ build  # live smoke: apply/diff/repair + usage bar-alignment against real binary
	bash scripts/smoke-test.sh

smoke-container: ⚙️  # build both agent images and run the in-pod Pi/Codex smoke test (LMCODER_MODEL_DIR=reuse host models, LMCODER_CACHE_DIR=download to a host dir)
	bash scripts/agent-canary/run.sh

test: ⚙️ check  # alias for check

test-dot8: ⚙️  # run tests with dot8 build tag
	GOWORK=off go test -tags dot8 ./...


format: ⚙️  # format source code
	go fmt ./...

release: check ⚙️  # release the project using harnez
	./$(BINARY) release

sync: ⚙️  # push here, pull there, build there, verify
	git push
	ssh $(HOST) "cd $(HOST_DIR) && git pull && make install && make status"

test-q1: 🤖  # run tests under Quota-1 enforcement
	harnez exec --quota-1 -- $(MAKE) test
