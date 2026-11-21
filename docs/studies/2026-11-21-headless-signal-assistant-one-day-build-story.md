---
kind: story
---

# Story: one day of a headless Signal assistant building itself (2026-11-21)

This is a **one-day, multi-repo story**. Almost none of the code it describes lives in harnez. The work happened in `~/projects/hub` (the event hub) and `~/projects/signal-bridge` (the Signal bridge), driven from a headless main Claude session whose cwd is `~/projects`, not a repository. The story is filed in harnez because harnez supplied the agent harness (`harnez agent`, model tiers, cost gate, quota reads) and owns the `/story` skill. All numbers below come from the two repos' git logs, the hub worker logs and the hub ledger, not from harnez git.

Previous story: [2026-11-19 session isolation, forge parity and the quota pack](2026-11-19-session-isolation-forge-parity-and-quota-pack-story.md). It covers harnez development; this one is a different kind of day (an assistant-operations day), so it has no harnez commit range and no baseline in this repo. The baseline is the state of `hub` and `signal-bridge` at the start of `2026-11-21` (`signal-bridge` `d128900`).

Review: **no second-vendor review was possible** (see the Review Note at the end).

## 1. Header & Context

- **Date and window:** `2026-11-21`. Owner messages start `17:19 +0100`; building ran from the owner's go-ahead at `21:42` to the last fix commit at `23:52`; the story was ordered at `23:59`.
- **Cast:**
  - *Owner (Uwe)*: talks only over Signal from the phone.
  - *Hub main session*: a headless Claude (Opus) session, cwd `/home/alfred/projects`, woken by the `hub` daemon for each Signal, worker or health event. It writes and sends nothing but `signal-reply` calls and `hub spawn` calls.
  - *Hub workers*: `claude:haiku:high` developers (one feature per worker, repo as cwd) and `agy:neon:med` (Gemini) reviewers for bigger features. This story's writer is a `claude:sonnet:med` worker.
- **Repos touched:** `hub` (3 commits), `signal-bridge` (10 commits), `harnez` (this file). Plus the machine (Esprimo, CachyOS): kernel update, swap and the `voxi` speech services.
- **Starting state:**
  - The hub dropped an event after three failed main turns. One `are you online` from `2026-11-20 21:50` had been lost because the Claude OAuth session had expired. Hub issue `006` was filed but unbuilt.
  - The bridge sent plain text only and ignored attachments, reactions and polls.
  - Esprimo had a stale kernel and 9.6 of 14.9 GiB swap in use.
- **Goal, as the owner stated it at `20:31`:** "Make a plan to improve my Signal experience. Can you send rich text? Can you send CTAs? Can you send a survey?" At `21:42`: "go for all of them, pick the order yourself ... you use Haiku high as a developer. When you implement bigger features, ask Neon to review the code and the feature and the assumptions."

## 2. Executive Summary

In about 2 h 10 min of building (`21:42` to `23:52`), one Opus main session and 12 Haiku developer sessions plus 5 Neon reviews shipped, in two Go repos:

- **hub 006:** outage hold and retry instead of dropping events (`4ed6dd4`, hardened in `43fc639` and `7103cd6`), plus `hub failed` and `hub retry`.
- **signal-bridge:** styled text, 👀 acknowledgement, typing indicator, reactions forwarded as text (`f0f9c9a`, `85de201`); polls (`fc7a59d`, `ed7c1c4`, ticket `011` closed); attachments in both directions (`9bafb78`, `c060f80`); voice-note transcription via the local `voxi` llama-server (`c57ee57`, `54eb7ce`); a race-condition test fix (`80ed13c`); the `-h` incident fix (`b83567c`).

Totals: 13 commits in two repos, `+7,014 / -715` lines gross, 35 review findings (10 High), 4 live checks by the owner. Everything is committed **locally, nothing pushed**.

The owner typed 12 messages, voted once, and sent one photo and one voice note, about 15 inbound events in a 6.5-hour window, and rebooted the machine once. He answered none of the open decision questions except through the poll and `drop`.

