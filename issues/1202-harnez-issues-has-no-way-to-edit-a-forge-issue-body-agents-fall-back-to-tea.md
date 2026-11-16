# 1202 — harnez issues has no way to edit a forge issue body; agents fall back to tea

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1196, 1197, 1201, 1067

---

/goal Agents can reliably update issue bodies and titles through `harnez issues` in local and forge-tracker checkouts, without losing concurrent edits; or stop and report when a user decision, forge capability, or denied permission blocks progress. Recheck live code and recent history first. On a loop, hard blocker or no progress, clear the goal and record the failure and resume point.

## 1. Problem & Motivation
Feedback from another session (2026-11-09): "I updated the issue with tea because harnez issues can't edit an issue body."
On a forge tracker, `harnez issues show` is read-only and no verb edits a body. Agents bypass harnez, so the edit skips its token handling and telemetry.

In Taia, `harnez issues show 161` resolved Codeberg from `origin`, but `fj issue edit 161 body` could not infer the repository. Retrying with explicit host and `harnez/taia#161` worked. Passing the full body as an argument also made failure output noisy. No issue-creation failure was observed; forge creation already supports `-F` (#1195).

## 2. Findings
- `harnez issues --help`: `show` on a forge origin is "read-only"; no `edit` verb exists.
- Forgejo/Gitea: `PATCH /repos/{owner}/{repo}/issues/{index}` takes `title` and `body` (unverified in harnez; canary first, docs/Canary.md).

## 3. Plan
- Add `harnez issues edit <n> [--title T] [--body-file F|-]` for forge and local trackers, with exact multiline preservation and repository discovery consistent with `show`. Define local file and index behavior without discarding unrelated edits. Return the issue URL or local path and actionable errors without leaking tokens or the full issue body.
- Prevent lost updates using a revision or expected-body check from the prior read. Refuse conflicting edits; verify whether the forge supports an atomic conditional update and state any limits rather than claiming an atomic guarantee from a non-atomic preflight. Check issue state before editing and report closed or missing issues clearly.
- Keep the show/create/edit/close experience consistent across tracker types. Verify current forge create `-F` behavior and output rather than assuming it is missing; address any gaps needed for a single-call workflow.
- Test local and fake-forge paths, including repository resolution, stdin/file bodies, exact payloads, conflicts, closed issues, concurrent updates where supported, and secret-safe errors. Canary the real forge mechanism first.

## Sprint 2026-11-16 (lean sprint 1228, 1212, 1202)

Searcher findings (report: `docs/studies/2026-11-16-two-agent-lean-sprint-forge-filing.md`):

- On a forge tracker `new`, `show` and `comment` exist; `close`, `open` and the other status
  verbs are local only (`cmd/harnez/issues.go:450`, `:486`); there is no `edit` and no `reopen`.
- `show` already uses harnez's own API client (`internal/forgetracker/forgetracker.go:88`,
  `internal/forge/issue.go:25`). Token lookup: stored Forgejo token, then
  `CODEBERG_TOKEN`/`FORGEJO_TOKEN`, then `gh auth token` (`internal/forge/api.go:43`). The stored
  token is read-only by design (`harnez forge token`, scopes in `spec/forge_token.yaml`).
- No harnez code sends an issue or pull-request write to a forge API today.
- fj v0.7.0 has `issue edit <n> title|body` with positional text only, `issue close`, and no
  reopen.
- Codeberg swagger: `PATCH /repos/{owner}/{repo}/issues/{index}` takes `title`, `body`, `state`
  and `updated_at`; `POST .../issues` takes `title` (required) and `body`;
  `POST .../issues/{index}/comments` takes `body`. Accepted `state` values and any conditional
  meaning of `updated_at` are unverified.

## Decision

Owner decisions, 2026-11-16 (asked by the host in the sprint session):

- Forge issue writes (create, edit, comment, close, reopen) go through harnez's own Forgejo API
  client with a write-capable stored token. `harnez forge token` gets an issue-write scope; the
  owner creates or rotates that token once. fj stays as the fallback for create and comment when
  no write token is stored.
- Forge writes are allowed only in a host session, never in a subagent.
- The live check runs on both `ubunatic/harnez` and `harnez/taia` on Codeberg, because the two
  repositories use different trackers (`harnez` and `none`). One scratch issue each, closed at
  the end.

