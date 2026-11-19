# 1228 — issues new on a forge tracker fails when fj is a wrapper that already sets the host

**Status**: Closed — Host-qualified fj issue create/comment references and login/manual-run diagnostics delivered in 00c70238 and a262a1dd (PR #173); verified live on codeberg.org/ubunatic/harnez#174 and codeberg.org/harnez/taia#292
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1226

---

## 1. Problem & Motivation

`harnez issues new "<title>" -F <body-file>` in a repository whose tickets live on the forge
(`issues.tracker: none`) failed on the owner's machine and filed nothing:

```
issues new: filing on codeberg.org/harnez/taia with fj
Error: issues new: fj failed: exit status 2
error: the argument '--host <HOST>' cannot be used multiple times
```

The agent rule in `.harnez/rules/Index.md` names this command as the way to file, so an agent
that follows the rule is stuck. Seen 2026-11-13 while filing taia #284 and #285; both were then
filed with `fj issue create --repo harnez/taia --body-file <file> "<title>"`.

## 2. Technical Specification / Findings

- `internal/forgetracker/create.go` builds
  `fj --host <host> issue create --repo <slug> --body-file <file> <title>`.
- On this machine `~/.local/bin/fj` is a shell wrapper: `exec ~/.cargo/bin/fj -H codeberg.org "$@"`.
  fj 0.7.0 rejects a second host flag.
- `harnez find issues` and `harnez issues show` worked against the same forge in the same
  session, so only the create path is affected (not checked: `comment`).
- Unverified: whether fj accepts the host inside `--repo`, or reads it from an environment
  variable, which would avoid the flag.

## 3. Implementation & Verification Plan

- Canary first: find one fj invocation that works both with the plain binary and with a wrapper
  that sets `-H`. Candidates: resolve the real binary past a wrapper, pass the host another way,
  or retry without `--host` when fj reports the duplicate.
- Add a `forgetracker` test for the chosen command form.
- If no form works for both, make the error say what to do: name the wrapper and print the
  `fj issue create` line to run by hand.

## Sprint 2026-11-16 (lean sprint 1228, 1212, 1202)

Searcher findings (report: `docs/studies/2026-11-16-two-agent-lean-sprint-forge-filing.md`):

- `comment` has the same defect: `cmd/harnez/issues_forge.go:161` builds its own
  `fj --host <host> issue comment <owner/repo>#N --body-file <file>`. Create and comment share no
  builder (`internal/forgetracker/create.go:67-72`).
- Read-only canary with fj v0.7.0 on origin PR 78: the plain binary (`~/.cargo/bin/fj`) fails on
  `owner/repo#N` without `-H` ("cannot find repo, no host specified"); the host-qualified reference
  `codeberg.org/owner/repo#N` works with both the plain binary and the `-H` wrapper. Verified for
  `issue view` only.
- fj has no host environment variable or config setting in its help texts.

## Decision

- Use the host-qualified repository reference (`<host>/<owner>/<repo>`, and `<host>/<owner>/<repo>#N`
  for an issue) and drop `--host` from every fj command line. Reason: it is the only form the canary
  showed working with both a plain fj and a wrapper that sets the host. (Host, 2026-11-16.)
- Create and comment get one shared fj command builder, so the host form cannot diverge again.

## M1 — host-qualified fj references for create and comment

1. Canary first, read-only: confirm that `--repo <host>/<owner>/<repo>` is accepted where
   `fj issue create` takes `--repo`, using a read-only fj sub-command that takes the same `--repo`
   argument (for example `fj issue search`), with both `~/.cargo/bin/fj` and `~/.local/bin/fj`.
   No command that creates, edits or comments. If the form is not accepted, stop and report.
2. One builder in `internal/forgetracker` for fj issue commands; `PlanCreate` and the comment path
   both use it. No `--host`/`-H` in any fj argv.
3. Tests: exact argv for fj create and fj comment (host-qualified, no host flag); a regression test
   that fails if a host flag reappears. The tea and gh forms stay unchanged and stay covered.
4. When fj still fails, the error names the tool and prints the command line to run by hand, with
   the body file path, never the body text.

Acceptance: the tests above pass; the installed binary's dry form of the command (or the test
argv) shows the host-qualified reference. The live create on a real forge is run by the host
after owner approval; do not close this ticket.

## M1 delivered: host-qualified fj references (`00c70238`)

`forgetracker.FJIssueCommand` builds both fj argv forms without a host flag; canary
`fj issue search --repo codeberg.org/ubunatic/harnez` exited 0 with the plain binary and the
wrapper. Host check with the installed binary: `harnez issues comment -d . 41 -F <file> --forge
--dry-run` prints `dry run: fj issue comment codeberg.org/ubunatic/harnez#41 --body-file "<file>"`.
Open: the live create on a real forge (host, after the API write path lands).

### Decision (review of M1, host and reviewer `forge-rev1`, 2026-11-16)

- Auth failures print the login fix only, not the "run manually" line: running the same fj command
  by hand fails the same way until the login is repaired.
- fj's own output stays in the error text unredacted: it is the only diagnosis for an fj failure,
  and harnez passes fj a body file path, never body text or a token. Accepted as a low risk.

### Pre-Work / Required Refinements (do these first in the next developer turn)

1. Split `FJIssueCommand` into two constructors (create, comment) that share one unexported
   helper for the `<host>/<owner>/<repo>` reference. No sentinel number, no ignored argument.
2. One helper for the fj login hint, used by every fj auth branch in `cmd/harnez/issues_forge.go`
   (the 401/"token is required" branch, the token-reuse branch, and the hint near line 155).
   Wording: `fj login for <host> failed; run "fj auth login" if your fj wrapper sets the host,
   otherwise "fj -H <host> auth login"`. One line, no trace.
3. Tests: a failed `issues new` with fj and a failed `issues comment` (ordinary error) each assert
   the exact `run manually: fj issue ...` line with the host-qualified reference and the body file
   path, and that the body file's text does not appear in the error.
