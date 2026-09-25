# Usage System: Advisor Handoff and Workflow Friction (2026-09-25)

## 1. Header & Context

- **Date:** 2026-09-25
- **Scope:** Continuing issue 560's Usage System work: assess status-line usage data, record a follow-up, consult Terra about when to integrate the public library, and leave an accurate restart point.
- **Starting state:** M1's standalone `usage` snapshot package and compatibility reader were already committed. The CLI and current collector were still separate. The worktree also had concurrent benchmark changes during part of the session.
- **Intended goal:** Decide whether a flag-gated CLI/watch integration was timely, get Terra's advice, and keep changes to the new library isolated from the installed `harnez` binary.

## 2. Executive Summary

The session produced a useful integration sequence and a durable handoff, but no new library code. Terra recommended waiting until controller ownership/IPC (M2) and centralized fetch policy (M3) exist before moving CLI one-shot, JSON, watch, and history paths (M4). A snapshot-only experiment could be made earlier, but would not exercise shared collection and would need careful labeling.

The process worked best when the question to Terra was bounded and the resulting advice was translated into an issue note. Two workflow faults are worth preserving: a compaction attempt was treated as unverified and required a fresh advisor session, and I reverted a generated docs index update before understanding it. The user reran the index command, recovered the valid entry, and committed it. No source data was lost, but the checkout was an unnecessary and risky action.

## 3. What Worked Well

- **Grounding the advisor question in the actual decision:** The ask was whether to integrate the current `harnez usage` command and watch mode behind a flag now or wait. Terra's answer separated a snapshot-only pilot from an actual controller/client integration and put the latter after M2 and M3.
- **Keeping the public package isolated:** The M1 reader remained standalone. The turn did not modify `internal/usage`, `cmd/harnez`, or the binary path while the user had asked to avoid those changes without prior review.
- **Turning advice into a restartable handoff:** Issue 560 now records the M1 state, the recommended M2 → M3 → M4 ordering, and the portability/lifecycle/API questions to resolve before controller implementation. The status-line follow-up was filed separately as issue 564.
- **Using persisted repo records:** Terra's recommendation and the reason for not starting M2 were committed in issue 560, so a later session need not rely on this chat transcript.
- **Recovering the generated index entry:** After the user reran `harnez index -d .`, a diff exposed one valid row for the new benchmark study. That made the mistake visible and the user explicitly asked to commit that row; it landed in `db8e0e0`.

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

### Advisor session compaction could not be verified

The existing Terra advisor session had a very large accumulated context. I attempted `harnez agent compact --name usage-system-design`, but got no usable summary and could not confirm from the available telemetry that compaction had happened. Reusing that session would have risked an advisor answer based on a bloated or stale context. I instead started a fresh read-only session, `usage-integration-review`, through Harnez with Terra at medium effort. It returned a concise recommendation without editing files.

**Lesson:** Treat compaction as successful only when a summary or another explicit signal is visible. If a resumed advisor's context state is uncertain, start a new bounded session and provide the precise decision and relevant file/ticket pointers. Record the new session name and role in the handoff. The limitation was not that Terra could not answer; the uncertainty was in session lifecycle observability.

### Issue search did not locate the requested ticket

`harnez find -d . issues "560 consolidate usage collection"` returned no result, and searching for `560` returned an unrelated ticket. `harnez issues show 560 --json` did reliably return the issue path and body. This cost extra command turns while following the repository's requirement to use Harnez issue tooling.

**Lesson:** When `harnez find` yields no result for a known ticket, switch to `harnez issues show <number> --json` instead of repeatedly changing fuzzy search terms. The JSON response gives a stable path for edits.

### A valid generated index row was discarded

While updating issue 560, `harnez index -d .` reported `issues/README.md up to date` and `updated docs/README.md`. I saw the docs index change as unrelated and ran `git checkout -- docs/README.md`. That reverted a legitimate generated entry for `studies/2026-09-25-bench-read-conditions.md`. I did not inspect the diff before discarding it. The user reran `harnez index -d .`, asked for a diff, and then authorized a commit of the restored one-line entry (`db8e0e0`).

**Impact and recovery:** The study itself was not deleted, and its index row was regenerated. There was no lasting data loss. The initial checkout was still a near-miss: it discarded a shared-workspace edit without checking its content or ownership. The initial `git status` had been clean for `docs/README.md`, but that did not make a later generated edit disposable.

