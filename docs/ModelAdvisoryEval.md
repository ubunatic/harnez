# Model Advisory Eval

What five cheap and mid-tier models got right and wrong when each one planned the same sprint.
Use this to decide which model gets which role in a harnez sprint. For general model traits, see
[Models.md](Models.md). For doc-delivery benchmarks, see [Bench.md](Bench.md).
The repo-independent practice distilled from it is
[practices/ModelRoles.md](practices/ModelRoles.md) (copy with `harnez init --docs model-roles`).

## Setup (2026-09-22)

- **Task:** one read-only advisory prompt, identical for every model. The prompt asked each
  model how it *would* run a lean sprint over `docs/Roadmap.md` "Suggested order of attack"
  steps 1–5 (tickets 496, 488, 289, 279, 491, 493, 495, 492, 446, 445, 355, 494). The answer had
  five sections: order, per-ticket approach, delegation, traps with `file:line`, and a cut list.
  The limit was 60 lines, and the models could not edit files, run tests or spawn agents.
- **Codex models** ran through `codex -a never -s read-only exec -m gpt-5.6-<m> -c
  model_reasoning_effort=<e>`, not `harnez agent`. `harnez agent` does not pass the tier to
  Codex (issue 497): every tier falls back to `~/.codex/config.toml` (`low` here), so it
  would have run `luna:med` at low.
- **Claude models** ran as native Claude Code subagents (`general-purpose`, `model: haiku` or
  `sonnet`). `harnez agent` maps `claude:sonnet` to a stale 3.7 model ID (issue 497).
- **Grading:** every answer was checked against the repo by the Opus orchestrator. Trap claims
  were checked with grep or a range read, not taken on trust.

## Cost and speed

| Model | Wall time | Tokens | Tool calls |
|---|---|---|---|
| codex luna:low | 56 s | 144k in (99k cached), 2.1k out | 4 |
| codex luna:medium | 82 s | 287k in (228k cached), 3.0k out | 7 |
| codex terra:low | 101 s | 220k in (152k cached), 2.7k out | 7 |
| claude haiku | 134 s | 60k (subagent total) | 24 |
| claude sonnet | 48 s | 49k (subagent total) | 5 |

The Codex and Claude token counts are not comparable. Codex reports cumulative input per turn,
while Claude Code reports a subagent total. Compare within each vendor only.

**Subscription cost is not measured yet.**

- Uncached Codex input was 45k for luna:low, 59k for luna:med and 68k for terra:low.
- `harnez usage --json` shows quota only as whole percentages per window, so a single advisory
  run doesn't move it.
- There are no rate cards yet (issue 445).
- To calibrate: run each model N times at the start of a fresh 5-hour window and read the
  change in `used_percent`, one model per window.

## Calibration: luna:low lean sprint (2026-09-22)

The first real measure of cost. One luna:low developer did 496, 488 and 497 (three milestones)
through `harnez agent`. There were 7 developer turns plus one `luna:med` canary, about 520k
uncached input tokens in total (about 6M cached), over 14 minutes of wall time.

| Quota window | Before | After |
|---|---|---|
| Codex 5-hour | 0% | 2% |
| Codex weekly | 96% | 96% (moved by less than 1 point) |

So a small luna:low ticket costs well under 1% of a 5-hour window. The 4% weekly headroom
is enough for several sprints of this size.

Quality of the luna:low work:

- 496 and 488 were right on the first try.
- 497 M1 had a blocking bug: resume hard-coded `effort=medium`. The developer "updated one
  stale assertion" to match the bug, so the tests stayed green. Host review of the diff caught
  it.
- M2 fixed the bug with runtime interface checks. M3 made the structure clean.

Lesson: when a cheap developer reports that it changed an existing assertion, that is where
the host must look first.

## Role assignment

The setup used from 491 on. It follows from the fact checks below and from the two sprints.

