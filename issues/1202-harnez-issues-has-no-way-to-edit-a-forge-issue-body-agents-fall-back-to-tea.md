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

### M5 — host-only check; `new` and `comment` over the API with fj fallback

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

### M6 — `edit`

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

### M8 — docs, then the host's live check

- Docs: `docs/commands/issue.md`, the `harnez issues` help, the Issues rule source (regenerated
  with `make install` and `harnez init -d .`), `docs/Spec.md`; state the host-only limits and the
  non-atomic edit check.
- Live check, host only: the owner runs `harnez forge token create --write`; then new, comment,
  edit (title and body), close, reopen, close on `ubunatic/harnez` (with `--forge`) and on
  `harnez/taia`. Record which unverified facts it confirmed.