Milestones for this ticket follow from the architect's design (next section, to be added).

## Design (architect `forge-arch`, claude:opus:low, 2026-11-16)

```
cmd/harnez/issues.go            dispatch: local or forge, flag checks
cmd/harnez/issues_forge.go      runIssues{New,Comment,Edit,Close,Reopen}Forge + requireHostSession
internal/forgetracker/write.go  plan: API when a write token is stored, else fj/tea (new, comment)
internal/forge/issue_write.go   Writer{Client,Token}: CreateIssue, EditIssue, Comment, LabelIDs
internal/forgetoken             same Store type; a second instance holds the write token
spec/forge_token.yaml + schema  new `write:` block (scopes, name_prefix, keyring_service, token_dir)
```

- Local or forge, one rule for every write verb: the forge is used when `resolveForgeTracker`
  succeeds (no `issues/` dir, `cmd/harnez/issues_forge.go:61-83`); `--forge` forces the forge on a
  repo with a local tracker, as `comment` does today (`cmd/harnez/issues.go:410-419`).
- Writes use only the stored write token. The read path (`TokenSource.For`,
  `internal/forge/api.go:57-74`) is unchanged; writes never fall back to env variables or gh.
- Without a write token, `new` and `comment` keep the fj/tea path; `edit`, `close` and `reopen`
  stop with `no write token for <host>; run: harnez forge token create --write`.
- Every forge write verb, the fj fallback included, calls `requireHostSession()` before any
  network or CLI call. `--dry-run` is exempt.
- Forgejo only. A `kind: github` forge gets `forge writes support Forgejo only; GitHub is ticket
  1209` with zero requests.

Verb surface (forge tracker; add `--forge` on a repo with a local tracker):

```
harnez issues new "Title" -F body.md [--label L] [--json]
harnez issues comment 42 -F note.md [--closed]
harnez issues edit 42 --title "New title"
harnez issues edit 42 -F body.md --if-updated <updated_at from issues show --json>   # or --force
harnez issues close 42 ["reason words"]
harnez issues reopen 42
```

- Local tracker without `--forge`: `edit` answers `issues edit: local tickets are files; edit
  issues/<n>-*.md and commit`; `reopen` answers `issues reopen: local tracker; use harnez issues
  open <n>`; `close` and `open` keep their local behaviour.
- Forge tracker: `open 42` answers `issues open: on a forge use reopen`.
- `--if-updated` and `--title` are valid only with `edit`.

Token: Forgejo defines `read:issue`, `write:issue`, `read:repository`, `write:repository`, and
write implies read (Forgejo source `models/auth/access_token_scope.go`, read from the `forgejo`
branch head). Codeberg reports version `17.0.0-100-2254cb0+gitea-1.22.0`
(`GET https://codeberg.org/api/v1/version`, 2026-11-16); that this version has the same scope
names is unverified until the owner creates the token.

Host-only check: `harnez agent` sets `HARNEZ_AGENT_ROLE` and `HARNEZ_SESSION_ID` for workers
(`cmd/harnez/agent_run.go:1591-1594`, `:1633-1635`); existing host detection is
`callerAgentRole()` (`cmd/harnez/agent_caller.go:66-71`) and `canManageTarget`
(`cmd/harnez/agent_args.go:52-57`). Limits, to be stated in the docs: a native subagent of the
host (it inherits the host environment), a worker that unsets the variables, and a worker that
calls curl, fj or tea itself are not stopped. It guards against accidents; it is not a security
boundary.

Lost updates: Forgejo's `updated_at` field on PATCH overrides the timestamp; it is not a
conditional update, and no ETag handling was found (Forgejo source `routers/api/v1/repo/issue.go`,
unverified for the deployed version). So the edit check is a preflight, not atomic.

### Decision (host, 2026-11-16)

- Two tokens, not one widened token: the read token and its users stay untouched, and the write
  token gets its own name prefix, keyring service and directory. Reason: least privilege for
  `pr wait` and `show`, and no migration of the stored token.
- `edit` reuses `--force` (already "skip the safety check" for `new`) instead of a new
  `--overwrite` flag; the help text names both meanings.
