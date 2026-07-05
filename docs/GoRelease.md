<!-- SPDX-FileCopyrightText: 2026 Uwe Jugel -->
<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# Go Release Pipeline Proposal

*Project: wayreel (generalises to any Go + goreleaser project)*
*Status: Proposed · 2026-07-04*

The release machinery for wayreel spans 615 lines across Makefile targets,
helper scripts, goreleaser config, and docs — and has required manual
intervention on every release so far. This document diagnoses root causes
and proposes a consolidation path.

**TL;DR**
- The release section is 48 % of the entire Makefile — yet all the hard
  work is already in goreleaser.
- goreleaser's `signs:` and native Gitea publisher eliminate the `fj`
  dependency and the separate `minisign -S` call entirely.
- Medium-term: `uman release` turns the remaining glue into a proper
  cross-project CLI.

---

## LoC Burden

| Component | Purpose | Lines |
|---|---|---|
| `Makefile` — release section | Orchestration targets | 127 |
| `scripts/bump_version/main.go` | Edits version.go | 73 |
| `scripts/release_diff/main.go` | Git diff since last tag | 56 |
| `scripts/check_untracked.sh` | Preflight guard | 11 |
| `.goreleaser.yaml` | Cross-platform build + signing config | 82 |
| **Executable subtotal** | | **349** |
| `docs/release_flow.md` | How-to + recovery paths | 177 |
| `issues/14-release-flow-safety.md` | Two rounds of bugs + fixes | 89 |
| **Documentation burden subtotal** | | **266** |
| **Total release overhead** | | **615** |

Key ratios:
- **48 %** of the entire Makefile is release machinery
- **3×** manual recovery operations on the first two releases
- **0** of goreleaser's publish + sign features currently exercised

---

## Where Releases Break

### 01 — Tag pushed before build succeeded ✅ Fixed (issue 14)
Original flow: `tag → push → build → sign`. Push happened before goreleaser,
so a build failure left a dangling remote tag. Fixed by reordering to
`tag → build → sign → push`.

### 02 — Make wildcard captured stale file list ✅ Fixed (2026-07-04)
`_dist_files = $(wildcard dist/*.tar.gz …)` was evaluated before goreleaser's
`--clean` ran, capturing old version filenames. `fj` tried to attach files
that no longer existed. Fixed by extracting `_fj-create` as a `$(MAKE)`
sub-invocation — a fresh make process evaluates the wildcard post-goreleaser.

**Root cause:** Make is the wrong tool for stateful multi-step orchestration.
Any global variable with `$(wildcard)` or `$(shell)` risks this class of bug.

### 03 — fj creates partial draft release on failure ⚠ Manual recovery
`fj release create` creates the Codeberg release record before attaching
files. On any attachment failure it leaves a draft without files. Re-running
fails with *Release has no Tag*. No `--overwrite` flag exists.

**Root cause:** `fj` is not designed for idempotent pipeline use. goreleaser's
native publisher handles rollback internally.

### 04 — Signing step lives outside goreleaser ⚠ Latent inefficiency
The `.goreleaser.yaml` `signs:` block already handles `MINISIGN_PASSWORD`
env, but `release-full` passes `--skip=sign` and calls `minisign -S`
separately. Two sign invocations, and goreleaser's tested passphrase-via-env
path is never exercised.

### 05 — _dist_files list diverges from goreleaser config ⚠ Maintenance debt
The Makefile's `_dist_files` wildcard patterns must manually track
goreleaser's archive `name_template` values. Two binary-archive patterns
were already stale. Every goreleaser config change risks a silent mismatch.

---

## Options

### Option A — Let goreleaser own signing and publishing *(recommended quick win)*

goreleaser has a Gitea/Forgejo publisher (`gitea:` in the `release:` block)
and a `signs:` section that reads `MINISIGN_PASSWORD` from env. Removing
`--skip=sign --skip=publish` and configuring the publisher eliminates:
- Separate `minisign -S` call
- The `fj` dependency
- The `_dist_files` wildcard
- The partial-draft failure mode
- The wildcard-timing bug class

