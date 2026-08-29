---
title: Release Pipeline
weight: 50
---

# Release Pipeline & Non-Interactive Signing

This document establishes the canonical standard for releasing projects across the
workspace using `harnez release`, `goreleaser`, `minisign`, and Forgejo (`fj`).

The pipeline is **language-agnostic**. Go is the most common case (and the source of
this file's name), but `harnez release` also detects and syncs Python, Zig, and Rust
version files, and can release projects that compile nothing at all — see
[§5 Non-Go / Scripted Projects](#5-non-go--scripted-projects).

---

## 1. Quick Start / Prerequisites

Ensure the following tools are installed and present in `$PATH`:
- `git`
- `go` (only for Go projects, and to `go install` the tools below)
- `goreleaser` (`go install github.com/goreleaser/goreleaser/v2@latest`) —
  used for packaging even by non-Go projects, see §5
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

A project with none of those files is perfectly valid: `version.yaml` alone is then the
source of truth, and the sync step is a no-op. Commit `version.yaml` (and, if the repo
runs `reuse lint`, add it to `REUSE.toml`) so the first automated bump does not trip
license linting.

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
```

Do **not** add a `signs:` block. `harnez release` invokes GoReleaser with
`--skip=publish --skip=sign` and signs `dist/SHA256SUMS` itself (see §5.3), so a
`signs:` section is dead config. Publishing is likewise owned by `fj`, not GoReleaser.

### 5. `Makefile` Target
Add the standard thin release recipe, depending on whatever the repo already calls its
test/lint target (`check`, `test`, …) — keep the project's own name, do not add an alias:
```makefile
release: check ⚙️  # release the project using harnez
	harnez release
```

`harnez release` owns build, signing, tagging, pushing, and publishing, so the older
hand-rolled `deps` / `release-build` / `release-publish` / `release-full` target sets
should be deleted when adopting this convention. Keep only generic helpers such as
`clean`.

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

---

## 5. Non-Go / Scripted Projects

Nothing in the pipeline requires a compiler. A Python, shell, or asset-only project
follows the same checklist; only the version file and the packaging step differ.

### 5.1 Python version files

`harnez release` writes `version.yaml` and then syncs, if present:
`__version__.py`, `version.py` (`__version__ = "0.1.0"` / `VERSION = ...` /
`version = ...`), and `pyproject.toml`'s `[project] version`. Single-file scripts that
carry no version constant need none of these — `version.yaml` is enough.

### 5.2 Packaging with no build

GoReleaser OSS has no "prebuilt binary" builder, so a project with nothing to compile
skips building entirely and ships a `meta:` archive that packs arbitrary files. Stage the
payload in a `before.hooks` step, outside `dist/` (GoReleaser empties `dist/` for its own
use):

```yaml
version: 2
project_name: <project>

before:
  hooks:
    - make pack                       # writes dist/<artifact>
    - mkdir -p .goreleaser-src
    - cp dist/<artifact> .goreleaser-src/<project>
    - rm -rf dist

builds:
  - skip: true

archives:
  - id: default
    meta: true                        # pack files without a build
    formats: [tar.gz]
    name_template: "<project>-{{ .Version }}"
    files:
      - src: .goreleaser-src/<project>
        dst: <project>
      - LICENSES/*
      - README.md

checksum:
  name_template: SHA256SUMS
  algorithm: sha256
```

Add `.goreleaser-src/` to `.gitignore` alongside `dist/`.

Projects that need no GoReleaser at all can instead point `harnez release` at any command
that fills `dist/`:
```bash
harnez release --build-cmd "make dist"
```

### 5.3 Signing is language-agnostic

Signing happens outside GoReleaser: `harnez release` runs
`minisign -S -s <key> -m dist/SHA256SUMS -t "<project> <version>"` after the build step.
The key must be passwordless (`-W`, see §2.3) because nothing feeds it a password, and it
resolves in this order: `--sign-key` → `~/.minisign/<project>.key` →
a repo-local `.minisign.key` / `minisign.key` / `<project>.key` → `~/.minisign/minisign.key`.

Prefer a dedicated per-project key over the shared fallback. Note that rotating from a
shared key to a per-project key means previously published releases stay verifiable only
with the old public key; commit the new `minisign.pub` and say so in the commit message.

### 5.4 Published assets

The publish step attaches everything in `dist/` matching `*.tar.gz`, `*.zip`, `*.minisig`,
`SHA256SUMS`, or `<project>-*`. A scripted project therefore ends up with the same three
assets as a Go one: the tarball, `SHA256SUMS`, and `SHA256SUMS.minisig`.
