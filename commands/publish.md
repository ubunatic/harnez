# Multi-Project Release & Publish Workflow

Triggered by requests like `/publish`, "publish this project", "release the current project", or "publish the website and release".

Reference Practices: `@docs/practices/GoRelease.md`, `@docs/practices/DeploymentTransparency.md`, `@docs/Website.md`, `docs/CLIDesign.md`
Reference Tickets: `issues/246-add-commit-and-publish-skills-for-staged-commit-ownership-and-multi-project-release-publish-workflows.md`, `issues/091-language-agnostic-release-spec-and-thin-make-release.md`

---

## 1. Objective & Non-Negotiable Safety Rules

`/publish` orchestrates the multi-step release and publication pipeline across project types (CLI binaries, mobile/desktop app artifacts, project documentation websites, and hybrid releases).

### Non-Negotiable Safety Gates
1. **Mandatory Explicit Confirmation Gate**: Publishing is an externally visible, hard-to-reverse action (pushes git tags, uploads binaries to forges, deploys live website files to public webservers). You MUST present a clear execution plan and obtain the user's explicit confirmation before running any destructive or publishing commands (`git push`, `fj release create`, `make sync` in `ubunatic.com`).
2. **Pre-flight Cleanliness**: All changes must be cleanly committed (clean git working tree) and local test suites (`make check`, `make test`, `reuse lint`) must pass before initiating a release.
3. **Media & Visual Verification**: If any new screenshots, recordings (WebM/reels), or visual assets were produced or modified, verify that user confirmation was obtained before proceeding.
4. **3-State Grounding & Live Liveness Assertion**: Never declare publication complete until verifying the live state (probe HTTP 200 on live endpoints, verify git tags and release assets).

---

## 2. Project Shape Auto-Detection

Inspect the current project to determine which release/publish tracks apply:

| Project Shape | Detection Criteria | Primary Action |
|---|---|---|
| **CLI / Library Tool** | `version.yaml` present, `.goreleaser.yaml`, or `release` target in `Makefile` calling `harnez release` | `harnez release` (version bump, build, minisign, forge upload) |
| **App / Binary Artifact** | Custom `release-build` / `release-sign` / `dist/` targets in `Makefile` (e.g. Android APK, GUI bundle) | `make release-preflight && make release-build && make release-sign` |
| **Static Website / Docs** | `website/` directory present (or declared in `~/.projects/.uman.toml`) | `uman website sync <project>` + `ubunatic.com` live sync |
| **Hybrid Project** | Both artifact release (`version.yaml` / `make release`) and `website/` directory | Full pipeline: Artifact Release + Website Sync & Deploy |

---

## 3. 5-Stage Publishing Pipeline

### Stage 1: Pre-flight Verification & Cleanliness Check
1. **Check Working Tree**:
   - Run `git status --porcelain`.
   - If uncommitted changes or unstaged files exist, ask the user or ensure changes are committed first (see `docs/Git.md`).
2. **Run Quality Checks**:
   - Execute project validation targets: `make check`, `make test`, or `go test ./...`.
   - Ensure all linters, formatting, and tests pass cleanly without errors.
3. **Dry-Run Artifact Build**:
   - For CLI tools: run `harnez release --dry-run` or `make build`.
   - For App artifacts: verify prerequisite toolchains (Minisign key, Android SDK/keystore, GoReleaser).
   - For Websites: if the project has a spec-driven website generator (e.g. `make website`, `make website-privacy`), execute it to ensure HTML/assets are up-to-date.

### Stage 2: Media & Artifact Verification Gate
1. Inspect generated artifacts in `dist/` (checksums, file sizes, binary presence).
2. If new screenshots, UI mockups, or demo recordings exist under `website/`, ensure explicit user confirmation was granted before public syndication.

### Stage 3: User Confirmation Gate (Mandatory Pause)
Stop and present the summary to the user:
- Project name & detected shape.
- Target release version / tag (e.g. `v0.4.2`).
- Generated artifacts to be signed and uploaded.
- Website subpage destination (e.g. `https://ubunatic.com/<project>/`).
- Upstream forge repository (e.g. `codeberg.org/ubunatic/<project>`).

**Ask for explicit user approval before proceeding to Stage 4.**

### Stage 4: Live Execution
Upon receiving explicit user confirmation:

1. **Artifact Release (if applicable)**:
   - Run `harnez release` (or `make release`).
   - Verifies `dist/SHA256SUMS.minisig` signing.
   - Pushes release tag and binary assets to the forge (e.g. Codeberg).

2. **Website Synchronization (if `website/` present)**:
   - Sync project website into the hosting repository:
     ```bash
     uman website sync <project>
     ```
   - Change to `~/projects/ubunatic.com`:
     ```bash
     # Verify links, license headers, and gopkg tags
     make check
     # Commit the synced website assets
     git add -A && git commit -m "feat(website): sync <project> documentation and release updates"
     # Deploy live to web hosting
     make sync
     # Push hosting repository changes
     git push
     ```

3. **Project Git Push**:
   - In the project directory, push commits and tags:
     ```bash
     git push --follow-tags
     ```

### Stage 5: Post-Publish 3-State Grounding & Liveness Probe
Adhere to `@docs/practices/DeploymentTransparency.md` and `@docs/practices/GoRelease.md` §6:

1. **Forge Release Verification**:
   - Probe git remote: `git ls-remote --tags origin <tag>`.
   - Verify forge release page status if accessible via CLI.
2. **Live Website Liveness Probe**:
   - For website projects, test the live HTTP response code directly:
     ```bash
     curl -s -o /dev/null -w "%{http_code}" "https://ubunatic.com/<project>/"
     ```
   - Assert HTTP `200 OK`.
3. **Report Outcome**:
   - Report release tag, live URL, artifact checksums, and deployment status cleanly to the user.