The `release-full` recipe collapses from 10 lines to 4.

**Requires:** `GITEA_TOKEN` env configured; one-time `.goreleaser.yaml` update.

### Option B — Go release script within wayreel

Extract orchestration to `scripts/release/main.go`. Proper Go stdlib error
handling, testable, no Make weirdness. Still wayreel-only; reimplements what
goreleaser already does.

### Option C — Add `uman release` command *(recommended medium-term)*

`uman` already manages multi-project push and status. A `release` command —
`uman release <project> --bump patch` — wraps version bump, preflight,
goreleaser, and push for any goreleaser-configured Go project. Shares the
pattern with every new project. Requires Option A first.

---

## Migration: Option A

### 1 — .goreleaser.yaml additions

```yaml
release:
  gitea:
    owner: ubunatic
    name:  wayreel
  draft: true           # still lands as draft; publish manually
  skip_upload: false    # remove --skip=publish from make

# signs: block is already present and correct — just remove --skip=sign
```

### 2 — Environment setup (once, e.g. in .bashrc or keyring)

```bash
# Codeberg API token (read by goreleaser's Gitea publisher)
export GITEA_TOKEN="$(cat ~/.config/codeberg/token)"

# minisign passphrase — goreleaser's signs: block already reads this
# If using Keyring: set MINISIGN_PASSWORD from the keyring query
export MINISIGN_PASSWORD="$(secret-tool lookup service minisign)"
```

### 3 — Makefile: release-full after Option A

```makefile
release-full: unlock-minisign ⚙️  # tag, build, sign, publish
    @case "$(SKIP_TAG)" in (1|true|yes|y) ;; (*) $(MAKE) tag ;; esac
    goreleaser release --clean
    git push origin main --tags
    @echo "✅ draft release live — visit Codeberg to publish"
```

### 4 — Makefile: what to delete

```
# DELETE: _dist_files variable  (goreleaser manages its own artifacts)
# DELETE: _fj-create target
# DELETE: release-publish target  (goreleaser retry handles this)
# DELETE: separate minisign -S call in release-full
```

**Keep:** `unlock-minisign`, `bump-*` / `prepare-*`, `preflight`.

**Verify before first live run:**
- `GITEA_TOKEN` permission scope: needs *repository* write for releases
- Test with `--skip=publish` once to confirm signing works

---

## uman release — Sketch

Once Option A is in place, the full release flow is short enough to live
in `uman`. The version bump logic in `scripts/bump_version/main.go` (73 lines)
and `scripts/release_diff/main.go` (56 lines) become `internal/` packages.

```
uman release <project> [flags]

  --bump    patch|minor|major   bump version before releasing
  --dry-run                     print steps without executing
  --skip-tag                    recovery: skip local tag creation
```

```go
// uman/release.go — skeleton
var releaseCmd = &cobra.Command{
    Use:   "release <project>",
    Short: "Bump, build, sign, and publish a goreleaser project",
    RunE:  runRelease,
}

func runRelease(cmd *cobra.Command, args []string) error {
    // 1. resolve project path from uman config
    // 2. bump version.go if --bump set (reuse bump_version logic)
    // 3. verify signing key (equivalent to unlock-minisign)
    // 4. git commit -am "release: vX.Y.Z" + git tag
    // 5. goreleaser release --clean  (signs + publishes natively)
    // 6. git push origin main --tags
    // 7. print Codeberg release URL
    return nil
}
```

---

## Recommended Path

**Now:** Option A — configure goreleaser to sign and publish. One
`.goreleaser.yaml` block, one env variable, ~30 lines deleted from the
Makefile. Eliminates pitfalls 3, 4, and 5 immediately.

**When uman needs releasing:** Option C — `uman release`, built on top of
Option A. Absorbs the bump scripts as internal packages. The Makefile
`release-full` becomes a thin wrapper or disappears entirely.