Honest headline: the flow worked because tasks were small, every feature had a second-vendor review that found real bugs, and tests used fakes. It was **not** a clean day. Two workers failed at launch on a moved binary path, one worker sent `-h` to the owner's phone, a hung test cost 15 minutes, a pipe hid a failing test and I restarted the bridge on top of it, and several worker contexts grew past the 100k tokens where Haiku input costs 5x.

## 3. What Worked Well

- **One feature per worker, one commit per worker.** Ten developer sessions produced the 13 non-merge commits of the day (one to three commits each), each with tests and a report (commit hash, per-item changes, test output, open questions). The reports are the reason the main session could verify cheaply: `git log -1`, `git status`, one `go test`.
- **Cheap developer plus second-vendor reviewer.** Haiku-high wrote; Neon-med (Google) reviewed from a different model family. The reviews are the day's best investment: 35 findings, 10 High, across five reviews of 1m30s to 6m08s each (about 18 minutes total). Details in section 4.
- **The review loop was cheap to run.** Review prompt names the commit, the doc and the assumptions to test; fix prompt says "read the review log, fix findings 1-N, one commit, tests". The fix worker gets the review verbatim from `~/.local/state/hub/logs/`, so nothing is paraphrased. The fix commits followed their reviews by 4 to 24 minutes (the 24 minutes for polls include the owner's live vote).
- **Swagger as source of truth.** Every `signal-cli-rest-api` call was checked against `GET /swagger/doc.json` on the live instance (GET only). Reviewers re-checked against it and against `AsamK/signal-cli` source. This caught real shape problems (single-value vs array `/v2/send` response, poll `optionIndexes`, `allow_multiple_selections` being poll-wide, not per option, the data-URI `filename=` form).
- **`httptest` fakes with a hard "no live calls" rule.** After the `-h` incident, every prompt carried: never run `signal-reply` or the bridge binary, never POST to `127.0.0.1:8080`, never restart the service, and `TestMain` points config and state at a temp dir. The `sig-help`, `sig-poll-fix`, `sig-attach`, `sig-voice` and `sig-voice-fix` logs each restate that the rule was kept. No second slip happened.
- **Local `voxi` for speech.** The voice note went phone to transcript in under a second on the PC (`This is a short voice message.`, logged at `23:54:37`), with no audio leaving the machine, no key and no per-minute cost.
- **Mutation checks in worker reports.** `sig-help` disabled the early return and showed the new test failing; `sig-attach-fix` ran its new listen test against the old synchronous loop in a scratch copy and showed it failing; `sig-voice-fix` showed its process-group test failing without the group kill. These are the reports that could be trusted.
- **Polls as the decision interface.** The main session asked "What should I build next?" as a real poll, and the owner's vote (`[poll vote] What should I build next?: Send images and charts`) arrived at `23:04`. That is a one-tap decision from a phone, which is the whole point of the project.
- **Go single binaries, `make install`, systemd user services.** See section 5.

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

| # | What happened | How it was caught | Fix |
|---|---|---|---|
| 1 | **The `harnez` path moved.** The hub's built-in default is `~/go/bin/harnez`; harnez now installs to `~/.local/bin/harnez`. `hub-006` and `sig-rich` died at launch (0-byte logs `hub-006-20261121-214259.log`, `sig-rich-20261121-214305.log`). | The main session got failure events within a minute and read the config defaults (`main.go:95`). | A symlink as a stopgap, and `harnezPath()` in `4ed6dd4` looks at `PATH` and falls back to `~/.local/bin`. Both tasks were respawned as `hub-006b` and `sig-rich2`. About 1 minute lost, no owner impact. |
| 2 | **Cost gate refused `agy:neon:high`** (`sig-rich-review-20261121-214907.log`: "costly (relative cost 219 ... pass --accept-cost"). The hub cannot pass `--accept-cost`. | The reviewer log was 130 bytes. | Reviewers moved to `agy:neon:med` (relative cost 109). The review quality was good enough that this is a net saving, but note the owner implied the high tier. |
| 3 | **`-h` incident.** `sig-rich2` ran `signal-reply -h` to see the help while testing live; inferred cause (the sources do not show the exact call): the same worker's fix #2 (commit `85de201`, from the reviewer's "only consume explicit flags so a leading `-` passes through") made `-h` an ordinary message. The owner received `-h` at `22:00` and wrote "you just sent me -h". | The owner told us. Nothing in the pipeline could have caught it: the worker used the real binary, so the test fakes never ran in that call. | `b83567c` (`signal-reply -h`/`--help` print usage, send nothing) with `TestReplyHelpSendsNothing`, mutation-checked. The main session apologised on Signal and added the "never run `signal-reply`" rule to all later prompts. **It was the day's only owner-visible bug.** |
| 4 | **A hung test cost 15 minutes.** `sig-rich2`'s first `make test`/`-race` hung on `TestSendSignalTimesOut` in the background until its time limit (`sig-rich2-20261121-215145.log`: 15m29s for a fix that was ready earlier). | The worker's own report. | The worker fixed the test-server bug and reran in the foreground. Pure wall-clock waste. |
| 5 | **Review caught the root-owned inbox path.** Attachments defaulted to `/signal-bridge/inbox`, a root-owned path that does not exist on a single-user host: every inbound file would have failed with `mkdir /signal-bridge: permission denied`. The worker's own report listed it as an open question. | Neon finding 1 on `9bafb78`, plus the worker's open question. | `c060f80`: default to the XDG state dir if `/signal-bridge/inbox` is not writable. The live photo later landed in `~/.local/state/signal-bridge/inbox/2026-11-21/`. |
| 6 | **Review caught blocking downloads.** Downloads ran synchronously inside the receive loop, so the keepalive ping starved and a long download could cancel its own context and drop the WebSocket. Same class as ticket `002`. | Neon finding 2 on `9bafb78`; also earlier High finding 1 on `f0f9c9a` (unbounded calls in the loop). | `c060f80` moves downloads off the loop; `TestListenKeepsReadingWhileADownloadRuns` verified against the old loop in a scratch copy. |
| 7 | **The pipe hid a test failure.** The main session chained `go test ... | tail -N` to a service restart (`22:10` and again before `22:50`). The pipe took `tail`'s exit code, so a failing `TestFailedForwardPutsBackTheWaitItReplaced` did not stop the restart. The main session saw the failure on its next run (5 of 5) and said so on the record: "the pipe swallowed the failure. So the live bridge is running code with that failing test." | A later manual rerun. | A Haiku worker (`sig-test-fix`, 2m06s) root-caused it (item 8). The lesson is in section 7: `set -o pipefail` or no pipe before a restart. |
| 8 | **A worker reported green on a racy test.** `sig-poll-fix` and `sig-voice-fix` each reported `make test` and `-race` passing; the main session's run failed deterministically (5 of 5). `sig-test-fix` found a goroutine-ordering race in `handle`: when a forward fails, the message's typing tick and `cancelAwaiting` (which restores the replaced wait) run as unordered goroutines, and if the restore wins, the tick sees a 2023-timestamped wait as expired and deletes it. Adding a log line changed timing and made it pass. The worker showed the failure was environment-independent (`env -i`, `GOMAXPROCS` 1/2/4/8). | The main session's own verification run, not the worker report. | `80ed13c`: the failure path waits on a `typed` channel closed by the tick. Verified `-count=5`, `env -i`, `GOMAXPROCS=1`, and five `-race` runs. **The earlier "green" reports were not wrong about what they ran; the failure only showed in the main session's run order/timing. Treat a worker's green as a claim, not evidence.** |
| 9 | **Worker contexts grew past 100k.** Haiku input above 100k costs 5x, and harnez compacts Haiku at 80k. Reported context usage at the end of turns: `hub-006b` 115.0k then 94.2k, `sig-rich2` 107.2k then 112.2k, `sig-poll` 112.3k, `sig-poll-fix` 143.5k, `sig-voice` 151.0k, `sig-voice-fix` 145.7k. The worker log at `21:56` warns "turn re-read 504.0k cached for 10.9k new (46x)". | Worker log footers. | Compaction fired where resumed (`[compact] verified /compact at 115.0k`). The hot spots were fix rounds on an existing session. No spend figure for workers exists in the ledger (see section 6), so the real cost is not measured. |
| 10 | **A hub design reversal in review.** Review finding 2 on `4ed6dd4` asked for "turn timeouts" to be an outage; `43fc639` did that, the worker flagged that an event that always times out then waits forever, and `7103cd6` ("timeouts are content failures, not outages") reverted it. | The worker's own open question. | Resolved by a second small fix. The detour added one worker turn (1m09s). |
| 11 | **A reviewer suggestion had a side effect.** The reviewer's finding 2 on `f0f9c9a` ("default to normal with `--styled` opt-in, only consume explicit flags so `- item` passes through") is exactly what made `-h` send. Review advice is a design decision, not a free fix. | The `-h` incident (item 3). | `b83567c`. |
| 12 | **Live verification trailed the docs.** `docs/Signal.md` "Not verified live" still lists attachments and voice notes as unchecked, although the owner sent a photo and a voice note at `23:54` and both worked. | This story's cross-check of `docs/Signal.md` against the transcript. | Not fixed: the story was ordered not to touch services or the other repos (follow-up 9, section 8). |
| 13 | **Three old events sat in `failed/`.** Messages from `2026-11-13`, `11-20` and an all-clear health notice (`20261121T175040`) were set aside by the old hub. The owner said `drop` at `22:37`. | The main session, by asking. | A worker moved them to `~/.local/state/hub/dropped/` with `mv -n` (10 s). The `11-20` "are you online" was never answered after the fact. |

