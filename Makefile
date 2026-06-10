.PHONY: ⚙️  # make all commands phony
BINARY  := claudeconfig
CONFIG  := config.yaml
TARGET  := $(HOME)/.claude
PROJECT := .
LANGS   := golang bash make
PREFIX  ?= /usr/local

help: ⚙️  ## show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ⚙️  ## build the binary
	go build -o $(BINARY) .

install: ⚙️ build  ## install binary to ~/go/bin (user)
	go install .

install-system: ⚙️ build  ## install binary to PREFIX/bin via sudo (system-wide)
	sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)

apply: ⚙️ build  ## apply config to ~/.claude + this project (golang bash make)
	./$(BINARY) apply -c $(CONFIG) -t $(TARGET) -p $(PROJECT) $(addprefix -l ,$(LANGS))

apply-system: ⚙️ build  ## apply config to ~/.claude only (no project, all langs)
	./$(BINARY) apply -t $(TARGET)

diff: ⚙️ build  ## show what apply would change in managed blocks
	./$(BINARY) diff -c $(CONFIG) -t $(TARGET)

clean: ⚙️ build  ## remove managed blocks from the Claude Code config directory
	./$(BINARY) clean -c $(CONFIG) -t $(TARGET)

status: ⚙️ build  ## show config summary and applied state
	./$(BINARY) status -c $(CONFIG) -t $(TARGET)

test: ⚙️  ## run linter and tests
	go vet ./...
	go test ./...
