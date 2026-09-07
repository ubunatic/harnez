# Rebase-safe issue repair review loop

- The real two-history fixture paid for itself: it verified deterministic 270/271 →
  277/278 allocation and exposed Git's staged-state behavior around autostash and
  `commit --only`.
- Independent review materially changed the design from an inert recovery file and
  mutating lint into a resumable phase machine and cached-index validation.
- Crash safety needs atomic publication at every layer. A no-overwrite final link is
  insufficient when its input temp can be truncated; write/close a unique scratch
  inode before publishing the deterministic transaction artifact.
- Live installation caught legacy unnumbered archived H1s that synthetic fixtures did
  not represent. Repository canaries remain necessary for hook and validation work.
