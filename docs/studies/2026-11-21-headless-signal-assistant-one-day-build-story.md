---
kind: story
---

# Story: one day of a headless Signal assistant building itself (2026-11-21)

A one-day, multi-repo story. The code lives in `~/projects/hub` (event hub) and `~/projects/signal-bridge`
(Signal bridge), not in harnez. The work was driven by a headless main session whose cwd is `~/projects`,
which is not a repository. The story is filed here because harnez supplied the harness (`harnez agent`, model
tiers, cost gate, quota reads) and owns the `/story` skill, and because the owner said "put it in harnez".

Previous story: [2026-11-19 session isolation, forge parity and the quota pack](2026-11-19-session-isolation-forge-parity-and-quota-pack-story.md).
That story covers harnez development, so there is no harnez commit range here. Baselines: hub `7fbbaaf`,
signal-bridge `d128900` (the parents of the day's first commits).

**How this story was made:** a `claude:sonnet:med` worker wrote a draft (harnez `c30899d2`). A
`claude:haiku:high` scout collected cited facts (`docs/studies/.story-1121-facts.md`, left untracked). A
`claude:opus:high` writer (this version) merged the two, re-checked every kept claim against the sources below,
and corrected both inputs where they were wrong (see "Corrections"). An `agy:neon:med` review then checked
it: minor fixes, 7 of 7 findings applied (see the Review Note).

**Sources.** S1 = `git log`/`git diff --numstat` in hub and signal-bridge. S2 = `~/.local/state/hub/logs/*20261121*`
(23 worker logs). S3 = `~/.local/state/hub/ledger.jsonl`. S4 = main-session transcript
`~/.claude/projects/-home-alfred-projects/f4330fa8-….jsonl` (grep/jq only). S5 = `systemctl --user show`, file
mtimes, `harnez agent models`. Times are local (+01:00) unless marked `Z`.

## 1. Header & Context

- **Window:** first health event `15:00`, first owner message `17:19`, go-ahead `21:42`, last fix commit `23:52`,
  story ordered `23:59`. Post-midnight wrap-up and owner feedback (`00:00` to `00:28`, 11-22) are covered briefly.
- **Cast:**
  - *Owner*: uses Signal on the phone only.
  - *Main session*: `claude:opus:high` (`claude-opus-5-5`, `effortLevel: high` in `~/.claude/settings.json`; 161
    assistant messages on 11-21, S4). The hub wakes it for each Signal, worker or health event. It acts through
    `signal-reply` (25 direct sends on 11-21, S4) and `hub spawn`.
  - *Workers*: `claude:haiku:high` developers, one `claude:haiku:low` chore worker, and `agy:neon:med` reviewers
    (`gemini-4-neon` in the log headers, S2).
- **Starting state:**
  - The hub set events aside after three failed main turns and had no way to send them again. Lost this way:
    "are you online" on 11-20 21:50 (expired Claude OAuth), "run harnez usage" on 11-13 (weekly quota), and the
    11-21 18:50 health all-clear (5-hour limit: 3 ledger errors at 18:50 to 18:53, S3). Hub ticket `006` was open.
  - The bridge sent plain text only and ignored attachments, reactions and polls.
  - Machine (Esprimo, CachyOS) at the 15:00 health alert: kernel `7.3.4-1` replaced by an update, swap at
    9.6 of 14.9 GiB (S4).
- **Goal, verbatim (S4):**
  - 20:31: "Make a plan to improve my Signal experience. Can you send rich text? Can you send CTAs? Can you send
    a survey?"
  - 21:42: "I would say go for all of them pick the order yourself you can go into decide mode if you need to go
    that way. and you ist die Haiku high Model as a developer. / when you implement bigger features, ask neon
    model to review the code and the feature and the assumptions"

## 2. Executive Summary

In 2 h 10 min of building (21:42 to 23:52), the main session delivered the following through 13 worker turns
that produced commits, plus 5 reviews:

- **hub 006**: hold the queue on outages, `hub failed`, `hub retry` (`4ed6dd4`, `43fc639`, `7103cd6`).
  Ticket closed.
- **signal-bridge**:
  - styled text (opt-in), 👀 read ack, typing indicator, reactions forwarded as text (`f0f9c9a`, `85de201`)
  - `-h`/`--help` fix (`b83567c`)
  - polls (`fc7a59d`, `ed7c1c4`; ticket `011` closed)
  - attachments in both directions (`9bafb78`, `c060f80`)
  - voice-note transcription through the local voxi server (`c57ee57`, `54eb7ce`)
  - a race fix (`80ed13c`)
- **Machine care**: the owner rebooted onto kernel 7.3.5, which emptied swap. Three stale failed events were
  archived on the owner's "drop".

The owner checked the poll, the photo and the voice note live from the phone (23:04, 23:54). Hub was pushed at
00:15 and signal-bridge at 00:17 on 11-22, after the owner wrote "commit and push" (`origin/main` reflogs).

**Owner effort on 11-21:** 15 inbound events (12 typed messages, 1 poll vote, 1 photo, 1 voice note) and 1
reboot. 4 decisions. The owner wrote no code, opened no terminal and approved no diff.

**Headline:** it went well because the tasks were small, every feature got a review from a second vendor (35
findings, 10 High), and tests ran against fakes. It was not clean:
- two launches failed on a moved binary path
- a worker sent `-h` to the owner's phone
- a hung test cost about 15 minutes
- the main session restarted the bridge on top of a failing test hidden by a pipe
- a worker reported green on a test that failed 5 of 5 a minute later

## 3. What Worked Well

- **One feature per worker, one commit per turn.** 10 worker sessions made the 13 commits. Each report gave the
  hash, the changes per item, the test output and open questions (S2). The main session could check each one with
  `git log -1`, `git status` and one `go test`.
- **Cheap developer, reviewer from a second vendor.** `claude:haiku:high` (relative cost 7) wrote the code and
  `agy:neon:med` (109) reviewed it (S5 `COST` column, `codex:luna:low` = 1). Five reviews took 1m30s to 6m08s each,
  about 18 minutes in total, and found 35 issues. 10 of them were High, including:
  - receive-loop blocking (twice)
  - a root-owned inbox path
  - poll votes that did not restart typing
  - no audio or transcript caps
  - a shutdown that falsely notified "back"
- **The review prompt fed the fix worker verbatim.** Fix workers read the review log from S2 instead of a
  paraphrase. Fix commits followed their reviews within 4 to 24 minutes; the 24-minute gap for polls includes the
  owner's live vote.
- **Mutation checks made reports trustworthy.**
  - `sig-help`: disabled the early return and saw `TestReplyHelpSendsNothing` fail.
  - `sig-attach-fix`: ran its listen test against the old loop in a scratch copy, where it failed.
  - `sig-voice-fix`: removed the process-group kill and saw the test fail (S2).
- **Polls as the decision interface.** "What should I build next?" was a real poll. The owner's tap at 23:04
  picked the next feature and settled review finding poll#2 (do 1:1 chats render polls?).
- **Event-driven orchestration.** A worker's result arrives as a hub event that wakes the main session, so the
  main session never polls. The hub restart was scheduled as a one-off timer
  (`systemd-run --on-active=90 --unit=hub-restart-006`, S4), so it did not kill the turn that ordered it. hub became
  active at 22:10:51 (S5).

### Tech choices that helped

| Choice | Why it helped (evidence) |
|---|---|
| Go single binaries, `make install`, systemd user services | No venv or packaging. Workers end with `make install`; a restart takes seconds. The bridge binary (`~/go/bin/signal-bridge`, mtime 23:52:41) predates the service start (23:53:06), so the live bridge runs the race fix (S5). |
| `signal-cli-rest-api` with its Swagger document as the contract | Reviewers checked shapes against Swagger and caught gaps it does not cover (`flag.Parse` and `-`-leading replies, poll index base). The bridge docs keep a "Not verified live" list for the rest. |
| `httptest` fakes with isolated state (`t.Setenv` for `XDG_STATE_HOME`, `SIGNAL_BRIDGE_CONFIG`) | Features were exercised without messaging the owner. After the `-h` incident, worker reports say nothing was run live (for example `sig-voice-fix` log line 31). |
| Local voxi for speech | No API key, no upload, no per-minute fee. The owner's voice note came back as "This is a short voice message." (S4). The main session's "under a second" is its own claim and was not measured. A second voice note at 00:16 on 11-22 shows the path in real use. |

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

| # | What happened | How caught | Fix / state |
|---|---|---|---|
| 1 | **harnez moved.** The hub's default binary was `~/go/bin/harnez`; harnez now installs to `~/.local/bin`. `hub-006` and `sig-rich` failed at launch with `fork/exec … no such file or directory` (0-byte logs). | Failure events within a minute (S4 20:43:16Z). | A symlink `~/go/bin/harnez → ~/.local/bin/harnez` (21:43, still present). `4ed6dd4` resolves harnez via `PATH`, falling back to `~/.local/bin`. Relaunched as `hub-006b` and `sig-rich2`; about 1 minute lost. |
| 2 | **Cost gate refused `agy:neon:high`** (relative cost 219): `pass --accept-cost to proceed` (`sig-rich-review` log). The hub cannot pass that flag. | 130-byte log. | The main session moved all reviews to `agy:neon:med` (109). The owner had said "neon model" with no tier, so the main session chose the tier. |
| 3 | **`-h` sent to the owner.** Review finding rich#2 asked to "default to `normal` … and only consume explicit flags" so that replies starting with `-` send. `85de201` replaced `flag.NewFlagSet` with a loop over `--plain`/`--styled` and dropped the `flag.ErrHelp` path: an implementation omission, not bad advice. `sig-rich2` then ran `signal-reply -h` against the real binary, and it sent "-h" (main-session apology, 21:00:22Z). | The owner: "you just sent me "-h"" (22:00). | `b83567c` at 22:09 (`-h`/`--help` print usage, mutation-checked). Every later prompt banned running `signal-reply`. It was the day's only bug the owner saw. Still open: `-hi` is sent as text. |
| 4 | **Hung test.** The first `make test`/`-race` in the `sig-rich2` fix turn hung on `TestSendSignalTimesOut` until "stopped at its time limit" (log line 7). The turn took 15m29s. | The worker's report. | Test-server bug fixed and rerun in the foreground. About 10 minutes of pure wait. |
| 5 | **Root-owned inbox default** `/signal-bridge/inbox`. On this single-user host, every inbound file would have failed. | att#1 (High) and the worker's open question. | `c060f80` falls back to the state dir when that path is not writable. The photo landed in `~/.local/state/signal-bridge/inbox/`. The constant is still tried first: ticket `017`. |
| 6 | **Blocking work in the receive loop**: synchronous sends (rich#1) and downloads (att#2) starve the WebSocket keepalive. | Review, twice. | `85de201` and `c060f80`. `TestListenKeepsReadingWhileADownloadRuns` fails against the old loop. Ticket `002` stays open. |
| 7 | **A pipe hid a failing test before a restart.** At 23:50:27 the main session ran `go test … 2>&1\|tail -1 && systemctl --user restart signal-bridge`; the result was `FAIL active active`. `tail` exited 0, so the restart ran. The same pattern appears at 23:10:34. | The main session read its own output, reran 5 times (`FAIL` ×5), and said "the pipe swallowed the failure". | `sig-test-fix` was spawned 8 s later. Before the fix the owner was told "One test is still failing … it doesn't affect normal use". That impact claim was not verified at the time. |
| 8 | **Green report on a failing test.** `sig-voice-fix` added `TestFailedForwardPutsBackTheWaitItReplaced` in `54eb7ce` and reported `make test` ok and `-race` ok. One minute later the main session got 5 of 5 failures. `sig-test-fix` found a goroutine-ordering race in `handle`: when a forward fails, the cancel goroutine runs before the typing tick, which then deletes the restored wait. "Adding a log line changed the timing and the test passed." It failed the same way under `env -i` and at `GOMAXPROCS` 1, 2, 4 and 8. | The main session's own run, not the worker's report. | `80ed13c` (+8 lines: a `typed` channel; the undo waits for the tick). `-count=5`, `env -i`, `GOMAXPROCS=1` and five `-race` runs all pass. Why the worker saw green was not found. Ticket `015`: two failed forwards in a row can still undo out of order. |
| 9 | **Haiku context above 100k.** Haiku 5.5 input above 100k costs 5x, and harnez compacts Haiku at 80k (`docs/AutoCompaction.md:94`, `spec/agent.yaml`). End-of-turn context usage (S2): `sig-voice` 151.0k, `sig-voice-fix` 145.7k, `sig-poll-fix` 143.5k, `hub-006b` 115.0k, `sig-poll` 112.3k, `sig-rich2` 112.2k. One check line warned: "turn re-read 504.0k cached for 10.9k new (46x)". | Log footers. | None yet. Fresh workers (`sig-voice`, `sig-poll-fix`) reached the same size as resumed ones, so task size drives context, not session reuse. |
| 10 | **Timeout reversal in hub.** hub#2 listed "turn timeouts" among missing outage patterns. `43fc639` added them, and the worker warned that an event that always times out would then wait forever. `7103cd6` reverted it: "timeouts are content failures". | The worker's open question. | One extra turn (1m09s). |
| 11 | **Story worker on the wrong model.** The main session picked `claude:sonnet:med` for this story against the owner's Haiku-high rule ("I didn't check Sonnet's cost"). The hub has no stop command, so the worker could not be cancelled. | The owner: "why sonnet?" (00:00, 11-22). | The plan was reset to a scout, a `claude:opus:high` writer and a review. The owner said "keep the output as input" and added the rule "when you name models always name their effort". |
| 12 | **Docs trail the live checks.** In `docs/Signal.md`, "Not verified live" still says attachments and voice notes are unchecked, after the 23:54 photo and voice note. | This story's cross-check. | Open. The wrap-up commit `13075da` fixed only the Roadmap row. |

What the tests could not prove: anything involving a real Signal client. Not yet checked live:
- the 0-based poll index, and whether a closed poll shows as closed on the phone (`signal-reply --close-poll` ran
  live at 23:10 and returned `rc=0`, S4)
- the attachment `DELETE` and the 30-day sweep
- a real `.m4a` (the live note was `.aac`)
- the lmcoder `8735` path

### Choices that caused, or would have caused, slowdown

| Choice | Effect |
|---|---|
| A hard-coded default binary path in the hub | 2 dead launches (item 1). The `signal-reply` symlink still points into `~/go/bin`, so install dirs remain mixed. |
| The cost gate, with no `--accept-cost` path through `hub spawn` | 1 refused review; the tier was chosen silently (item 2). |
| No `hub stop` | The story worker could not be cancelled (item 11). |
| Main session in a non-repo cwd | Project skills (`/story`) do not apply there; the story had to be routed into a repo by hand. |
| Chaining `\| tail` into a restart | A failing build went live (item 7). |
| Applying review advice without weighing side effects | The timeout reversal (item 10): hub#2 asked for timeouts as outages, and `7103cd6` undid it 2 minutes later. |
| Large Haiku turns | Work billed in the 5x band (item 9). |
| Auth outage design | hub#5 (Low): an expired login holds the queue forever until someone runs `claude login` at the PC. Partly fixed in `43fc639` (a notice and a `hub status` hint); the hold itself stays, and the notice goes over Signal only, with no second channel (bridge ticket `013`). |

**Alternatives not taken** (judgement, not measured):
- `agy:neon:high` with `--accept-cost`: twice the relative cost of `med`, and `med` already found 35 issues.
- Python: a venv and service wrapper per tool, and no `-race`.
- Cloud speech recognition: an API key, the owner's voice uploaded, a per-minute fee, and a second outage source
  for the hub to hold.
- Signal groups for polls: group-membership policy (bridge ticket `007`) for a need that 1:1 chats met.
- A Matrix bridge: a second server and trust model for a phone that already has Signal.

## 5. Quality & Invariants Audit

| Area | Verdict | Evidence |
|---|---|---|
| Architecture & modules | Improving (bridge), stable (hub) | The bridge split into `presence.go`, `polls.go`, `attachments.go`, `transcribe.go` (5,371 Go lines incl. tests). The hub is still one `main.go` (1,835 Go lines incl. tests). |
| Idempotency | Mostly sound | Atomic state writes, `O_EXCL` files, `mv -n` for dropped events. Gap: `loadSent` ignores read errors and can overwrite `sent.json` (ticket `016`; the same flaw was fixed for `polls.json`). |
| Backward compatibility | Changed twice in one day | Styled became opt-in; `--poll` without arguments now errors; transcripts are quoted. The only consumer is the hub prompt. |
| Tests & verification | Good for logic, thin on reality | Final worker runs were green, including five `-race` passes on `80ed13c`. All tests run against fakes. Item 8 shows a worker's green is a claim. |
| Security | Reviewed | `0600`/`0700`/`O_EXCL` files, size and count caps, `-protocol_whitelist file` for ffmpeg, process-group kill on timeout. Hub trust-model ticket `008` is still open. |
| Docs/ticket sync | Mixed | `docs/Signal.md` +658 lines, kept up to date with each commit except "Not verified live" (item 12). Tickets 006 and 011 closed; bridge 015 to 017 and hub 025 filed after midnight. |

## 6. Efficiency & Velocity Assessment

**Commits (S1).** 13 on 11-21: hub 3, signal-bridge 10, of which 4 `feat` and 9 `fix` (hub `4ed6dd4` is the 006
feature but is tagged `fix:`). After midnight: hub `8698ceb` (ticket 025), signal-bridge `13075da` (Roadmap
sync, tickets 015 to 017), harnez `c30899d2` (draft) and this rewrite.

**Lines (S1, `git diff --numstat <base>..<last>`):**

| Repo | Net | Production Go | Tests | Docs | issues/ |
|---|---|---|---|---|---|
| hub | +949 / -35 | +343 / -33 | +597 / -0 | +8 / -1 | +1 / -1 |
| signal-bridge | +5,456 / -71 | +2,210 / -66 | +2,584 / -3 | +658 / -0 | +4 / -2 |

Tests outweigh production code: 1.7x in hub and 1.2x in the bridge. The gross per-commit sum is +7,014 / -715,
because fix commits rewrite the lines their feature commits added.

**Workers (S2, 23 logs):**
- 15 developer turns: 11 `claude:haiku:high` sessions and 1 `claude:haiku:low` (`drop-failed`, 10 s)
- 6 review runs: 5 `agy:neon:med` and 1 `agy:neon:high` refused by the cost gate
- 2 failed launches

At most two workers ran at once. Story workers (11-22): `claude:sonnet:med` draft, `claude:haiku:high` scout
(dispatched twice, 00:03, S4), `claude:opus:high` writer, `agy:neon:med` review pending.

**Wall clock.** Plan request to plan: under 1 minute (20:31 to 20:32). Go-ahead to the first two commits: 6
minutes. Building took 2 h 10 min. Summed log durations: about 75 minutes of developer turns (15m29s of it the hung
test) and about 18 minutes of reviews. Fastest: `sig-help`, a mutation-checked fix in 1m18s.

**Review findings: 35** (10 High, 16 Medium, 9 Low). By review: hub 5, rich 5, polls 8, attachments 8, voice 9.

| State | Count | Which |
|---|---|---|
| Fixed (claimed by the fix worker; some verified) | 31 | hub 1 to 4 (#4 verified in code: `signalSend` timeout); rich 1 to 5 (#4 array-or-object `/v2/send` and #5 ACI author check, typing after a successful `PUT`, verified in the `85de201` message and diff); polls 1 and 3 to 8; attachments 1 to 8 (#1 only partly, ticket `017`); voice 1 to 3 and 5 to 9 |
| Partly fixed | 1 | hub#5: `43fc639` adds a kind-change notice and `hub status` says an auth outage needs `claude login`; the hourly hold until a human logs in is kept |
| Resolved live | 1 | poll#2: 1:1 chats show polls (owner's vote) |
| Kept by choice, documented | 2 | voice#4 (FIFO head-of-line); voice#6 (default endpoint `18131` vs the lmcoder proxy `8735`) |

**Cost.**
- Main session only, from S3:
  - 11-21: $4.94 over 39 turns and 41 events; the main session reported "about $3.40" at 22:37.
  - Through 11-21: $10.25 over 83 turns since 11-12.
  - Post-midnight: $1.12 over 17 turns (00:01 to 00:16). Through 00:28: $1.84 over 30 turns. The ledger was still
    growing at the time of writing.
- The ledger has no model or token fields and **no worker dollars**. Worker cost is therefore unmeasured, and
  "cheap developer" rests on the relative cost table (haiku:high 7, neon:med 109), not on spend.
- Claude weekly quota: 9% at 20:23, 10% (and 17% of the 5-hour window) at 22:37.

**Verdicts.**
- The reviews were the best return of the day: roughly 1 review minute per 4 developer minutes, and 10 High
  findings.
- The costly failures had two causes. Plumbing: the path default, the cost gate, the missing `hub stop`, a hung
  test. Agent discipline:
  - `sig-rich2` ran the real `signal-reply -h`.
  - `sig-voice-fix` reported green on a test that failed 5 of 5.
  - The main session restarted the bridge behind a pipe, dispatched `claude:sonnet:med` against the owner's
    Haiku-high rule, and sent the owner claims it had not checked: "it doesn't affect normal use" and "under a
    second".
- `claude:haiku:high` handled narrow tasks well but often ran into the 5x context band.

## 7. Key Learnings & Evergreen Upstream

Candidates (none applied by this story):

1. **No pipe between a test run and a restart.** Use `set -o pipefail`, or run `go test` unpiped and gate on its
   exit code. In a headless session a swallowed failure goes live.
2. **A worker's green is a claim.** The host re-runs the target tests before any restart. For new concurrency
   tests, ask for `-count=5`. `-race` does not explain item 8: `sig-voice-fix` reported both `make test` (plain
   `go test ./...`, 2.162s, not cached) and `-race` green, but the host got 5 of 5 failures without `-race`.
3. **Workers never run notification binaries** (`signal-reply`), not even for `-h`. State it in every prompt; keep
   state isolated in tests.
4. **Review every I/O, concurrency or state feature with a second vendor.** `neon:med` was enough. Ask the fix
   worker to name any finding it disagrees with before applying it.
5. **Size Haiku tasks to stay under 100k context**: split features rather than relying on fresh sessions (item 9).
6. **Binary paths are resolved via `PATH`**, never a fixed default (item 1).
7. **Update the live-verification docs after every owner test.**
8. **Record worker spend.** Without it, a story cannot price the cheap-developer pattern.
9. **The host names model and effort** (`claude:haiku:high`) and checks the owner's model rules before
   dispatching. Owner rule, 00:03 on 11-22.

## 8. File & Diff Summary

### 8.1 Owner effort, 11-21 (S4)

| Time | Event | Kind |
|---|---|---|
| 17:19 | "why did you miss my call?" | message (the bridge has no call support) |
| ~18:45 | Rebooted Esprimo | **physical**; inferred from "up 1h38m" at 20:23 |
| 20:23 | "rebooted give me a status report" | message |
| 20:31 | Plan request (quoted in §1) | message, request |
| 21:42 | "go for all of them … Haiku high … ask neon model to review" | message, **decision** |
| 22:00 | "you just sent me "-h"" | message, bug report |
| 22:06 | "what is the current tech stack …" | message |
| 22:36 | "status report plz" | message |
| 22:37 | "drop" (3 stale failed events) | message, **decision** |
| 23:04 | Poll vote: "Send images and charts" | **physical**, **decision** |
| 23:54 | Photo (btop on Esprimo) | **physical**, live test |
| 23:54 | Voice note | **physical**, live test |
| 23:55-23:57 | Skills, cwd, "find the /story skill" | 3 messages |
| 23:59 | "put it in harnez put write a story on all we did today" | message, **decision** |

Totals:
- 12 typed messages, 1 vote, 1 photo and 1 voice note, which is 15 events.
- 4 physical actions: the reboot, the vote, the photo and the voice note.
- 4 decisions: the go-ahead (including the developer model and the review rule), `drop`, the poll choice and the
  story.

From 00:00 to 00:17 on 11-22 the owner sent 13 more events (12 typed messages and 1 voice note), mostly
instructions:
- "keep the output as input"
- the effort-naming rule
- "you could kill the process in rare cases"
- "200k is the limit for all" (autocompact)
- "restart yourself when possible"
- hub `/issue` (ticket 025)
- `/wrap`
- "commit and push": hub was pushed at 00:15 and signal-bridge at 00:17, and bridge tickets 015 to 017 were filed in
  `13075da`

From 00:18 to 00:28 the owner sent 9 more events (6 voice notes, 1 typed message, 2 reactions). They set Signal
formatting rules:
- Private links are fine, but put them in a list or at the bottom, not inline.
- Use bold titles, and no ticket number in front of a link that already carries it.
- Variant E of the formatting samples, with no spoiler markup.
- Project emoji: 🏢 hub, 💬 signal-bridge, 🤖 harnez, 🎙️ voxi, 🖥️ Esprimo (confirmed with a 👍 reaction).

The last voice note (00:27) was a feature idea: reaction-based status icons. 👀 would mean the system picked a
message up, and a different icon would mean the assistant decided or is acting. No ticket exists for it yet.
That the voice notes and reactions carried this exchange is itself the day's features in use.

Questions the main session asked at 15:00 and that have no answer in S4: switch voxi back to its primary model,
make voxi free memory when idle (its llama-server held 2.8 GB in the photo), and the verdict on epic `#277`.

### 8.2 Machine care

| Time | Event |
|---|---|
| 15:00 | Health alert: kernel replaced (reboot needed), swap at 9.6 GiB. The main session attributed the swap to the idle voxi model being swapped out. |
| 20:23 | After the reboot: kernel 7.3.5, RAM 3.5 of 14 GiB, swap empty, disk 6%, no failed units. The snapper check still needs root. |
| 22:37 | `drop-failed` (`claude:haiku:low`) moved 3 set-aside events to `~/.local/state/hub/dropped/`. |

### 8.3 Commits (S1)

| Repo | Commit | Time | Subject (+/-) |
|---|---|---|---|
| hub | `4ed6dd4` | 21:48 | fix: hold the queue on outages; add hub failed and hub retry (+571/-23) |
| hub | `43fc639` | 21:55 | fix: outage review follow-ups (+381/-73) |
| hub | `7103cd6` | 21:57 | fix: timeouts are content failures, not outages (+74/-16) |
| bridge | `f0f9c9a` | 21:48 | feat: styled replies, read ack, reactions as text, typing while waiting (+739/-41) |
| bridge | `85de201` | 21:59 | fix: keep the receive loop free, plain replies by default, lock shared state (+582/-135) |
| bridge | `b83567c` | 22:09 | fix: signal-reply -h and --help print usage instead of sending (+66/-1) |
| bridge | `fc7a59d` | 22:41 | feat: polls for owner decisions (+681/-10) |
| bridge | `ed7c1c4` | 23:10 | fix: poll review findings on fc7a59d; close 011 (+650/-125) |
| bridge | `9bafb78` | 23:18 | feat: attachments in both directions (+1,113/-55) |
| bridge | `c060f80` | 23:33 | fix: attachment review findings on 9bafb78 (+839/-128) |
| bridge | `c57ee57` | 23:39 | feat: transcribe voice notes before forwarding (+692/-9) |
| bridge | `54eb7ce` | 23:50 | fix: voice note review findings on c57ee57 (+618/-99) |
| bridge | `80ed13c` | 23:52 | fix: undo of a failed forward waits for its typing tick (+8/-0) |

New files: bridge `presence.go`, `polls.go`, `attachments.go`, `transcribe.go` and their tests; hub
`outage_test.go`.

### 8.4 Open follow-ups

Filed (11-22):
- bridge `015`: out-of-order undo of two failed forwards
- bridge `016`: silent `sent.json` overwrite
- bridge `017`: inbox default
- hub `025`: Signal slash commands invoke skills

Not filed:
1. `docs/Signal.md` "Not verified live" is stale (item 12).
2. Live checks: poll index base and closed-poll rendering on the phone, attachment `DELETE` and sweep, `.m4a`, the `8735` path.
3. Voice: default endpoint `18131` vs `8735` with no failover; FIFO head-of-line blocking.
4. hub:
   - a second channel for "login expired"
   - `hub stop`
   - an `--accept-cost` path or a documented review tier
5. bridge module path `codeberg.org/flashcoder/signal-bridge` vs remote `github.com/alfizock/signal-bridge`
   (not investigated); `signal-reply -hi` is sent as text.
6. harnez:
   - record worker dollars in a ledger
   - warn when a Haiku turn passes 100k context
   - evergreen candidates §7.1 and §7.2
7. Owner decisions: voxi primary model, idle memory, epic `#277`, snapper root access.

Ticket Extraction Gate: no harnez tickets were allocated; this task was limited to committing the story file.
The bullets under item 6 are the harnez candidates.

### 8.5 Corrections to the inputs

- **Scout:** "the owner did not name the reviewer". Wrong: the 21:42 event has a second line, "ask neon model to
  review …" (S4 line 1348). The draft's "the owner implied the high tier" is also unsupported; no tier was given.
- **Draft:**
  - It said `sig-poll-fix` reported green on the racy test. The test did not exist until `54eb7ce` (`git log -S`),
    so only `sig-voice-fix` did.
  - The pipe-before-restart times "22:10" and "22:50" were UTC; local times are 23:10 and 23:50.
  - The test-to-production ratios were swapped (hub 1.7x, bridge 1.2x).
  - "33 addressed in code" did not match the sources: 31 fixed, 1 partly fixed (hub#5), 1 resolved live and 2
    kept by choice (§6 table, after the review's finding 1).
- **Both inputs:**
  - "Nothing pushed" and "no tickets filed" are outdated. Pushes happened at 00:15 and 00:17 on 11-22, and
    tickets 015 to 017 and 025 were filed after midnight.
  - The draft's "could not confirm the bridge runs `80ed13c`" is resolved by binary mtime and service start time
    (S5).
- **Removed:** the owner's personal name, and "under a second" as a measured fact.

## Review Note

Reviewer: `agy:neon:med` (`gemini-4-neon`), read-only, log `story-review-20261122-002246.log`. **Verdict: minor
fixes.** It confirmed the headline counts (commits, LOC, worker turns, ledger, owner events, the §8.5 correction on
"neon model"). It raised 7 findings, and the writer (`claude:opus:high`) checked each against its cited evidence.
**7 applied, 0 rejected:**

1. rich#4 and rich#5 are fixed in `85de201`, and hub#5 is partly fixed in `43fc639` (§6 table).
2. `-race` does not explain the green report (§7.2).
3. Poll close ran live; only the phone rendering is unchecked.
4. The post-midnight count to 00:17 is 13 events, not 14.
5. The post-midnight cost is now given with its time window.
6. Durations are 10 s and 1m18s.
7. The `-h` cause is now an implementation omission, and the verdict credits agent-discipline slips (including the
   main session's) alongside plumbing.

The post-midnight facts (pushes, tickets, Signal formatting rules, the reaction-icon idea) were added after the
review and have not been reviewed.