Notes on what the reviews could **not** prove:

- Nothing involving a real Signal client was verified by tests. Live acceptance of polls (1:1 chat renders, votes arrive), a photo and a voice note came from the owner's phone at `23:04` and `23:54`. Not yet confirmed live: the 0-based poll index, poll close, the attachment DELETE and 30-day sweep, the `lmcoder` proxy path, and a real `.m4a` from a phone (the live voice note was `.aac`).
- I could not confirm that the bridge was restarted onto `80ed13c` after the race fix. That is the main session's step and is not in the sources read.

## 5. Quality & Invariants Audit

| Area | Verdict | Evidence |
|---|---|---|
| Architecture & module separation | Stable, improving | `signal-bridge` grew from one `main.go` to `main.go`, `presence.go`, `polls.go`, `attachments.go`, `transcribe.go` (5,371 lines of Go in total, tests included). `hub` is still a single `main.go` (hub ticket `009` "split main.go into packages" remains open). |
| Idempotency | Mostly sound | Atomic state writes (`CreateTemp` + `Rename`), `flock` on `state.lock`, `O_EXCL` file creation, `mv -n` for the dropped events. **Gap:** `loadSent` ignores read errors and can silently overwrite `sent.json` (the poll worker fixed the same flaw for `polls.json`, and noted `sent.json` out of scope). |
| Backward compatibility | Slightly worse | Defaults changed twice in one day (styled to normal, `{file}` substitution kept, `signal-reply --poll` with no arguments now errors). Transcripts are now `strconv.Quote`d, which changes the line format the hub parses. No old consumers beyond the hub prompt. |
| Test coverage & verification | Good for logic, weak for reality | Last runs: `make test` `ok ... 2.157s`; `-race` five times `ok` (3.7 s); `env -i` run `ok`; the race test `-count=5` `ok`. All against fakes. See the live-verification gaps in section 4. |
| Security | Reviewed, mostly | Inbound files: `0600`/`0700`/`O_EXCL`, `LimitReader` caps (50 MB in, 25 MB per outbound file, 50 MB total, max 10 files), ID pattern, `-protocol_whitelist file` for ffmpeg, process-group kill. Hub ticket `008` (trust model) is still open. |
| Docs/ticket sync | Mixed | `docs/Signal.md` grew by 658 lines and was updated with every commit; ticket `011` closed with `harnez index`; but the "Not verified live" section is stale and `docs/Roadmap.md` row 011 still says "After 008 and 009". |