| Role | Model | Why |
|---|---|---|
| Host / orchestrator | claude opus | Reviews diffs, writes pre-work, catches assertion-hidden bugs. Writes no code. |
| Advisors (discovery) | codex terra:low + claude sonnet | Different vendors, different blind spots: terra finds code traps, sonnet finds cross-doc and dependency issues. Together they covered all four key findings. |
| Developer, clear small ticket | codex luna:low | Cheapest. Right on the first try for 496 and 488. |
| Developer, interface or design change | codex luna:med | luna:low hid a bug behind a changed assertion in 497. luna:med plans well and has changed no assertion so far. |
| Reviewer at the risk seam | claude sonnet | Independent vendor, fast. |
| Mechanical execution only | claude haiku | Unreliable as an advisor. Use only with explicit acceptance tests. |

## Sprint 491 with this setup (2026-09-22)

The first sprint to use the role table, and the first advisory run through `harnez agent`
after 497 fixed tier passing.

| Step | Model | Wall time | Tokens |
|---|---|---|---|
| Advisor | terra:low | 1m06s | 63k new, 253k cached |
| Advisor | sonnet | 1m12s | 332k total |
| Plan (read-only) | luna:med | 1m39s | 75k new, 136k cached |
| M1 config keys | luna:med | 3m41s | 102k new, 838k cached |
| M2 flag, schema gating | luna:med | 8m03s | 184k new, 3.4M cached |

Codex quota moved from weekly 97% / 5-hour 5% before the advisors to 98% / 11% after M2. The whole
luna:low sprint (496/488/497) moved the weekly counter by less than 1 point. This one moved it
by 1 point after two advisors, a plan and two milestones, so luna:med work costs visibly more.
Whole-point resolution can't say how much more.

Observations:

- **The two advisors converged.** Both proposed the same four milestones and the same main
  traps: `ensureTelemetrySchema` running before config load, the `applyMerge` delete
  semantics and ownership, and `DiffAll` skipping Codex/AGY under a selection. Every `file:line`
  claim checked out. When two different-vendor advisors agree, the host can write pre-work
  directly from them.
- **They split on the review seam.** terra put it at the single settings write, sonnet at the
  delete-on-nil path destroying user keys. These are the same code; sonnet named the failure
  mode. Merge both into the pre-work.
- **A read-only plan turn pays off with luna:med.** The plan found two gaps in the host's
  pre-work: `SkillRequires` also has callers in `apply.go` and `plan.go`, and `Config` has
  runtime-only fields that a round-trip test must skip.
- **luna:med skips a listed acceptance test and still reports "open problems: none".** M2
  omitted the byte-identity test from its milestone. The host must check each listed
  acceptance item against the diff, not trust the report. The same applies to luna:low.
- **Host review adds what advisors miss.** Neither advisor saw that a typo in `requires:`
  silently removes the skill under every selection. The host caught it while reviewing M1,
  and it became M2 pre-work.

## Reading Codex quota

- Codex reports `used_percent` as whole numbers only. Rollout logs store it as floats, but
  no value in history has ever had a fraction. The resolution is 1 point per window, so
  measure batches, not single runs.
- Codex writes the rate limits after every turn into the session rollout
  (`~/.codex/sessions/…/rollout-*.jsonl`). `harnez agent delete` also deletes that rollout,
  so read the per-turn numbers before deleting a session, or they are gone.
- `harnez usage` shows two Codex windows: the 5-hour window and the weekly window. The
  weekly one is the binding constraint for a multi-ticket sprint.

## Fact checks

Each check is a claim you can verify in the repo. ✓ means the model caught it, ✗ means it missed
it or got it wrong, and ~ means partly.

