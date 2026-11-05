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
   With `git.protected_main` in `.harnez/repo.yaml`, those commits go through a PR first, so
   `main` is level with its remote before you release. Wait for that PR with
   `harnez pr wait <number> --contains <sha>` (background); release only after exit 0.
2. **Push**: `git push`. (Skip under `protected_main`: `harnez release` pushes the branch and
   tag itself. If the forge rejects the push to `main`, stop and report it. See `docs/GoRelease.md`
   for the `release/vX.Y.Z` route.)
3. **Match the project's setup**: `harnez release` defaults to a patch bump, a minisign
   signature and a GoReleaser or `make dist` build. Check the project before the first run:
   - **Bump**: read the last tags (`git tag --sort=-v:refname | head -3`). Choose the step the
     project uses, for example `-b minor` when tags go v0.19.0 → v0.20.0. Ask the user when
     the history is unclear. A project with no `version.yaml` is versioned by its tags alone.
   - **Signing**: a project that ships no minisign-signed archives has no
     `~/.minisign/<project>.key`. Pass `--skip-sign` only when the project never signs;
     a missing key in a project that does sign is an auth stop (step 5).
   - **Build**: with no `.goreleaser.yaml` and no `dist` Make target, pass `--build-cmd`, or
     `--skip-build` for a tag-only release, and say that no binaries are attached.
   Then run `harnez release -n` with those flags and read its plan.
4. **Release**: run `harnez release` with the same flags.
5. **On failure**:
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
6. Report the resulting tag/version and confirm the push succeeded
   (`git ls-remote --tags origin <tag>`).