- `close 42 "reason"` on a forge posts the reason as a comment, then closes; if the comment fails,
  the issue stays open. Reason: same command shape as the local `close N "<reason>"`. This replaces
  the architect's "refuse reason words".
- `new --label` over the API: label names are resolved to ids with one GET; an unknown name is an
  error that lists the existing names. Reason: fj cannot set labels and tea is not installed.
- The sprint stays a lean sprint (the owner started it as one), although the milestones are now
  fully specified.

### Unverified until the live check

1. A `read:repository` + `write:issue` token can create, comment, edit, close and reopen, and can
   read labels, on both repositories.
2. `state` accepts `open` and `closed`.
3. PATCH returns the updated issue with a new `updated_at`.
4. The token-create API accepts `write:issue`.

## Milestones (M1 and M2 are in tickets 1228 and 1212)

General rules for every milestone: tests use `httptest` and the fake fj only, never a real forge;
no token and no issue body in any error or log line; no Go default for scopes, prefixes or
services (they come from `spec/`); do not change `Lookup`, `TokenSource.For` or `GetJSON`
behaviour; no GitHub write path; do not run `secret-tool search` (it prints secrets); do not close
1202, 1212 or 1228.

### M3 — write-token spec and CLI

Pre-work first: the "Pre-Work / Required Refinements" lists in tickets 1228 and 1212.

- Write set: `spec/forge_token.yaml`, `spec/schemas/forge_token.schema.json`,
  `internal/forgetoken/{spec,ops}.go` and tests, `cmd/harnez/forge_token.go` and test,
  `cmd/harnez/testdata/help.golden`, `docs/Spec.md` if it lists the spec fields.
- Spec: `write: {scopes: [read:repository, write:issue], name_prefix: harnez-write,
  keyring_service: harnez-forge-token-write, token_dir: forge-tokens-write}`. Schema: `write` is
  required, its scopes match `^(read|write):[a-z]+$`; the top-level read `scopes` keep `^read:`.
- CLI: `harnez forge token create|rotate|revoke|status|import --write` selects a second `Store`
  built from `spec.Write`; new `forgetoken.LookupWrite(host)`.
- Tests to lock:
  - `LoadSpec().Write.Scopes` equals `[read:repository write:issue]`.
  - A spec with `scopes: [write:issue]` at the top level fails schema validation.
  - `create --write` calls the client with a name starting `harnez-write-` and scopes
    `[read:repository write:issue]`, and stores under `forge-tokens-write`.
  - `status` with only a read token prints `write token: none (harnez forge token create --write)`.
  - `Lookup` returns the read secret and `LookupWrite` the write secret when both are stored.
- Edge cases: `revoke --write` leaves the read token; `import --write` without a secret fails; no
  secret appears in any output.
- Acceptance: tests pass; existing read-path tests are unchanged.

### M4 — API writer in `internal/forge`

- Write set: `internal/forge/issue_write.go`, `internal/forge/issue_write_test.go`,
  `internal/forge/issue.go` (`RemoteIssue.UpdatedAt`, json `updated_at`).
- `Writer{Client, Token}` with `CreateIssue(fg, owner, repo, title, body, labelIDs)`,
  `EditIssue(fg, owner, repo, n, EditFields{Title, Body, State *string})`,
  `Comment(fg, owner, repo, n, body)` and `LabelIDs(fg, owner, repo, names)`. No anonymous retry
  on 403 (that is only right for reads, `internal/forge/api.go:90-101`). Accept 200 and 201.
- Tests to lock (httptest):
  - `CreateIssue` sends `POST /api/v1/repos/o/r/issues` with body `{"title":"T","body":"B\n\nx"}`
    and header `Authorization: token tok`; with label ids the body also has `"labels":[1,2]`.
  - `EditIssue` with only a body sends `PATCH .../issues/7` with `{"body":"B"}` and no `title` key.
  - `EditIssue` with state `closed` sends `{"state":"closed"}`.
  - `Comment` sends `POST .../issues/7/comments` with `{"body":"c"}`.
  - `LabelIDs(["bug","nope"])` against labels `bug`, `P2` fails and names `nope` and the existing
    labels.
  - On 403 the error contains `HTTP 403`, neither the token nor the body, and exactly one request
    was made.
  - A `github` forge returns the 1209 error with zero requests.
