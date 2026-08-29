<!-- SPDX-FileCopyrightText: 2026 Uwe Jugel -->
<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# Harnez Go Release Pipeline & Non-Interactive Signing Case Study

*Project: harnez*
*Status: Established · 2026-08-29*

This document provides a postmortem and architectural narrative of setting up the automated release pipeline for `harnez` (a Go CLI repository) using `uman release`, `goreleaser`, `minisign`, and `fj`.

---

## 1. Background & Release Architecture

In the workspace ecosystem, repository orchestration and release flows are unified through the `uman` CLI (`uman release <project>`). The standard release lifecycle consists of:
1. **Preflight**: Verify required binaries (`git`, `goreleaser`, `minisign`, `fj`).
2. **Key Unlock Verification**: Validate that a minisign secret key exists and can sign a payload.
3. **Version Determination & Bump**: Read current version from `version.go` (or tags), increment semver, update `version.go`, commit, and create a lightweight annotated tag `vX.Y.Z`.
4. **Binary Compilation & Packaging**: Run `goreleaser release --clean --skip=publish --skip=sign` to cross-compile for `linux/amd64` and `linux/arm64` and create `.tar.gz` archives and `SHA256SUMS`.
5. **Cryptographic Signing**: Sign `dist/SHA256SUMS` using `minisign -S` producing `dist/SHA256SUMS.minisig`.
6. **Remote Push**: Push the branch and the new `vX.Y.Z` git tag to the remote origin (`codeberg.org/ubunatic/<project>`).
7. **Forge Publishing**: Use `fj release create` to draft the release and attach all dist archives, checksums, and signature files to Codeberg/Forgejo.

---

## 2. Obstacles Encountered & Solutions

During the initial release setup for `harnez` v0.1.0, several concrete hurdles were diagnosed and resolved:

### Obstacle 1: Interactive Password Prompts in Automated / Agentic Releases
- **Problem**: Default `minisign` keys created via `minisign -G` prompt interactively for a passphrase on stdin/tty. When delegated to an autonomous background agent or automated CI script without tty interaction, the prompt hangs or aborts.
- **Root Cause**: `minisign` encrypts the secret key with a password by default unless explicitly instructed otherwise.
- **Solution**: Generate a dedicated, unencrypted secret key using the `-W` flag:
  ```bash
  minisign -G -W -f -p ~/.minisign/harnez.pub -s ~/.minisign/harnez.key
  ```
  And declare this project-specific key in `.uman.toml`:
  ```toml
  [release.projects.harnez]
  minisign_key = "~/.minisign/harnez.key"
  ```
  This guarantees deterministic, zero-prompt cryptographic signing.

---

### Obstacle 2: Codeberg/Forgejo Repository API Returns `404 Not Found` on `fj release`
- **Problem**: Running `fj release create` (or `fj release list`) failed with:
  ```text
  Error: not found: The target couldn't be found.
  Location: src/release.rs:241:19
  Error: fj failed: exit status 1
  ```
  even though the git repository existed and `git push` succeeded.
- **Root Cause**: On Forgejo/Codeberg, newly created repositories may have the **Releases** unit (`has_releases`) disabled by default in repository settings. When `has_releases: false`, the Forgejo API returns a `404 Not Found` for any release endpoint (`/api/v1/repos/{owner}/{repo}/releases`), which `fj` reports as a missing target.
- **Solution**: Enable the releases unit on the repository via the Codeberg REST API:
  ```bash
  curl -X PATCH -H "Authorization: token $CODEBERG_TOKEN" \
       -H "Content-Type: application/json" \
       -d '{"has_releases": true}' \
       https://codeberg.org/api/v1/repos/ubunatic/harnez
  ```
  Once enabled, `fj release create` immediately succeeded.

---

### Obstacle 3: Safe Idempotency & Resuming Release Flows
- **Problem**: When the release failed at Step 7 (`fj release create`), the local tag `v0.1.0` was already created and pushed to the remote. Running `uman release harnez --bump 0.1.0` would attempt to recreate or re-bump the version.
- **Solution**: Utilize the `--continue` flag in `uman release`:
  ```bash
  uman release harnez --continue
  ```
  `--continue` inspects existing local tags, skips re-bumping and re-tagging, runs GoReleaser with `--skip=validate`, re-signs artifacts, and safely completes the publish step.

---

### Obstacle 4: Cobra Version Wiring
- **Problem**: Setting `var Version = "0.1.0"` in root `version.go` is insufficient unless wired into the CLI entry point.
- **Solution**: In `cmd/harnez/main.go`, bind the root Cobra command's version:
  ```go
  root := &cobra.Command{
      Use:     "harnez",
      Version: harnez.Version,
      Short:   "...",
  }
  ```
  This automatically equips the CLI with `--version` / `-v` reporting `harnez version 0.1.0`.

---

## 3. Reference Configuration

### `.goreleaser.yaml`
```yaml
version: 2

project_name: harnez

builds:
  - id: harnez-linux
    main: ./cmd/harnez
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    binary: harnez
    ldflags:
      - -s -w -X ubunatic.com/harnez.Version={{.Version}}

archives:
  - id: default
    formats:
      - tar.gz
    name_template: "harnez-{{ .Version }}-{{ if eq .Arch \"amd64\" }}x86_64{{ else if eq .Arch \"arm64\" }}aarch64{{ else }}{{ .Arch }}{{ end }}-linux"
    files:
      - README.md
      - config.yaml

  - id: binary
    formats:
      - binary
    name_template: "harnez-{{ if eq .Arch \"amd64\" }}x86_64{{ else if eq .Arch \"arm64\" }}aarch64{{ else }}{{ .Arch }}{{ end }}-linux"

checksum:
  name_template: SHA256SUMS
  algorithm: sha256

signs:
  - cmd: sh
    args:
      - -c
      - >-
        if [ -n "${MINISIGN_PASSWORD}" ]; then
          echo "${MINISIGN_PASSWORD}" | minisign -S -s "${MINISIGN_KEY_FILE}" -m "${artifact}" -t "harnez {{ .Version }}";
        else
          minisign -S -s "${MINISIGN_KEY_FILE}" -m "${artifact}" -t "harnez {{ .Version }}";
        fi
    artifacts: checksum
    signature: "${artifact}.minisig"
    env:
      - MINISIGN_KEY_FILE
      - MINISIGN_PASSWORD
```

---

## 4. Key Takeaways for Sibling Projects

1. **Always verify `has_releases` on Codeberg/Forgejo**: If `fj` reports target not found, query `GET /api/v1/repos/{owner}/{repo}` to confirm `"has_releases": true`.
2. **Passwordless Keys for Agentic Autonomy**: Create `~/.minisign/<project>.key` with `minisign -G -W` to avoid interactive blocking during background agent handoffs.
3. **Use `--continue` on Partial Failures**: Never manually force-push or delete tags when an upload fails; use idempotent recovery flags.