**Lesson:** Inspect every generated diff before reverting it. If an index command updates a file that appears outside the immediate ticket, determine what source entry caused the change. Revert only demonstrably unrelated output, and never use a path-wide checkout as a shortcut around review.

### No M2 implementation was attempted

After Terra's recommendation, the user said to continue. I inspected the M2 criteria and stopped before coding because runtime platform support, lock/socket portability, owner lifetime, and the refresh/subscription IPC surface remain open and become durable public API decisions. This avoided inventing compatibility behavior, but the stopping explanation could have been more direct: the next step is to record or decide these constraints, then implement M2; this was not a code failure or test failure.

**Lesson:** For an authorized architecture task, turn unresolved choices into a short concrete decision list and continue with independent work where possible. If implementation really depends on those choices, leave an explicit checkpoint, as issue 560 now does, rather than implying M2 is underway.

## 5. Quality & Invariants Audit

| Area | Assessment | Evidence / limits |
|---|---|---|
| Architecture & module separation | Preserved | M1 is in `usage/`; this session did not connect it to `cmd/harnez` or `internal/usage`. Terra advised delaying integration until ownership and fetch policy are implemented. |
| Idempotency | Partly demonstrated | `harnez index` reported `issues/README.md up to date`; it added a single docs index row. We did not rerun it after restoring the row, so full idempotency was not independently checked. |
| Backward compatibility | M1 baseline remains covered | Earlier M1 tests cover legacy snapshot decoding and versioned JSON. No compatibility code changed in this session. |
| Test coverage & verification | No new code verification | Prior M1 verification passed with `go test -count=1 ./usage` and `go build ./usage`. No tests were needed or run for the issue/index documentation updates. |
| Workspace safety | One near-miss, recovered | A generated `docs/README.md` row was reverted without diff inspection, then regenerated and committed after the user requested a diff. The final index commit contains only that one row. |
| External side effects | None | Terra's advisor session was read-only; no service, provider API, or remote repository action was performed. |

## 6. Efficiency & Velocity Assessment

There is no reliable wall-clock or token accounting for this continuation, so this story does not estimate hours saved or agent cost. The advisor interaction produced the key sequencing answer in one fresh session. Session recovery and the failed fuzzy issue lookup added avoidable tool turns. The index mistake added a user correction and a recovery cycle that could have been avoided by inspecting the generated diff before checkout.

Qualitatively, the productive pattern was: pose Terra a narrow decision, capture the answer in issue 560, and stop before making an irreversible public interface choice. The weak pattern was using a destructive Git operation based on the filename/change category rather than reviewing the actual diff.

## 7. Key Learnings & Evergreen Upstream

- Confirm advisor compaction from visible output before reusing a large session; otherwise start a fresh, bounded advisor session.
- For known issue numbers, use `harnez issues show <number> --json` when fuzzy search does not return the expected ticket.
- Review generated index diffs before deciding whether they are unrelated. A generated file can contain another contributor's newly added source entry.
- Preserve the user's authorization boundary as an explicit checkpoint: library-only package work may proceed independently, while CLI/binary integration needs advance notice and confirmation under this task's instruction.
- Record Terra advice and unresolved architecture choices in the issue, not only in chat. For this work, decide supported OSes, lock/socket mechanism, owner exit/idle policy, and IPC semantics before M2 code.
- No AGENTS.md change is proposed from one occurrence. If the generated-file checkout pattern recurs, add a narrowly scoped Git hygiene rule to the agentic workflow docs.

## 8. File & Diff Summary

- **Created in this story:** `docs/studies/2026-09-25-usage-system-advisor-handoff.md`.
- **Updated for indexing:** `docs/README.md` will contain the generated row for this story.
- **Earlier work referenced:** M1 library files `usage/snapshot.go` and `usage/snapshot_test.go`; follow-up issue 564; issue 560 handoff.
- **Commits relevant to this case:**
  - `bf9e275` — M1 public snapshot reader (starting baseline, not created in this continuation).
  - `1e57ee6` — file issue 564 for session status-line usage data.
  - `95ebb6b` — record issue 560 handoff and Terra sequencing advice.
  - `db8e0e0` — restore/index the benchmark study row after the accidental checkout.
- **Deleted files:** None.