## 6. Efficiency & Velocity Assessment

**Wall-clock and flow:**

| Phase | Time | Note |
|---|---|---|
| Owner's plan request to reply | `20:31` to `20:32` | The main session answered with a 7-point plan in under a minute. |
| Go-ahead to first commits | `21:42` to `21:48` | Both first commits (`4ed6dd4`, `f0f9c9a`) at `21:48`, 5 to 6 minutes after the respawns. |
| Build window | `21:42` to `23:52` | About 2 h 10 min for five features, all fixes included. |
| Worker compute time (sum of logged turns) | about 93 min | 15 developer turns = 75 min (including the 15 min hung-test turn), 5 reviews = 18 min. Workers ran at most 2 in parallel. |
| Best case | `sig-help` | A bug fix in 1m18s, mutation-checked. |

**Counts:**

- **Commits:** 13 (hub 3, signal-bridge 10), all non-merge, all unpushed. Feature vs fix: 4 `feat` (`f0f9c9a`, `fc7a59d`, `9bafb78`, `c57ee57`) and 9 `fix` (hub `4ed6dd4` is tagged `fix:` although it is the 006 feature; the rest are review follow-ups, the `-h` fix and the race fix).
- **Lines:** hub `+1,026 / -112` (`+343/-33` production Go, `+597` tests, `+9/-2` docs). signal-bridge `+5,988 / -603` (`+2,210/-66` production Go, `+2,584/-3` tests, `+658/-0` docs, small `issues/` changes). **Tests are about as large as production code in both repos (1.2x and 1.7x).** Total gross `+7,014 / -715`. Measured with `git log --numstat` and `git diff --numstat d128900..HEAD`, not tool counters.
- **Workers:** 12 developer sessions (15 completed turns), 5 reviewer sessions, 2 failed launches, 1 refused review (cost gate), and this story writer. Reviewers: `agy:neon:med`; developers: `claude:haiku:high`.
- **Review findings:** 35 total: 10 High, 16 Medium, 9 Low (hub 5, rich 5, polls 8, attachments 8, voice 9). Addressed in code: 33 by direct commit, 1 (poll finding 2, polls in 1:1 chats) resolved by the live vote, and 2 voice findings (#4 head-of-line blocking, #6 default endpoint) answered with documentation of the trade-off rather than a change. For the rich-text review, the fix prompt listed all five findings and `85de201` is the answer; I did not verify items 4 and 5 individually.
- **Owner physical/decision load:** see section 8.1.
- **Cost:** only the **main session** is in `ledger.jsonl`: 39 main turns today, `$4.94` (events 41), `$10.25` across all 83 recorded turns since `2026-11-12`. The main session said "about $3.40" at `22:37`, consistent with the ledger at that time. **Worker cost is not recorded in dollars** (worker logs list tokens only, such as `session 107.3k new, 458.9k cached` for a review, and `5.4M cached` for the `sig-attach` developer). Whether the cheap developers were cheap in absolute terms is therefore unmeasured. Claude weekly quota was 9% at `20:23` and 10% at `22:37`.

**Efficiency verdicts:**

- Review ratio: roughly one review minute per 5 developer minutes caught 10 High findings. That is the best return of the day.
- The expensive failure modes were not model quality but *plumbing*: path default, the cost gate, hung tests, a restart chained to a pipe.
- Haiku-high handled narrow tasks well and long fix rounds poorly (context at 143k to 151k at the end).

## 7. Key Learnings & Evergreen Upstream

Candidates to promote (none are applied by this story):

1. **Never chain a test run into a restart through a pipe.** Use `set -o pipefail` or `&&` without `| tail`, or run `go test` without a pipe and print the exit code. In a headless session a swallowed failure goes live.
2. **A worker's "green" is a claim.** The main session should re-run the target tests itself in its own environment before restarting a service, and treat flaky timing as a race until disproved. Ask workers for `-count=5` and `env -i` runs.
3. **Hard no-live-call rules in every worker prompt**, and `TestMain` that redirects config and state to a temp dir so that a test cannot reach the live service. Add the rule that workers never run the real notification binary (`signal-reply`) even to check help text.
4. **Second-vendor review for every feature that touches I/O, concurrency or state.** Cheap review (`agy:neon:med`) beat the expensive tier on cost and was good enough. Log the cost-gate refusal; hubs cannot pass `--accept-cost`.
5. **Read the review, then decide.** Two of the day's three behaviour reversals (`-h`, timeouts) came from applying review advice without weighing its side effect. Tell the fix worker "list any finding you disagree with and why".
6. **Keep fix rounds on a fresh worker, not a resumed one**, once context usage is over 80k. The log suggested exactly that (`fresh start ~115.0k`).
7. **Binary paths are config, not defaults.** A default of `~/go/bin/harnez` broke when harnez moved. Resolve through `PATH` first; the hub now does.
8. **Keep docs honest after live checks.** The photo and voice note passed, but `docs/Signal.md` still lists them as unverified.
9. **A story needs a cost column for workers.** `ledger.jsonl` records only the main session. Without worker dollars, "cheap developer" is a belief.

### Which tech choices helped

| Choice | Why it helped |
|---|---|
| **Go single binaries + `make install`** | Each worker's last step was `make install`; the main session verified `git log -1`, `go test`, and `ls -l ~/.local/bin/signal-reply`. No dependency drift, no venv, instant rollback by re-install. `signal-reply` is a symlink to the one binary. |
| **systemd user services** | `hub.service` and `signal-bridge.service` restart in seconds; the main session scheduled the hub restart as a one-off timer (`hub-restart-006`) so it would not kill its own turn. `journalctl --user` gave the failure evidence for the diagnosis worker. |
| **`signal-cli-rest-api` with its swagger as source of truth** | Workers and reviewers could check each call against the live `swagger/doc.json`, which made hallucinated fields visible. Where swagger was wrong (`required` lists every field, response shape for `/v2/send`) the reviewers caught it from the upstream source. |
| **`httptest` fakes + "no live calls"** | Features were exercised without ever messaging the owner, except in the one incident where a worker ran the real binary. |
| **Cheap developer + second-vendor reviewer** | Section 3 and 6. |
| **One feature per worker** | Easy to review, easy to revert, easy to resume. |
| **Local `voxi` instead of cloud ASR** | Sub-second transcript, no key, no audio leaving the PC, no per-minute fee. Voxi's own spec showed what to guard (`max_chars: 2000`, 30 s default timeout, cold start about 25 s). The cost: the bridge now depends on a local GPU/CPU service and ffmpeg. |

### Which choices caused or would have caused slowdown

| Choice | Effect |
|---|---|
| **OAuth expiry and quota dropping events** (hub before 006) | Lost the owner's `are you online` on `11-20`, a `run harnez usage` on `11-13`, and a health all-clear on `11-21 18:50`. This is the reason `006` was top of the list. The hold is now built, but an expired login holds forever until the owner runs `claude login` at the PC (the notice says so). |
| **Harnez path change** | Two workers dead at launch (item 1). |
| **Cost gate blocking `agy:neon:high`** | One refused review; reviewers moved down a tier (item 2). |
| **Contexts over 100k on Haiku** | 5x input pricing above 100k, compaction at 80k, `46x` cached re-read warnings (item 9). |
| **Synchronous work in the receive loop** | Caught twice in review (`f0f9c9a`, `9bafb78`). Ticket `002` remains. |
| **`harnez` quota files** | Claude quota readings were stale (`fable unknown, last reading 44h50m old`) in the logs; quota is a guess for some tiers. |

### Alternatives that would have hurt

- **Python.** The workers would have needed a venv, a service wrapper and a packaging story. The "one binary, `make install`, systemd" loop would be gone, and the type checker would not catch half the mistakes that `go vet` and `-race` did. The race and the keepalive starvation would have appeared as runtime hangs.
- **Cloud ASR** (a hosted speech API). It needs a key, uploads the owner's voice, adds a per-minute fee and a second outage source on top of Claude's. It would also have given the same hub-hold problem for a second provider. The local `voxi` call was one HTTP POST on `127.0.0.1:18131`.
- **Signal groups** for polls and decisions. Neon's review warned that Signal clients might support polls only in groups; the live vote showed 1:1 chats work. The group alternative would need group membership policy (ticket `007`) and an identity-key story for every member (ticket `014`). It would also have cut the bridge's "drop `groupInfo` frames" simplicity.
- **A Matrix bridge** instead of Signal. A second server, a bridge process and a new trust model, to do what one Docker container and one Go binary already do. The owner's phone already has Signal.
- **Resuming the same Haiku worker for every round.** Context 143k to 151k at the end of fix rounds shows where that leads.

## 8. File & Diff Summary

### 8.1 How much the owner had to do

From the main-session transcript, the `[signal]` events of `2026-11-21` (local time):

| Time | What | Kind |
|---|---|---|
| 17:19 | "why did you miss my call?" | Message |
| ~18:45 | Rebooted Esprimo for the kernel update (health all-clear at `18:50`) | **Physical** |
| 20:23 | "rebooted give me a status report" | Message |
| 20:31 | "Make a plan to improve my Signal experience..." | Message (request) |
| 21:42 | "go for all of them ... Haiku high ... Neon to review" | Message (**the one big decision**) |
| 22:00 | "you just sent me -h" | Message (bug report) |
| 22:06 | "what is the current tech stack..." | Message (question) |
| 22:36 | "status report plz" | Message |
| 22:37 | "drop" | Message (decision: drop 3 stale events) |
| 23:04 | Voted in the poll "What should I build next?" | **Physical**, decision (images) |
| 23:54 | Sent a test photo (btop screenshot) | **Physical** |
| 23:54 | Sent a test voice note | **Physical** |
| 23:55-23:57 | Three questions: harnez skills, cwd, find `/story` | Messages |
| 23:59 | The story request | Message (request) |

Count: **12 typed messages, 1 poll vote, 1 photo, 1 voice note (15 inbound events), 1 reboot.** Decisions: **4 real ones** (the go-ahead with models and review rule; `drop`; the poll choice; the story request). He did not type code, open a terminal, restart a service, read a log or approve a diff. Two of his messages were bug reports or tests that only he could do (`-h`, the photo and voice note).

Unanswered decisions the main session asked for at `15:02`, `19:23` and `22:54`: switch `voxi` back to its primary model, make `voxi` free memory when idle, the verdict on epic `#277`, hub `006` (answered implicitly by "go for all of them"), and "Should I push signal-bridge and hub to your remote?". These are still open.

### 8.2 Machine care

- `15:00`: health alert: kernel `7.3.4` replaced by an update (reboot needed), swap `9.6 of 14.9 GiB` used, and "Snapshots: needs root to read (snapper)".
- The main session told the owner and attributed the swap to the idle `voxi` model server being pushed out to swap. After the reboot (`19:23` status): kernel `7.3.5`, RAM `3.5 of 14 GiB`, swap empty, disk 6%, no failed units, only the snapper check still failing because it needs root.
- The **snapper question** (the check cannot read snapshots without root) is unanswered; the stack answer at `22:06` says btrfs with snapper.
- The `voxi` restart was part of the reboot recovery; the `voxi-r2t2` llama-server holds `2.8 GB` per the owner's own btop photo. The permanent fix (free memory when idle) is not done.

### 8.3 Commits

hub (`~/projects/hub`):

| Commit | Time | Subject |
|---|---|---|
| `4ed6dd4` | 21:48 | fix: hold the queue on outages; add hub failed and hub retry (+571/-23) |
| `43fc639` | 21:55 | fix: outage review follow-ups (+381/-73) |
| `7103cd6` | 21:57 | fix: timeouts are content failures, not outages (+74/-16) |

signal-bridge (`~/projects/signal-bridge`):

| Commit | Time | Subject |
|---|---|---|
| `f0f9c9a` | 21:48 | feat: styled replies, read ack, reactions as text, typing while waiting (+739/-41) |
| `85de201` | 21:59 | fix: keep the receive loop free, plain replies by default, lock shared state (+582/-135) |
| `b83567c` | 22:09 | fix: signal-reply -h and --help print usage instead of sending (+66/-1) |
| `fc7a59d` | 22:41 | feat: polls for owner decisions (+681/-10) |
| `ed7c1c4` | 23:10 | fix: poll review findings; close 011 (+650/-125) |
| `9bafb78` | 23:18 | feat: attachments in both directions (+1,113/-55) |
| `c060f80` | 23:33 | fix: attachment review findings on 9bafb78 (+839/-128) |
| `c57ee57` | 23:39 | feat: transcribe voice notes before forwarding (+692/-9) |
| `54eb7ce` | 23:50 | fix: voice note review findings on c57ee57 (+618/-99) |
| `80ed13c` | 23:52 | fix: undo of a failed forward waits for its typing tick (+8/-0) |

Key files created: signal-bridge `presence.go`, `polls.go`, `attachments.go`, `transcribe.go` and their `_test.go` files; hub `outage_test.go`. Largest edits: `docs/Signal.md` (signal-bridge), `main.go` in both repos.

Tickets: hub `006` closed by its commit (the commit is tagged `fix:`; I read the file rename in the stat only); signal-bridge `011` closed (`ed7c1c4`); signal-bridge `002` (slow forward blocks read loop) is partly addressed by `85de201`/`c060f80` but not closed; `009` and `010` are only partly implemented (attachments, voice notes, reactions, read ack, typing); I did not check their status files.

### 8.4 Open items

1. **Push not done.** All 13 commits sit on local `main` in `hub` and `signal-bridge`. The owner was asked at `22:54` and has not answered.
2. **`voxi` is still on its fallback model**; "back to primary" awaits the owner.
3. **Idle memory:** `voxi` should free memory when idle, otherwise swap fills again. The llama-server holds `2.8 GB`.
4. **The stale-wait race for two failed forwards.** `80ed13c` fixes one ordering; the `sig-test-fix` report says two failed forwards can still undo out of order (message C starts while B's undo is pending; B's undo restores a stale wait). Needs per-message serialised undo.
5. **`sent.json` silent overwrite** (`loadSent` ignores read errors), the same flaw fixed for `polls.json`.
6. **Hub ticket `003`/`004`/`005`** (transient main failure discards session, hub restart kills workers, timeouts do not stop process trees) are not touched by this day's work.
7. **Auth outage holds forever** until the owner runs `claude login` at the PC. No second channel exists (bridge ticket `013`, dead Signal link, is open).
8. **Machine:** the snapper health check needs root; epic `#277` verdict pending.
9. **Docs:** `docs/Signal.md` "Not verified live" and `docs/Roadmap.md` row `011` are stale.
10. **Live checks missing:** poll index base, poll close, attachment DELETE and the 30-day sweep, a real phone `.m4a`, the `8735` lmcoder path, and whether the bridge runs the `80ed13c` build.

### 8.5 Follow-Up Tickets

No tickets were filed. The task was a read-only story with one committed file; filing needs `harnez issues new` in `hub` and `signal-bridge`, which would add commits there. Recommended tickets:

| Repo | Proposed ticket |
|---|---|
| signal-bridge | Serialise undo of failed forwards per message (open item 4) |
| signal-bridge | `loadSent` must return errors and never overwrite `sent.json` (open item 5) |
| signal-bridge | Update `docs/Signal.md` "Not verified live" after the owner's photo and voice-note test; fix Roadmap row `011` |
| signal-bridge | Live-verify poll index base, poll close, attachment DELETE and sweep, `.m4a` conversion |
| hub | A second channel for the "login expired, run `claude login`" notice |
| hub | Let the hub pass `--accept-cost`, or document why reviewers must use `neon:med` |
| harnez | Record worker dollars in a ledger; warn when a resumed Haiku session is above 100k |
| harnez | Candidate evergreen rule: no pipes before a restart; worker "green" is a claim (section 7, items 1 and 2) |
| owner | Decide: push; `voxi` primary model; idle memory; epic `#277`; snapper |

Ticket Extraction Gate: not run with allocated workers, because no tickets were allocated. The list above is the open set.

## Review Note

**Second-vendor review: not done.** The `/story` skill asks for a read-only `agy:neon:med` review. `harnez agent start --role reviewer --model agy:neon:med` is available (`harnez agent models` lists the model as available), but this story was written by a leaf `developer` worker (session `story-1121`), and harnez refuses it: `agent role "developer" is a leaf worker: it must not start, resume or manage agents`. So this draft was reviewed by its writer only. What was re-checked by the writer against sources: commit hashes, times and per-commit line counts (from `git log --stat`), gross LOC by file type (from `git diff --numstat`), the five review finding counts (5, 5, 8, 8, 9 = 35; severities counted from the logs), worker turn durations (summed from the log footers), ledger totals (`$4.94`, 39 turns on 11-21), and the owner's 15 events (from the transcript). Not independently checked: the main-session transcript claims about restarts and the hub-restart timer, and the exact ~18:45 reboot time (inferred from the health all-clear at 18:50). The host should run the review (`harnez agent start --role reviewer --model agy:neon:med`) and fold in fixes.