- Edge cases: 404, 412 (detail passed through), 500, a reply that is not JSON.
- Acceptance: tests pass; no forge write code exists outside this file and `internal/forgetoken`.

### M3 and M4 delivered (2026-11-16)

- Pre-work `a262a1dd`: `FJCreateCommand` and `FJCommentCommand` share `fjRepo`; one `fjLoginHint`
  for every fj auth branch; exact-message tests for the "run manually" line.
- M3 (write-token spec and CLI) `97c0ec9c`: `write:` block in `spec/forge_token.yaml` and schema,
  `Spec.ForWrite`, `forgetoken.LookupWrite`, `--write` on `harnez forge token`. Installed binary:
  `harnez forge token status` ends with `write token: none (harnez forge token create --write)`.
- M4 (API writer) `b77e1634`: `forge.Writer` with `CreateIssue`, `EditIssue`, `Comment`,
  `LabelIDs`; `RemoteIssue.UpdatedAt`.
- Developer's `make test-q1` exited 0, last result line `ok ubunatic.com/harnez/scripts/gotest-summary`.
  Host rerun: `ok` for `internal/forgetoken`, `internal/forge`, `internal/forgetracker`, `cmd/harnez`.

### Decision (review of M3 and M4, host and reviewer `forge-rev2`, 2026-11-16)

- The forge's own `message` is shown for every failed write, not only for 412, as one line capped
  at 200 characters. Reason: a missing token scope or a 404 must be diagnosable from the error.
  The message is dropped when it contains the token. A message that echoes the body in an escaped
  form is accepted as a low risk: the body is what the caller just sent.
- The write token may be used for the reads a write needs (label list, the issue GET before an
  edit or close). It is never used by `TokenSource.For`.

### M5 — host-only check; `new` and `comment` over the API with fj fallback

- Pre-Work / Required Refinements (do these first):
  1. `forge.Writer` errors: keep the cause of a transport error (`%w`; a `url.Error` holds the
     method and URL, no header). Show the forge `message` for every non-2xx status (see the
     Decision above). Tests assert the exact status text (`HTTP 403`, not `HTTP `), a 403 whose
     message is `token does not have at least one of required scope(s): [write:issue]` shown in
     the error, and a message containing the token dropped.
  2. `Writer.Comment` returns the new comment's `html_url`.
  3. `Writer.LabelIDs` reads every page (`limit=50&page=N` until a short page). Tests: 51 labels
     with the wanted one on page 2; the ids returned for known names; the GET method and path.
  4. Writer tests: one success with status 200; `CreateIssue` returns `number`, `html_url` and
     `updated_at` from the reply.
  5. A nil `Writer.Client` must not mean "no timeout": use the same client and timeout as the
     read path (`internal/forge/api.go`).
  6. Token CLI tests through the command, not the helper: `forge token create --write` with the
     fake client and fake terminal stores in the write store and sends the write scopes;
     `revoke --write` and `rotate --write` leave the read token's secret and metadata unchanged,
     and `revoke`/`rotate` without `--write` leave the write token unchanged.
  7. `harnez forge token` help (short and long text) describes both tokens; `status` prints the
     write token's name, scopes, store and created date when one is stored (never the secret).

- Write set: `cmd/harnez/issues_forge.go`, `cmd/harnez/issues.go`,
  `internal/forgetracker/write.go` and test, `cmd/harnez/issues_forge_test.go`,
  `cmd/harnez/issues_comment_test.go`, `cmd/harnez/testdata/help.golden`.
- `requireHostSession()` error: `issues <verb>: forge writes run only in a host session (this is
  harnez worker <session>); hand the write to the host`.
- Tests to lock:
  - `HARNEZ_AGENT_ROLE=developer` with `comment 7 -F f` gives that error; fake fj and API get no
    call. The same with `--dry-run` prints the plan and exits 0.
  - Write token stored: the API is used and the fake fj is not called; output
    `commented on codeberg.org/o/r#7: <url>` and `filed as #N at <url>; no commit`.
  - No write token: fj argv exactly as M1 builds it.
  - `new --forge "T" -F f` on a repo with a local tracker files on the forge and reserves no
    local number.
  - `new --label bug` with a write token sends the label id; an unknown label fails before the
    POST.
- Edge cases: the empty-body refusal (1212 M2) still runs first, and also applies to `new`;
  `--forge` with `show` or `list` is refused.
