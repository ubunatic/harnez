---
title: Go Release Pipeline
weight: 50
---

# Go Release Pipeline & Non-Interactive Signing

This document establishes the canonical standard for releasing Go CLI applications and sibling projects across the workspace using `harnez release`, `goreleaser`, `minisign`, and Forgejo (`fj`).

---

## 1. Quick Start / Prerequisites

Ensure the following tools are installed and present in `$PATH`:
- `git`
- `go`
- `goreleaser` (`go install github.com/goreleaser/goreleaser/v2@latest`)
- `minisign`
- `fj` (`forgejo-cli` / authenticated to Codeberg)
- `harnez` (`ubunatic.com/harnez`)

---

## 2. Project Setup Checklist

To enable clean, automated releases in any repository:

### 1. `version.yaml` (Single Source of Truth)
Place a `version.yaml` specification at the repository root:
```yaml
# yaml-language-server: $schema=spec/schemas/version.schema.json
version: 0.1.0
```

`harnez release` reads and bumps `version.yaml`, then automatically updates language-native version files:
- **Go**: `version.go` (`var Version = "0.1.0"`)
- **Python**: `__version__.py` / `version.py` / `pyproject.toml`
- **Zig**: `build.zig.zon` / `version.zig`
- **Rust**: `Cargo.toml`

### 2. Cobra Root Command Wiring (Go)
In `cmd/<binary>/main.go`, wire the Cobra root command version:
```go
root := &cobra.Command{
    Use:     "mytool",
    Version: Version,
    Short:   "...",
}
```

### 3. Passwordless Minisign Key
Generate a dedicated, non-interactive signing key pair:
```bash
minisign -G -W -f -p ~/.minisign/<project>.pub -s ~/.minisign/<project>.key
```
`harnez release` auto-detects `~/.minisign/<project>.key` by project directory name. Alternatively, supply `--sign-key <path>` or `-s <path>`.

### 4. `.goreleaser.yaml`
Scaffold standard GoReleaser v2 configuration:
```yaml
version: 2

project_name: <project>

builds:
  - id: <project>-linux
    main: ./cmd/<project> # or . if single main.go
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    binary: <project>
    ldflags:
      - -s -w -X main.Version={{.Version}}

archives:
  - id: default
    formats:
      - tar.gz
    name_template: "<project>-{{ .Version }}-{{ if eq .Arch \"amd64\" }}x86_64{{ else if eq .Arch \"arm64\" }}aarch64{{ else }}{{ .Arch }}{{ end }}-linux"
    files:
      - README.md

  - id: binary
    formats:
      - binary
    name_template: "<project>-{{ if eq .Arch \"amd64\" }}x86_64{{ else if eq .Arch \"arm64\" }}aarch64{{ else }}{{ .Arch }}{{ end }}-linux"

checksum:
  name_template: SHA256SUMS
  algorithm: sha256

signs:
  - cmd: sh
    args:
      - -c
      - >-
        if [ -n "${MINISIGN_PASSWORD}" ]; then
          echo "${MINISIGN_PASSWORD}" | minisign -S -s "${MINISIGN_KEY_FILE}" -m "${artifact}" -t "<project> {{ .Version }}";
        else
          minisign -S -s "${MINISIGN_KEY_FILE}" -m "${artifact}" -t "<project> {{ .Version }}";
        fi
    artifacts: checksum
    signature: "${artifact}.minisig"
    env:
      - MINISIGN_KEY_FILE
      - MINISIGN_PASSWORD
```

### 5. `Makefile` Target
Add the standard thin release recipe:
```makefile
release: check ⚙️  # release the project using harnez
	harnez release
```

---

## 3. Remote Forgejo / Codeberg Verification

Forgejo returns a `404 Not Found` to `fj release` if the repository's Releases feature is toggled off.

`harnez release` automatically queries `GET /api/v1/repos/<owner>/<repo>` using `$CODEBERG_TOKEN` / `$FORGEJO_TOKEN` and auto-enables `has_releases` via API if needed.

You can also manually verify or enable it via API:
```bash
curl -X PATCH -H "Authorization: token $CODEBERG_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"has_releases": true}' \
     https://codeberg.org/api/v1/repos/<owner>/<repo>
```

---

## 4. Releasing & Recovery

### Standard Release (Bumps Patch by default)
```bash
make release
# or
harnez release
```

### Automated / Pre-Specified Bump
```bash
harnez release --bump patch   # or minor / major / 1.0.0
```

### Dry Run (Preview Execution)
```bash
harnez release --dry-run
```

### Resuming Failed / Interrupted Uploads
If GoReleaser succeeds or tags are pushed but publishing encounters a network/forge error, **do not delete or move tags**. Resume safely:
```bash
harnez release --continue
```
