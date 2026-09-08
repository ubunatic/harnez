# Cross-Repo Release Onboarding and the go.work Silent-Substitution Trap

**Date**: 2026-09-08
**Repos touched**: voxi, harnez, uman
**Tickets**: voxi 085, harnez 284, uman 024

## What happened

A single session chained four related pieces of work, escalating from a
routine onboarding task into a real correctness fix in harnez's release
engine:

1. `voxi`'s `make release` was a stub echoing manual instructions to run
   `uman release`. Investigation showed the workspace had already
   standardized on `harnez release` (harnez issue 091,
   `docs/practices/GoRelease.md`) — `spriteview` had migrated, but voxi
   and `webman` were still on the old pattern.
2. Onboarded voxi: `version.yaml`, a root-level `package voxi` (not
   `package main` — voxi's `main` lives in `cmd/voxi/`), `.goreleaser.yaml`,
   a dedicated minisign key, and the thin `Makefile` target. Verified with
   `--dry-run`, then did a real release: `harnez release` succeeded end to
   end except the final `fj` publish, because `fj` had just been installed
   and never authenticated. `harnez release --continue` after `fj auth
   login` finished cleanly with no re-bump/re-tag — the documented
   recovery path worked exactly as described on the first real use.
3. Deprecated `uman release` (the older, duplicate implementation) via
   cobra's native `Deprecated` field rather than removing it, since
   `webman` still depends on it and wasn't being migrated in this pass.
4. With voxi's `audiolevel` package now importable, dropped harnez's
   `replace ubunatic.com/voxi => ../voxi` in favor of a real pinned
   `require` — this worked because `ubunatic.com/voxi` already had a live
   go-import vanity redirect to `codeberg.org/ubunatic/voxi`, so the
   public module proxy and sumdb could resolve it like any other module.

## The trap: `go.work` and the release engine's own build step

Removing the `replace` directive reintroduced the friction it was
papering over: co-developing harnez and voxi together now meant either
re-adding `replace` before each dev session, or something better. `go.work`
is the right tool — untracked, lives outside either repo (in the shared
parent `~/projects/`), and is well-supported by `go build`/`test`/`gopls`.

But Go auto-detects `go.work` by walking up from the CWD to find one, with
no scoping to "only when I mean it interactively." That means it also
silently applies inside `harnez release`'s own build step — goreleaser and
`make` both just shell out to `go build`. Concretely, with
`~/projects/go.work` active (`use ./harnez ./voxi`):

```
$ go list -m ubunatic.com/voxi          # workspace active
ubunatic.com/voxi                        # no version — using local checkout
$ GOWORK=off go list -m ubunatic.com/voxi
ubunatic.com/voxi v0.1.1                 # pinned, tagged version
```

A release cut from inside that workspace would have silently built harnez
against whatever was sitting uncommitted in the local voxi checkout,
rather than the pinned `go.mod`/`go.sum` dependency the release claims to
be built against — a correctness gap in the one guarantee a release
pipeline exists to provide.

**Fix**: `harnez release`'s build subprocess (goreleaser/`make`/
`--build-cmd`) now runs with `GOWORK=off` forced by default
(`internal/release/runner.go`'s new `runBuildCmd`/`buildEnv`), with a
`--allow-workspace` flag to opt back in when that substitution is
genuinely intended. The `[build]` log line states which mode ran, so it's
visible in output rather than an invisible environment difference.

## What caught it, and what wouldn't have

This was found by literally creating the workspace and running
`go list -m` / `harnez release --dry-run --force` against it before and
after the fix — not by writing a unit test first and trusting it. A
plausible-looking `TestBuildEnvGOWORK` unit test was added afterward and
does correctly pin the `buildEnv()` contract, but it would not by itself
have caught the original bug, since the bug was entirely about which
environment `exec.Command` actually inherits at the OS-process level in a
real Go toolchain invocation — exactly the class of bug
`docs/AgenticLoop.md`'s "Live/Real-Environment Verification for hooks &
env-resolution features" checklist item calls out. Green tests are not
evidence a hook- or environment-resolution-dependent feature works; a
real, live invocation against the real environment is.

## Takeaways

- A vanity `go-import` redirect (`ubunatic.com/<project>` → Codeberg) is
  what makes "release it, then depend on it as a normal module, no
  `replace` needed" work at all — worth remembering when onboarding any
  new `ubunatic.com/*` module.
- `go.work` is the right answer for cross-repo co-development in this
  workspace, but any tool that shells out to `go build` from inside a
  workspace member directory (release engines, CI-adjacent scripts,
  `make` targets that build without an explicit `GOWORK=off`) needs to
  decide deliberately whether it wants workspace substitution or not —
  the default should almost always be "not," because the entire point of
  a release/build artifact is that it corresponds to pinned, tagged
  inputs.
- Deprecating a still-used tool (`uman release`) via the language's own
  idiom (cobra's `Deprecated` field) rather than deleting it or hand-rolling
  a warning kept `webman` working with zero coordination cost, while still
  making the migration path visible in `--help` output.