- Acceptance: tests pass; `harnez issues --help` matches the golden file.

### M5 delivered (2026-11-16)

- Commits `b2f68047` (pre-work) and `624510c8` (M5), developer `forge-dev2` (codex:terra:low).
  Its first write turn stopped after 2 minutes with three uncommitted files and no blocker; the
  second turn (9m29s) finished both commits. Developer's `make test-q1`: exit 0, no `--- FAIL`.
- Delivered: `requireHostSession(verb)`; `issueWriteAPI` and `forgetracker.WriteAPI` (API only
  for `kind: forgejo` with a stored write token); `new` and `comment` over the API with the
  outputs `filed as #N at <url>; no commit` and `commented on <host>/<o>/<r>#N: <url>`;
  `new --forge`; label ids over the API; blank-body refusal for `new`; `-F -` for `new` goes
  through a temporary file on the CLI path; `Writer.Comment` returns the URL; label paging;
  forge message on every failed write; `forge token status` prints write-token metadata.
- Incident: an early version of the worker-denial test ran the real
  `fj issue comment codeberg.org/o/r#7`. fj answered `Error: not found: The target couldn't be
  found.` Host check: anonymous `GET https://codeberg.org/api/v1/repos/o/r` gives 404. Nothing
  was written. The test now replaces the CLI runner.
- Host verification: `go test` of `internal/forge`, `internal/forgetracker`,
  `internal/forgetoken` and `cmd/harnez` is `ok` with fake `fj`, `tea` and `gh` first on `PATH`;
  none of them was called. Installed binary: host dry runs of `comment 41 --forge` and
  `new --forge` print the fj command; with `HARNEZ_AGENT_ROLE=developer` the dry run still
  prints and the real call stops with the host-session error before any CLI runs;
  `list --forge` gives `issues: --forge requires new or comment`.

### Decision (review of M5, host and reviewer `forge-rev3`, 2026-11-16)

- A nil `Writer.Client` returns `forge write: no HTTP client` and sends nothing. This replaces
  pre-work item 5 of M5: the command passes a client with the `issue_scan.timeout` of
  `spec/pr.yaml`, so no second timeout value exists.
- `new` on a GitHub origin keeps the existing `gh` path. Every other forge write on a
  `kind: github` forge is refused (ticket 1209).
- The host-only check covers every write of `new`, the `--remote` / `CLAUDE_CODE_REMOTE` gh
  flow included.
- A dry run prints what would really run: the API plan when a write token is stored, the CLI
  command otherwise. It decides from the stored token's metadata and never reads the secret.
- `edit`, `close` and `reopen` accept `--dry-run` on a forge target: they print the plan, send
  no write request and are exempt from the host-only check, so a worker can hand the exact
  command to its host.

### M6 — `edit`

- Pre-Work / Required Refinements (do these first, own commit `(#1202 M6 pre-work)`):
  1. Test guard for the whole `cmd/harnez` test package (`main_env_test.go`, `TestMain`): the
     default `issueCLIRun` fails with `test did not stub the issue CLI: <argv>`, and the HTTP
     client that `issueWriteAPI` builds comes from a package variable whose test default
     refuses every non-loopback host. Tests that need a CLI or the API replace them as today.
  2. `new` with `--remote` or `CLAUDE_CODE_REMOTE` (`cmd/harnez/issues.go`, the gh flow near
     line 407) calls `requireHostSession("new")` before gh runs; dry runs stay exempt. Test:
     worker markers set, the gh stub gets no call.
  3. `comment` on a forge that is not `kind: forgejo` stops with `issues comment: forge writes
     support Forgejo only; GitHub is ticket 1209` before the issue GET, the token lookup and
     any CLI, also with `--dry-run`. Test: zero calls to the issue source. `new` is unchanged.
  4. Dry runs with a stored write token print the API plan and need no fj on `PATH`:
     `dry run: would comment on codeberg.org/o/r#7 over the API (write token)` and
     `dry run, would file on codeberg.org/o/r over the API (write token)`. Labels are not
     resolved in a dry run. The choice uses the token store's metadata (what
     `forge token status --write` reads), not the secret. Without a write token the output is
     unchanged. Test both, with a token lookup that fails the test when called.
  5. `Writer.LabelIDs` stops at an empty page or at a page that adds no new label id, not at a
     short page (a server page cap below 50 would end the loop early; the cap is unverified).
     Test: pages of 30, 30 and 0 labels with the wanted label on page 2.
  6. Writer error text: no trailing space when the forge message is dropped
     (`forge API request failed: HTTP 403`); the 200 cap counts runes, not bytes.
  7. Stronger tests: the API path of `new` and `comment` runs with an `issueCLIRun` that fails
     the test; `new --json` over the API asserts the exact JSON line; the status loop in
     `internal/forge/issue_write_test.go` (near line 153) asserts the exact `HTTP <code>` text;
     `show --forge` and `list --forge` assert the exact error text; the worker-denial tests
     assert that the token lookup is not called.