| Check | luna:low | luna:med | terra:low | haiku | sonnet |
|---|---|---|---|---|---|
| 289 is already fixed: `ignoredModuleScanDir` skips `testdata` (d602096, `gowork_test.go:178`) | ✗ plans to reimplement | ✓ | ✓ "audit/close" | ✗ | ✗ |
| 279: don't unlink the lock (open-before-unlink inode race) | ✓ | ✓ | ✓ | ✗ "old sidecar gone" | ✓ |
| MVP writes `settings.json` twice (`components/apply.go:77-103`); 491 must make it one write | ✓ | ✓ | ✓ | ~ | ✗ |
| `applyMerge` can only add or replace, not delete (`claude/apply.go:77-95`) | ~ | ✓ | ~ | ✓ | ✗ |
| `ensureTelemetrySchema` runs before config load (`cmd/harnez/main.go:521`), so gating it needs reordering | ✗ | ✗ | ✓ | ~ names it, not the ordering | ✗ |
| Three sprint commands hard-code `harnez agent start` (`docs/commands/{sprint,lean-sprint,reverse-sprint}.md`) | ✗ | ✗ | ✗ | ✗ | ✓ |
| 492 depends on 491 (ticket header), despite the roadmap's "alongside" | ~ ordered | ~ ordered | ~ ordered | ✗ "parallel" | ✓ explicit |
| 495's peer detection needs 491, but its capture API does not | ~ | ~ | ✓ scopes it | ✓ | ✗ "no dependency" |
| 488: worktrees have a `.git` *file*; the HEAD check must not break them | ✓ | ✓ | ✓ | ✗ | ✓ |
| Wrong citations | none | none | one weak (494 vs `README.md.lock` ignore) | 447 cited as a telemetry ticket (it is a matrix-color ticket); an unverified macOS/Linux git-exclude claim | none |

## Judgment quality

- **Cut list.** luna:med, terra and sonnet cut the risky long tail: 279, the 494 migration and
  445. luna:low deferred 279 and trimmed 445 and 494. haiku kept 279 and 494 in scope and only
  trimmed their edges, which is the weakest cut.
- **Delegation map.** All five agree that 491, 493 and 495 need a stronger model or a design
  check. Where they differ:
  - haiku rates 445 and 492 as cheap-solo work.
  - terra and sonnet rate 279 as design-heavy.
  - sonnet singles out 495's schema as the place where a weak model over- or under-builds.
  - sonnet's and terra's reviewer points match the real risk seams: after 491 and after 495/446.
- **Instruction adherence.** All five stayed read-only.
  - terra followed the repo's tool-feedback protocol unprompted. The read-only sandbox blocked
    its `harnez rate` call, and terra reported that honestly.
  - haiku used 24 tool calls for a 60-line answer, the least efficient exploration.

## Takeaways

1. **terra:low was the best Codex auditor for the cost.** It found the one trap nobody else saw
   (telemetry schema ordering), had the most code-grounded traps and made the right 289 call.
   It was the slowest of the Codex runs.
2. **luna:med beats luna:low on facts.** luna:med caught 289 and the `applyMerge` limit, at
   about 2× the input tokens. luna:low is fine for executing a clear ticket, but not for
   auditing whether a ticket is still valid.
3. **sonnet was the best at cross-document judgment and the fastest.** It was the only model to
   read the ticket headers against the roadmap (492's dependency) and to check how widely the
   skill files are affected (493). It was weaker on code-level traps and missed 289.
4. **haiku is unreliable as an advisor.** It made a wrong citation, proposed the one fix the
   ticket explicitly forbids (279) and made the weakest cuts. Keep it for mechanical execution
   with explicit acceptance tests.
5. **No single model caught everything.** Of the four highest-value findings (289 fixed,
   schema ordering, skill fan-out, 492 dependency), the best single model got two. Merging
   **terra:low and sonnet** covers all four. Two cheap, different-vendor advisors beat one
   expensive one for discovery.
6. **Stale-ticket detection is a model-quality signal.** A ticket whose fix already landed is
   the cheapest waste to avoid. Only the models that read code before planning caught it
   (luna:med, terra).

## How to rerun

1. Put the prompt in a scratch file.
2. Run the Codex models in parallel through `codex -a never -s read-only exec --json`, with an
   explicit `-c model_reasoning_effort`.
3. Run the Claude models as native subagents.
4. Extract the answers with
   `jq 'select(.type=="item.completed" and .item.type=="agent_message")'` and the usage from
   `turn.completed`.
5. Grade against the repo, not against each other.

Once 497 is fixed, `harnez agent start --role advisor --model codex:luna:med` should give the
same result. Rerun one cell through it to confirm.
