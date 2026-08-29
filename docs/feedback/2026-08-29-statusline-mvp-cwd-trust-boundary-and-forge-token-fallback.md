<!-- SPDX-FileCopyrightText: 2026 Uwe Jugel -->
<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# Session Retrospective: Status-Line CWD MVP, the Directory-Trust Gap, and a Forge-Token Doc Fix

*Date: 2026-08-29*
*Status: Recorded*

---

## 1. What happened, in order

1. **Incident.** In a separate `ubunatic.com` session, an agent ran `cd ~/projects/ubunatic.com && make -n ...`
   inside a shell shared with the human's own interactive terminal. The `cd` outlived the
   tool call; the user's next command (`make -C ubunatic.com`, typed expecting to still be
   in `~/projects`) failed with a confusing `ubunatic.com: No such file or directory` —
   because `-C ubunatic.com` was now resolving relative to `ubunatic.com/ubunatic.com`.
   Root cause only surfaced after the user asked "why do you always change my working
   directory?" — the error message itself gave no hint.
2. **Ticket filed** (`issues/095`): two proposed mitigations — (1) a documented convention
   that agentic workers restore the shell's original directory before yielding control, and
   (2) a `statusLine` showing `cwd` so drift is visible instead of silent.
3. **Part 2 implemented via a forked subagent**, scoped explicitly to an MVP: cwd only,
   nothing else. Delivered `internal/statusline/statusline.go`, `cmd/harnez/statusline.go`,
   and wiring into `internal/claude/apply.go`'s managed-settings mechanism, gated by a new
   `status_line` config key.
4. **User observed the shipped result live**: `harnez apply` produces a genuinely separate,
   permanent second line above the input box, pushing the built-in hotkey/shortcut footer
   down. The agent's own research (Claude Code's official statusline docs, checked live)
   had already found and documented this as a hard architectural constraint of the current
   tool, not an implementation shortfall — confirmed correct by the user's live observation.
5. **User raised a sharper, product-level point**: Claude Code's one-time "is it OK to work
   in this project?" prompt is presented as a trust boundary, but nothing re-checks or
   re-scopes directory access afterward — any approved Bash call can `cd` anywhere the OS
   user can reach. A documented convention (issue 095, part 1) is a mitigation an
   agent can choose to follow, not an enforcement mechanism; it doesn't close that gap.
   Ticket 095 was amended with this framing, and separate product feedback was drafted
   (queued locally via `SendFeedback`, not sent without the user's explicit approval) since
   actually closing the gap needs a Claude Code permission-model change, not anything
   `harnez apply` can deliver.
6. **`harnez release` run twice**, surfacing and then resolving a `[forge] Warning: ...
   status 401 ... token is expired` on the `has_releases` preflight check. Traced in code
   (`internal/release/forge.go`'s `GetForgeToken`) rather than assumed: with no
   `$CODEBERG_TOKEN`/`$FORGEJO_TOKEN` set, this check falls back to reading `fj`'s own
   credential store (`~/.local/share/forgejo-cli/keys.json`). Running `fj auth login`
   refreshed that file and the warning was gone on the next release — confirming the
   fallback path rather than guessing at it. `docs/practices/GoRelease.md` §3 was
   incomplete on this point (only mentioned env vars) and has been corrected.

## 2. Effectiveness & efficiency

- **Empirical verification over assumption, twice.** Both the statusline/footer
  same-line-impossible claim and the forge-token-fallback mechanism were confirmed by
  reading the actual source/docs and observing real runs, not inferred from memory or
  training-time knowledge of the tool. This mattered: guessing either would have produced
  a plausible-but-wrong evergreen doc entry (e.g. "the token comes only from env vars" was
  the doc's prior, incomplete claim).
- **Forking for the implementation task worked well.** The fork inherited the ticket,
  the `CLIDesign.md` apply/init separation constraint, and repo conventions without
  re-derivation, and self-scoped correctly (left part 1 of the ticket alone, held the
  MVP boundary, ran a review pass before committing per this repo's own workflow).
- **A design constraint surfacing after implementation, not before, cost a cycle.** The
  "second line is unavoidable" finding was already in the ticket and the fork's report
  before the user tried it live — but the user's reaction ("this becomes a permanent
  second line always") reads as a fresh objection rather than an acknowledged tradeoff.
  Restating a known, already-flagged constraint at the point the user actually
  encounters its consequence (not just once in a ticket body) would have saved a round
  of re-explanation.

## 3. Agentic coding insights

- **A convention (documentation) and an enforcement mechanism (runtime gating) are not
  substitutes**, and conflating them in a ticket's framing understates the residual risk.
  Issue 095 initially framed "agents restore cwd" as sufficient; the user's objection
  correctly identified that this only holds if the agent is well-behaved, and does
  nothing about the underlying trust-boundary gap in the tool itself.
- **Verify a claimed root cause against the actual code path before writing it into an
  evergreen doc.** `GoRelease.md` §3 stated a single-source-of-truth (env vars) for a
  token that, per `internal/release/forge.go`, actually has a documented fallback chain.
  The doc wasn't wrong so much as incomplete in a way that made a real, recurring
  situation (env vars unset, `fj`'s own store used) look unaccounted for.

## 4. What I'd do differently

- When a subagent's report already states an architectural constraint clearly, proactively
  re-surface it (not just leave it buried in a ticket) the moment the user's own
  observation is about to collide with it — rather than waiting for the user to notice and
  re-raise it as if new.
- Before writing a "how X works" doc paragraph that names a specific credential source or
  mechanism, grep the actual implementation first if it's available, rather than
  documenting the first mechanism found in existing docs as if it were complete.

## 5. Discovery gaps

- Did not independently verify whether Claude Code's statusline JSON schema or
  footer-sharing behavior has changed in versions between this session's live doc check
  and whenever this record is read later — that finding has a documented recheck trigger
  in issue 095 but is not itself durable across tool upgrades.
- Did not check whether other sibling `harnez`-managed projects (e.g. `lmcoder`, whose
  `origin` is a non-forge SSH mirror per the prior 2026-08-29 retrospective) hit the same
  `has_releases` token-fallback behavior — this session's fix was verified only against
  `harnez` itself.

## 6. Proposed harness improvements (not implemented — user's call)

- Filed as `issues/096`: `harnez release`'s `has_releases` 401 warning names the failure
  but not the fix — it could suggest `fj auth login` directly in the warning text when the
  underlying error indicates an expired/invalid token, instead of requiring a user to trace
  `GetForgeToken`'s fallback chain by hand (as this session did).
- Not filed as a ticket, left to the user to decide: whether `harnez apply`'s statusLine
  feature should stay MVP-scoped (cwd only) or grow richer now that the "always a second
  line" cost is confirmed and paid regardless — a richer status line no longer has an
  incremental screen-cost, since the second line already exists once `status_line: true`
  is set.
