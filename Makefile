.PHONY: ⚙️  # make all commands phony
BINARY  := claudeconfig
CONFIG  := config.yaml
TARGET  := $(HOME)/.claude
PROJECT := .
PREFIX  ?= /usr/local

help: ⚙️  ## show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ⚙️  ## build the binary
	go build -o $(BINARY) .

install: ⚙️ build  ## install the binary to PREFIX/bin (default: /usr/local/bin)
	go install .
	@sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY) && echo "✅ Installed for all users" || echo "⚠️ System istall failed"

apply: ⚙️ build  ## apply config.yaml to the Claude Code config directory
	./$(BINARY) apply -c $(CONFIG) -t $(TARGET) -p $(PROJECT)

diff: ⚙️ build  ## show what apply would change in managed blocks
	./$(BINARY) diff -c $(CONFIG) -t $(TARGET)

clean: ⚙️ build  ## remove managed blocks from the Claude Code config directory
	./$(BINARY) clean -c $(CONFIG) -t $(TARGET)

status: ⚙️ build  ## show config summary and applied state
	./$(BINARY) status -c $(CONFIG) -t $(TARGET)

test: ⚙️  ## run linter and tests
	go vet ./...
	go test ./...