- Write set: the cmd files of M5, `internal/forgetracker/forgetracker.go` (`show --json` gains
  `updated_at`) and tests, `cmd/harnez/testdata/help.golden`.
- Tests to lock:
  - `edit 7 --title X` sends PATCH `{"title":"X"}`.
  - `edit 7 -F f` without `--if-updated` and without `--force`: `issues edit: body edits need
    --if-updated <updated_at from issues show --json> or --force`.
  - A different timestamp: `issues edit: o/r#7 changed at <ts>, expected <ts>; run issues show
    again`, and no PATCH is sent.
  - The returned body differs from the sent body: error.
  - Local tracker without `--forge`, and no write token: the texts above.
- Edge cases: `--title` together with `-F`; `-F -` (stdin) keeps the body byte for byte; an empty
  or whitespace-only body is refused; a closed issue needs `--closed`; a pull-request number is
  refused.
- Acceptance: tests pass; help says "a write landing between the check and the PATCH is not
  detected".

### M7 — `close` and `reopen` on the forge

- Write set: `cmd/harnez/issues.go`, `cmd/harnez/issues_forge.go` and tests,
  `cmd/harnez/testdata/help.golden`.
- Tests to lock:
  - Forge `close 7` sends PATCH `{"state":"closed"}`, output `closed codeberg.org/o/r#7`.
  - Forge `close 7 done now` posts the comment `done now`, then PATCHes; when the comment POST
    fails, no PATCH is sent.
  - `reopen 7` sends `{"state":"open"}`; an issue that is already open says so without a PATCH,
    and likewise `close` on a closed issue.
  - Forge `open 7` gives the "use reopen" error.
  - Local `close` is unchanged; its existing tests stay green.
- Edge cases: a 412 from the forge is passed through; the host-only check applies.
- Acceptance: tests pass.

### M6 and M7 delivered with a blocker (2026-11-16)

- Commits `00ef6564` (M6 pre-work), `58c24095` (M6 `edit`), `af87b676` (M7 `close`/`reopen`),
  developer `forge-dev2`. Its first turn stopped after 1m26s on its own outdated paging test;
  the second (9m23s) made the three commits. Its `make test-q1` failed on one outdated assertion
  in `TestIssuesNewForgeErrors`, which it fixed without a rerun.
- Host full run: `make test-q1` exit 0, 78 `ok` lines, no `FAIL`, with fake `fj`, `tea` and `gh`
  first on `PATH` (none called).
- Host check with the installed binary (no write token stored, so nothing could be written):
  - Blocker: `close --forge --dry-run 41` prints the dry-run line and then also runs the local
    verb: `041: already Closed (no change); use --amend-reason to change the reason`. With
    `reopen` and `edit` the second line is `Error: issues: unknown verb "reopen" ...`. Cause
    (reviewer `forge-rev4`, `cmd/harnez/issues.go:326,354`): the forge dispatch for `edit`,
    `close`, `reopen` and `open` sits in cobra's `Args` validator; after it returns nil, `RunE`
    runs the local status path. A real `close --forge 7` would also close local ticket 007.
  - `close --forge` without a number panics: `slice bounds out of range [2:1]` at
    `cmd/harnez/issues.go:354`.
  - As specified: worker markers refuse `edit` and `close` and allow the dry run; `edit -F`
    without `--if-updated` or `--force`, local `edit`, local `reopen`, forge `open` and the
    missing write token give the texts of this ticket.

### Decision (review of M6 and M7, host and reviewer `forge-rev4`, 2026-11-16)

