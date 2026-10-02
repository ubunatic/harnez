# 685 — harnez compact --auto for the calling session and --dry-run stats

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: issues/635 (compaction engine), issues/646 (Claude compaction hook)

---

## 1. Problem & Motivation

`harnez compact` needs an explicit transcript path. Run from inside an agent session (e.g. Claude
Code's `! harnez compact`), the user does not know the path, and a bare `harnez compact` silently
prints nothing (reads empty stdin). The user expected `harnez compact --auto --dry-run` to work and
got `unknown flag: --auto`.

/goal Add `--auto` (compact the session `harnez compact` is called from) and `--dry-run` (stats only,
no writes), verified from a live Claude session, or stop and report when blocked on how to identify
the calling session for a provider or on a user decision about in-place writes to a live transcript.

## 2. Technical Specification / Findings

- `--auto`: find the calling session's transcript (Claude: `~/.claude/projects/<cwd-slug>/<id>.jsonl`;
  identify the session via env/parent process, or the newest transcript for the cwd as a fallback,
  stating which was chosen on stderr). Codex: follow-up. agy: out of scope (it has no `/compact`).
- `--dry-run`: no output file or in-place write; print stats (entries, bytes/tokens before and
  after, pruned/truncated counts). May reuse `--format summary`.
- Bare `harnez compact` on a TTY stdin without a transcript should error with a usage hint, not
  print nothing.
- Open: whether `--auto` without `--dry-run` may write in place while the host still appends to the
  transcript; decide with the user or with issue 646's hook findings.

## 3. Implementation & Verification Plan

- Tests for transcript discovery (temp `~/.claude/projects` tree) and for `--dry-run` writing nothing.
- Live check: `! harnez compact --auto --dry-run` in a Claude Code session prints stats for that session.
