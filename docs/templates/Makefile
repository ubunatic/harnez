.PHONY: ⚙️  # make all commands phony
BINARY  := myapp
PREFIX  ?= /usr/local

help: ⚙️  ## show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ⚙️  ## build the binary
	go build -o $(BINARY) .

install: ⚙️ build  ## install to ~/go/bin (user)
	go install .

test: ⚙️  ## run linter and tests
	go vet ./...
	go test ./...

clean: ⚙️  ## remove build artifacts
	rm -f $(BINARY)
