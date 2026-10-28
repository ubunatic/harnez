# 997 — harnez apply should give agy an allowlist of common read tools so interactive sessions stop asking for ls, cat and harnez

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Feature
**Related**: 995, 996, 373

---

## 1. Problem & Motivation

An interactive `harnez-agy` session (owner, 2026-10-28, harnez 0.2.6+4022) asks for approval of
`ls`, `cat`, `harnez` and other everyday tools. `harnez apply` configures Claude's permission
allowlist (46 allow rules) but writes nothing permission-related for agy: it only sets
`statusLine` in `~/.gemini/antigravity-cli/settings.json` and hooks in
`~/.gemini/config/hooks.json`. agy should get the same common-tool allowlist.

995 solved the headless side for advisor/reviewer with `--dangerously-skip-permissions --sandbox`;
interactive sessions need a real allowlist, not skip-all.

## 2. Technical Specification / Findings

- `agy --help` has no allowlist flag (only `--dangerously-skip-permissions`, `--sandbox`).
- The agy binary (`~/.local/bin/agy`, `strings`) knows: JSON keys `agentPermissions`, `allowlist`,
  `allow`, `deny`, `autoExecutionPolicy`; enums `AUTO_EXECUTION_{OFF,AUTO,EAGER,PROCEED_IN_SANDBOX}`,
  `AUTO_EXECUTION_POLICY_{REQUIRE_REVIEW,PROCEED_IN_SANDBOX,NOT_ENFORCED}`,
  `PERMISSION_PRESET_{NONE,DEFAULT,AUTO,REQUEST_REVIEW}`, `PermissionGrants`. The settings.json
  shape for them is not documented.
- Current `settings.json` keys: `colorScheme`, `model`, `statusLine`, `trustedWorkspaces`.

## 3. Implementation & Verification Plan

1. Canary first (host): find the settings shape agy honours for a command allowlist and an
   auto-execution policy (agy docs, its `/permissions`-style UI writing settings.json, or a diff
   of settings.json after approving "always allow" once). Record the exact shape here.
2. Spec: the common-tool list lives in `spec/` (reuse the source of Claude's 46 allow rules where
   the semantics match: read tools, `ls`, `cat`, `rg`, `git status/diff/log/show`, `harnez`
   read commands, `make test*`), with its schema; no Go list.
3. `harnez apply` merges the agy allowlist into `~/.gemini/antigravity-cli/settings.json`
   idempotently, keeping user keys; drift shown by status like the agy hooks.
4. Tests: merge into empty / existing settings, idempotency, user keys kept.
5. Canary: a fresh `harnez-agy` session runs `ls`, `cat`, `harnez issues list` without a prompt
   and still asks before a write outside the allowlist.

## Canary 1 (host, 2026-10-28, 22:41)

The owner approved many tools in a live `harnez-agy` session (conversation 6a040f1a). No config
file changed: `settings.json` keys unchanged (last write 22:33 at session start),
`~/.gemini/config/` untouched. Only the conversation DB (`conversations/<id>.db`, tables
`trajectory_meta`, `steps`, `trajectory_metadata_blob`, ...) and summaries changed. So the
approvals are per conversation and are not persisted as a global allowlist. The global shape for
`agentPermissions` / `allowlist` / `autoExecutionPolicy` still has to come from agy docs or the
prompt's own scope options (e.g. "always allow" / "for this workspace", if offered).
- Owner also had to approve plain file `edit` in the same session (22:45). The allowlist must
  cover agy's built-in file tools (view/list/edit), not only shell commands; whether `edit` is
  auto-allowed inside trusted workspaces is a decision for the owner.

## Catalog of owner approvals (host, 2026-10-28, 22:55)

Source: the `permissions` blob of each `steps` row in `~/.gemini/antigravity-cli/conversations/*.db`
(read-only). A grant is `{kind, prefix}`; agy offers the command's **prefix** as the grant scope
(e.g. `git status`, `cat`, `bash -c '`). 54 grants: 46 `command`, 8 `write_file`.

| Group | Prefixes granted | Proposed wildcard tier |
| --- | --- | --- |
| read-only shell | `ls`, `cat`, `echo`, `which`, `strings`, `find`, `git status`, `git diff`, `git log` | **auto**: plus `head`, `tail`, `wc`, `rg`, `grep`, `file`, `stat`, `git show`, `git branch`, `git rev-parse` |
| harnez / agy | `harnez`, `agy` | **auto** for `harnez` read verbs (`read`, `find`, `issues list`, `agent list`, `usage`, `--version`); `agy` only `--help` |
| build / test | `make test`, `make test-q1`, `make install` | **auto**: `make test*`, `make lint`, `go test`, `go build`, `go vet`; `make install` auto (repo rule) |
| local git writes | `git add`, `git commit`, `git checkout`, `git config` | **auto**: `git add`, `git commit`; **ask**: `git checkout`, `git config` (can switch branch / change remotes) |
| fs writes | `mkdir`, `write_file` in the workspace (8, all under the project) | **auto** inside trusted workspace; **ask** outside |
| shell wrappers | `bash -c '`, `bash -i -c '`, `zsh -c '`, `fish -c '`, `./dotfiles` | **ask**: a wrapper prefix approves any script (no real scope) |
| never auto | — (none granted) | `git push`, `rm -rf`, `sudo`, `curl \| sh`, `--dangerously-*` |

Notes:
- Shell-wrapper grants were all for alias / completion probing in dotfiles; they defeat any
  allowlist, so they stay `ask` even though the owner approved them once.
- Grants are per conversation (canary 1), so the catalog is the input for the spec list; the
  global settings shape (canary step 1) is still unknown.

## Decision (owner, 2026-10-28)

The prefix tiers above are not a security boundary: `make test` runs whatever the agent wrote into
the Makefile. The owner treats the dedicated agent account on this machine as the sandbox. So
`harnez apply` writes a broad agy allowlist (the catalogued prefixes plus the additions, and the
in-workspace file tools), with a short ask list as an accident guard (`git push`, `sudo`,
recursive deletes outside the workspace). The list is a spec value, so another machine can narrow it.

## Decision refined (owner, 2026-10-28)

Goal of the allowlist: agents must not break their own machine, and they are **steered** to
`make` targets and Go code instead of ad-hoc commands. Not a security boundary, a guard rail:

- **allow**: `make <target>`, `go` (build/test/run/vet), `harnez`, read tools (`ls cat head tail
  wc rg grep find stat which`), local git (`status diff log show branch add commit`), in-workspace
  file tools.
- **ask** (so the agent writes a make target or Go program instead): shell wrappers (`bash -c`,
  `zsh -c`, `fish -c`, `sh -c`), pipes into interpreters (`| sh`, `| python`), `python`/`node`
  one-liners, `curl`/`wget`, `chmod`/`chown`, `rm -r`, `mv`/`cp` outside the workspace,
  `git push`/`checkout`/`config`/`reset`, `sudo`, `systemctl`, `kill`/`pkill`, package managers.
- This replaces the "broad allowlist incl. shell wrappers" note above: wrappers stay **ask**,
  because they are the main door to crazy commands.
