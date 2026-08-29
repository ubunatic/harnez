---
title: Go Release Pipeline
weight: 50
---

# Go Release Pipeline & Non-Interactive Signing

This document establishes the canonical standard for releasing Go CLI applications across the workspace using `uman release`, `goreleaser`, `minisign`, and Forgejo (`fj`).

---

## 1. Quick Start / Prerequisites

Ensure the following tools are installed and present in `$PATH`:
- `git`
- `go`
- `goreleaser` (`go install github.com/goreleaser/goreleaser/v2@latest`)
- `minisign`
- `fj` (`forgejo-cli` / authenticated to Codeberg)
- `uman` (`ubunatic/uman`)

---

## 2. Project Setup Checklist

To enable clean, automated releases in any Go repository:

### 1. `version.go`
Place a `version.go` file at the root or main package:
```go
package main

var Version = "0.1.0"
```
*In `cmd/<binary>/main.go`*, wire the Cobra root command version:
```go
root := &cobra.Command{
    Use:     "mytool",
    Version: Version,
    Short:   "...",
}
```

### 2. Passwordless Minisign Key
Generate a dedicated, non-interactive signing key pair:
```bash
minisign -G -W -f -p ~/.minisign/<project>.pub -s ~/.minisign/<project>.key
```
Add the key mapping to root `.uman.toml`:
```toml
[release.projects.<project>]
minisign_key = "~/.minisign/<project>.key"
```

### 3. `.goreleaser.yaml`
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

### 4. `Makefile` Target
Add the standard release recipe:
```makefile
release: check ⚙️  # release the project using uman
	uman release <project>
```

---

## 3. Remote Forgejo / Codeberg Verification

Forgejo returns a `404 Not Found` to `fj release` if the repository's Releases feature is toggled off.

Verify or enable it via API:
```bash
curl -X PATCH -H "Authorization: token $CODEBERG_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"has_releases": true}' \
     https://codeberg.org/api/v1/repos/<owner>/<repo>
```

---

## 4. Releasing & Recovery

### Interactive Release
```bash
make release
# or
uman release <project>
```

### Automated / Pre-Specified Bump
```bash
uman release <project> --bump patch # or minor / major / 1.0.0
```

### Resuming Failed / Interrupted Uploads
If GoReleaser succeeds or tags are pushed but publishing encounters a network/forge error, **do not delete or move tags**. Resume safely:
```bash
uman release <project> --continue
```