- `show <n> --forge` is allowed (read only, `--json` included); `list --forge` stays refused.
  Reason: on a repo with a local tracker, `edit -F --if-updated` needs the forge issue's
  `updated_at`, and the error text points at `issues show --json`. This replaces the edge case
  "`--forge` with `show` is refused".
- The `--remote` / `CLAUDE_CODE_REMOTE` flow of `new` goes back to its behaviour before
  `00ef6564`; the only addition is the host-only check directly before the gh call that
  creates the issue. The dry-run line added there skipped the title, duplicate and remote
  checks.
- When the reason comment of `close` is posted and the state change fails, the error says so,
  so the user does not post the reason twice.

### M8 — docs, then the host's live check

- Pre-Work / Required Refinements (do these first, own commit `(#1202 M8 pre-work)`):
  1. Blocker: move the forge dispatch of `edit`, `close`, `reopen` and `open` out of the `Args`
     validator into `RunE`, before the local status path. `Args` only validates. After a forge
     verb, dry run or real, no local verb runs. Tests through `runHarnez` in a repo that has a
     local `issues/` dir with a ticket of the same number: `close --forge --dry-run 7`,
     `close --forge --dry-run 7 done now`, `reopen --forge --dry-run 7` and
     `edit --forge --dry-run 7 --title X` each print exactly their one dry-run line and exit 0;
     the local ticket file is byte-identical afterwards and no commit was made. Plain
     `close 7 "reason"` in that repo still closes the local ticket as before.
  2. A missing number is an error, not a panic: `close --forge`, `edit --forge` and
     `open --forge` give `issues <verb>: requires an issue number`.
  3. `close 7 reason`, comment posted, PATCH failed: `issues close: the reason was posted as a
     comment (<url>) but closing failed: <forge error>; run close again without a reason`.
     Test with exact text and request order.
  4. `new` remote flow: remove the dry-run early return added in `00ef6564`; call
     `requireHostSession("new")` directly before the gh call that creates the issue. Tests:
     worker markers with flag-only `--remote` and, separately, with only `CLAUDE_CODE_REMOTE`;
     the gh runner gets no create call.
  5. `show <n> --forge [--json]` reads the forge issue on a repo with a local tracker; the JSON
     has `updated_at`. `list --forge` gives `issues: --forge does not support list`.
  6. Missing tests named by the reviewer: a successful `edit --closed`; an empty body for
     `edit`; pull-request refusal and missing write token for `close` and `reopen`; the dry
     runs of `close` and `reopen`; the no-token dry run of `new` and `comment` with a secret
     lookup that fails the test; label paging that ends on a repeated page; the worker tests of
     `edit`, `close` and `reopen` assert that the secret lookup is not called.

- Docs: `docs/commands/issue.md`, the `harnez issues` help, the Issues rule source (regenerated
  with `make install` and `harnez init -d .`), `docs/Spec.md`; state the host-only limits and the
  non-atomic edit check.
- Live check, host only: the owner runs `harnez forge token create --write`; then new, comment,
  edit (title and body), close, reopen, close on `ubunatic/harnez` (with `--forge`) and on
  `harnez/taia`. Record which unverified facts it confirmed.

### M8 pre-work and docs delivered (2026-11-16)

- Commits `8d9a47c7` (pre-work) and `bd937e33` (docs), fresh developer `forge-dev3`
  (codex:terra:med), one turn of 5m27s. Its `make test-q1` failed on two outdated tests, fixed
  without a rerun; it left most of pre-work item 6 open and said so.
- Host full run: `make test-q1` exit 0, 78 `ok` lines, fake `fj`, `tea`, `gh` on `PATH` not
  called. `make install` and `harnez init -d .` leave `git status` clean.
- Host check with the installed binary (no write token stored): `close`, `reopen` and `edit`
  with `--forge --dry-run 41` each print only their dry-run line; `close --forge`,
  `edit --forge`, `open --forge` without a number give `issues <verb>: requires an issue
  number`; `show --forge 41 --json` returns the forge issue with
  `"updated_at": "2026-10-31T19:57:51+01:00"`; `show --forge 78` says it is a pull request;
  `list --forge` gives `issues: --forge does not support list`; `show 41` still shows local
  ticket 041.
