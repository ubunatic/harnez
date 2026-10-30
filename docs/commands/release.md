# Lean CLI Release

Triggered by `/release`. Narrower than `/publish` (see `commands/publish.md`) — no
website sync, no liveness probe. Use for a routine CLI/binary version bump; use
`/publish` when website sync or a full multi-stage confirmation gate applies.

Reference Practice: `@docs/practices/GoRelease.md`

---

## Steps

1. **Commit pending code**: `git status`. Commit any uncommitted changes on the
   current branch. If stray `issues/` or `docs/` changes are mixed in, follow
   the same authorship/sanity-check bar as `/commit`.
   With `git.protected_main` in `.harnez/repo.yaml`: branch (`git switch -c release/vX.Y.Z`),
   then release, then open a PR and rebase-merge it. Never push `main`.
2. **Push**: `git push`. (Skip under `protected_main`: `harnez release` pushes the release
   branch and tag itself.)
3. **Release**: run `harnez release`.
4. **On failure**:
   - If it fails because a login/auth step is needed (e.g. forge token, minisign
     key), stop and ask the user to complete that step — do not attempt to work
     around auth interactively.
     Forge token errors suggest `fj auth login`; after login, resume with
     `harnez release --continue` so a complete signed `dist/` is reused.
   - Resume lists remote assets and prints `exists, skipped` for existing names.
     Local archives must match published `SHA256SUMS`; checksum conflicts require
     manual recovery. Never delete remote assets automatically or rebuild a valid
     signed set to work around a conflict. Missing local files trigger rebuilding.
   - For any other failure, diagnose and fix (failing test, dirty tree, version
     spec issue) and retry once. Escalate to the user if the second attempt
     also fails.
5. Report the resulting tag/version and confirm the push succeeded
   (`git ls-remote --tags origin <tag>`).
