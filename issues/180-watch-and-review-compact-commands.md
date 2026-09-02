# 180 — Harnez Should Watch `/compact` Events and Assess Before/After Quality

**Status**: Closed (scoped down — see Research Findings and Resolution Note)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [issues/178](178-distill-smart-mode-error-pattern-preservation.md) (output distillation, adjacent context-loss concern)

---

## 1. Problem & Motivation

`/compact` (context summarization) is one of the biggest single points of information loss in a
long agent session — a bad compaction can silently drop the one detail (a file path, an exact
error string, a decision rationale) that made the rest of the session coherent. Today harnez has
no visibility into when compaction happens or whether it preserved what mattered. Agents and users
only discover a bad compact after the fact, when the agent starts asking questions it should
already know the answer to.

## 2. Technical Specification / Findings

Needs research before implementation — this is filed as a feature ticket with an open design, not
a spec-complete one. Open questions:

- What signal is available that a `/compact` happened? For Claude Code, `/compact` runs in the
  harness itself, not as a tool call harnez can hook (unlike `harnez rate` after tool calls). May
  need to look at session transcript files/hooks (e.g. `PreCompact`/`PostCompact`-style hook
  points if the host exposes them) rather than active polling.
- "Before" state: capture (or reference) the transcript content immediately prior to compaction.
  "After" state: the generated summary. A useful assessment needs both.
- What does "assessment" mean concretely? Candidate cheap heuristics: byte/line reduction ratio,
  whether specific proper nouns/file paths/identifiers present pre-compact still appear
  post-compact, whether open TODOs or pending-task markers survived.
- Where does the assessment surface? Likely a `harnez rate`-style log entry or a note surfaced via
  the "session state" mechanism proposed in issue 183, rather than a new blocking UI.

## 3. Implementation & Verification Plan

1. Research harness hook points (Claude Code hooks config, Codex equivalents) for pre/post-compact
   signals; document findings before writing code.
2. Prototype a lightweight diff/heuristic comparator (no LLM call required for v1) that flags
   likely information loss (e.g. an identifier mentioned N times pre-compact and 0 times
   post-compact).
3. Log the assessment through harnez's existing event/telemetry path; verify with a synthetic
   before/after pair.
4. If a host-native hook isn't available, document that constraint and downgrade scope to a
   manual/opt-in `harnez compact-check <before> <after>` command instead of automatic watching.

## 4. Research Findings

Checked Claude Code's own hook-development reference (`~/.claude/plugins/marketplaces/
claude-plugins-official/plugins/plugin-dev/skills/hook-development/SKILL.md`), which enumerates
every hook event the harness exposes: `PreToolUse`, `PostToolUse`, `Stop`, `SubagentStop`,
`SessionStart`, `SessionEnd`, `UserPromptSubmit`, `PreCompact`, `Notification`.

- **`PreCompact` exists** and fires before context compaction, with the stated purpose "Use to
  add critical information to preserve" — i.e. it's a hook point, and it fires while the
  pre-compact transcript is still the live context, so a "before" snapshot is technically
  capturable.
- **There is no `PostCompact` event.** The event list above is exhaustive per this reference —
  nothing fires after compaction completes. This means harnez cannot observe the actual generated
  summary (the "after" state) automatically; the harness gives no signal that compaction has
  finished or what it produced.

This confirms the ticket's own suspicion: full automatic before/after watching is **not
implementable** in the current harness, specifically because the "after" half of the comparison
has no hook to capture it, even though the "before" half technically does. Wiring only the
`PreCompact` half (snapshot the transcript, but never see what the compaction actually kept)
would not deliver the ticket's actual goal — assessing what was lost — so it was not worth
half-implementing.

**Decision**: downgrade to the ticket's documented fallback — a manual/opt-in
`harnez compact-check <before> <after>` command, per point 4 of the Implementation Plan above.
A user or agent who suspects a bad compaction (or who wants to check proactively) saves the
pre-compact transcript and the post-compact summary as two files and runs the comparator
manually. This also keeps the door open for a future host update that adds a `PostCompact` event
— the comparison logic (`internal/compactcheck`) is decoupled from how the two snapshots get
captured, so an automatic wiring on top of it would be additive, not a rewrite.

## 5. Resolution Note

Implemented the scoped-down deliverable:

- `internal/compactcheck/compactcheck.go`: a `Compare(before, after string) Report` heuristic
  (no LLM call). It extracts path-like/identifier-like tokens (regex: 6+ char runs containing
  `._-/`) from both texts, and flags any identifier mentioned 2+ times in `before` that appears
  zero times in `after` as a likely-dropped detail. Also reports a byte-count reduction ratio.
  `FormatReport` renders a short human-readable summary.
- `cmd/harnez/compactcheck.go`: `harnez compact-check <before-file> <after-file>` — reads two
  files, runs the comparator, prints the report. Registered in `cmd/harnez/main.go`'s root
  command list.
- `internal/compactcheck/compactcheck_test.go`: covers the core heuristic (a 3x-mentioned
  identifier that vanishes is flagged; a 1x-mentioned one that vanishes is not, since it's below
  the `minMentions` signal threshold; a surviving identifier isn't flagged; empty-before is
  handled without a divide-by-zero) and `FormatReport`'s output shape.

Design decisions:
- **minMentions = 2**: a single mention disappearing is too weak a signal at this heuristic's
  precision (could be a typo, a one-off aside); repeated mentions are a much stronger "this
  mattered" signal, keeping the false-positive rate low for a v1 that has no semantic
  understanding of the text.
- **No wiring to `PreCompact` in this ticket**: per the research finding above, `PreCompact`
  alone can't deliver useful before/after assessment without the missing after-hook, so
  auto-capturing "before" only (with no automatic "after") was judged not worth the added
  complexity for a v1 — a user/agent running `compact-check` manually already has both files in
  hand from having read the transcript.
- **Not wired into the session-state mechanism (issue 183)** in this ticket — 183 is a separate,
  independently-scoped ticket; `compactcheck.Report`'s shape (byte reduction + dropped-identifier
  list) is stable and small enough that 183 (or a future ticket) can log a `compact-check` result
  through it without changes here.

Verification: `go build ./...` and `go test ./...` pass (all packages including the new
`internal/compactcheck` tests). `make install` and `make lint` both run clean. Manually verified
`harnez compact-check` end-to-end against two synthetic snapshot files.

**Scope change flagged**: the original ticket framed this as "watch `/compact` events... assess
before/after quality" with automatic watching as the primary framing and the manual command only
as a fallback (Implementation Plan step 4, conditional on "if a host-native hook isn't
available"). That condition was met, so the delivered scope is the fallback: a manual, opt-in
`harnez compact-check` command — not automatic watching, not a `PreCompact` hook wired into
`config.yaml`'s `hooks:` list. No PreCompact hook was added to this project's own `apply` config
in this ticket.