- Reviewer `forge-rev5`: no remaining fallthrough from a forge verb to a local verb; local
  verbs keep their paths; the remote flow of `new` differs from before `00ef6564` only by the
  host-only check; `show --forge` uses the read token only.

### Decision (review of M8, host and reviewer `forge-rev5`, 2026-11-16)

- Local `close` and `open` without a number now say `issues <verb>: requires an issue number`
  (before: cobra's argument-count error). Accepted: clearer, and no valid call changes.
- `new --remote --dry-run` still creates the issue, as before this ticket (reviewer finding,
  not checked by the host). Not changed here; the docs must say that `--dry-run` does not
  cover the remote flow.

### M8 refinements (last developer turn, commit `(#1202 M8 refinements)`)

1. Tests still missing from pre-work item 6 and the dispatch test: no commit is made and the
   local ticket file is byte-identical after the forge dry runs (assert `git rev-parse HEAD`
   before and after); plain `close 7 "reason"` in that repo still closes the local ticket; a
   successful `edit --closed`; pull-request refusal and missing write token for `close` and
   `reopen`; label paging that ends on a repeated page; the no-token dry run of `new` and
   `comment` and the worker runs of `edit`, `close`, `reopen` with a secret lookup that fails
   the test when called.
2. Docs and help state only what the code does:
   - `comment` falls back to fj only; `new` falls back to fj or tea, and to gh on GitHub.
   - `--dry-run` previews forge writes, not the `--remote` / `CLAUDE_CODE_REMOTE` flow of `new`.
   - The help line for `comment --dry-run`: it prints the fj command, or the API plan when a
     write token is stored.
   - `docs/commands/issue.md`: bring back what `bd937e33` dropped, corrected against the code:
     the `harnez find -d <repo> issues` search line (check what it searches before stating
     it), the private-repo token note, the forms without `--forge` for a repository whose
     tracker is the forge next to the `--forge` forms for a repository with local tickets,
     one `--dry-run` example each for `new` and `comment`, and the label guidance (labels must
     exist on the forge; over the API they are resolved by name; fj cannot set labels).
   - Regenerate copies with `make install` and `harnez init -d .`; source and copy in the
     same commit.

### M8 refinements delivered (2026-11-16)

- Commit `e1bbdf05`, developer `forge-dev3`, one turn of 3m43s: tests
  `TestForgeRefinementEdges` and `TestWriterLabelsRepeatedPage`, the extended
  `TestForgeDryRunDoesNotTouchLocalTicket` (no commit after the forge dry runs, local
  `close 7 "reason"` still closes the local ticket), and the docs and help corrections of
  item 2. No new test found a bug. Its `make test-q1` exited 0.
- No reviewer for this commit: it changes tests, docs and help text only; the host read
  every added assertion.
- `origin/main` had moved (ticket 1235, goself install). Merged into the sprint branch as
  `2096aca3` without conflict. Host full run after the merge: `make test-q1` exit 0, 78 `ok`
  lines, fake `fj`, `tea`, `gh` on `PATH` not called. `make install` and `harnez init -d .`
  leave `git status` clean.
- Host check with the merged, installed binary (no write token stored):
  `close --forge --dry-run 41 "done now"` prints the API plan; `comment` and `new` with
  `--forge --dry-run` print the fj command; `reopen --forge 41` with worker markers gives
  `issues reopen: forge writes run only in a host session (this is harnez worker probe-2);
  hand the write to the host`.

### Open: live check (waits for the owner)

- `harnez forge token status` on 2026-11-16: `write token: none`. The owner runs
  `harnez forge token create --write` (password and 2FA code on the terminal).
- Then, host only, one scratch issue each on `ubunatic/harnez` (with `--forge`) and
  `harnez/taia`: new, comment, show --json, edit title and body with `--if-updated`, close
  with a reason, reopen, close. Both issues are public and stay as closed issues.
- Still unverified until then: Codeberg accepts the scope `write:issue` at token creation;
  a token with `read:repository` and `write:issue` can do all five writes and read labels on
  both repositories; `state` accepts `open` and `closed`; PATCH returns the new
  `updated_at`; the forge returns the body byte-identical. Organisation-level labels are
  not covered.
- Tickets 1228, 1212 and 1202 stay open until the live check has passed.
