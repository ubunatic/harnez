# Environment Setup Runbook

This document describes how an agent or developer sets up, builds, runs, and tests the `harnez` CLI.

## 1. Overview

`harnez` is a Go-based agentic workspace manager and developer CLI. To build and test `harnez` successfully, an agent environment requires:

- **Go Toolchain** (>= 1.24)
- **GNU Make**
- **Git** and **Git LFS**
- **Minisign** (required for cryptographic release verification tests in `internal/release`)

---

## 2. Automated Environment Setup

The repository provides an automated setup script that verifies toolchains, installs required system dependencies (`minisign`), and pre-downloads Go modules.

To execute the setup:

```bash
bash scripts/setup-env.sh
```

### What `scripts/setup-env.sh` does:

1. **Validates Build Tools**: Verifies presence of GNU `make` and `git`.
2. **Checks Go Version**: Ensures `go` is installed and reports the Go toolchain version.
3. **Installs Minisign**: Automatically installs `minisign` via `apt-get` (Linux) or `brew` (macOS) if not present.
4. **Initializes Git LFS**: Ensures Git LFS hooks are active if `git-lfs` is installed.
5. **Downloads Dependencies**: Runs `go mod download` to prime the Go module cache.

---

## 3. Building the `harnez` CLI

Once the environment is prepared, build the local `harnez` binary:

```bash
# Build binary in local root as ./harnez
make build

# Verify build output
./harnez status
./harnez --help
```

To install the binary to user PATH (`~/go/bin/harnez`):

```bash
make install
```

---

## 4. Testing & Verification Workflows

`harnez` includes several testing targets in the `Makefile`:

| Target | Command | Purpose |
| :--- | :--- | :--- |
| `make check-fast` | `go test ./...` | Fast local feedback loop during development. |
| `make check` / `make test` | `gofmt` + `go vet` + `go test` | Full static analysis and unit/integration test suite. |
| `make smoke` | `bash scripts/smoke-test.sh` | Live smoke test verifying apply, status, diff, and drift repair. |
| `make test-q1` | `harnez exec --quota-1 -- make test` | Test suite executed under Quota-1 guardrail enforcement. |

### Running Unit & Integration Tests

```bash
# Run full check (vet + unit tests with GOWORK=off)
make check
```

---

## 5. Containerized Proof & Setup (Podman / Docker)

To verify that `harnez` can be built, run, and tested in a clean isolated container environment:

### Building with Podman or Docker

```bash
# Build the container image using the root Containerfile
docker build -t harnez-env .

# Or using Podman
podman build -t harnez-env .
```

### Running inside the Container

```bash
# Execute the built harnez CLI inside the container
docker run --rm harnez-env ./harnez status

# Run the complete test suite in the container
docker run --rm harnez-env make check
```

The container uses a multi-stage build following `docs/Containerfile.md`:
- Stage 1: Compiles `harnez` and downloads dependencies.
- Stage 2: Installs `minisign`, system tools, and runs `scripts/setup-env.sh` and `make check` to prove environment reproducibility.
