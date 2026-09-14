# Podmac Extraction and Live Verification

The macOS Podman helper became the separate `../podmac` Go repository. Harnez
no longer contains or installs `scripts/macos-podman`; its macOS portability
docs remain here. The open shared-folder issue moved from harnez issue 345 to
podmac issue 001. Both projects passed Go checks and `make install`. The
running guest was left untouched because its disk and `/shared` mounts still
pointed into harnez.

The preceding favicon fix exposed a verification gap. The original noVNC
customization fetched `app/images/favicon.svg`, which noVNC did not serve.
The command built and the guest started, but the browser stayed on the blue
icon. Generating an SVG data URL removed that dependency. Executing the
injected script, checking two host colors, reading the served page, and
visually confirming the browser tab provided evidence that a Go build could
not. Podmac issue 004 tracks visible diagnostics for future patch failures.

The extraction exposed two path boundaries. Storage defaults to a path
relative to the current working directory, so moving the binary does not move
or identify the guest disk; podmac issue 002 tracks a safer default. A running
container retains the mounts it was created with, while `podmac status`
currently prints invocation paths; podmac issue 003 tracks actual mount
reporting. We preserved the live disk in harnez and made an ignored local
symlink in podmac instead of moving data under the running guest.

I made two avoidable housekeeping mistakes. A broad `.gitignore` pattern for
`macos-podman` also ignored the source directory, blocking a later `git add`;
anchoring the pattern fixed it. After removing the tracked source, I checked
`git status` but did not check the physical `scripts/` directory. Git does not
track empty directories, so the user had to point out the leftover directory.
The issue tracker and docs index were also discovered later than they should
have been during the cross-repository move.

Next time I would inspect the target repository and its docs and issue index
before moving files, check ignore rules against both source and binary paths,
verify the live container's actual mounts, and inspect the source directory
after removal in addition to checking Git. A useful proposed AGENTS.md
improvement is an extraction checklist that requires those physical-directory,
ignored-file, and runtime-state checks before declaring a repository move
complete. This is a proposal only; no harness rule was changed.
