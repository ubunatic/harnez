# 878 — retro skill: session report from compaction summaries and key user messages

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[241-session-tree-tool-call-efficiency-audit-command-and-agent-skill]],
[[635-harnez-decide-fast-verbatim-transcript-compaction-explore-picker-skills]], skills `story`
and `evergreen` (`~/.claude/skills/`), ubunatic.com `docs/studies/2026-10-23-session-retro-staging-jam-sitegen-headbox-viewer.md` (the manual run this ticket repeats)

---

## 1. Goal

/goal A skill (working name `retro`) turns one long agent session into a candid study in
`docs/studies/` from a cheap extract (compaction summaries plus the owner's own messages), without
reading the raw transcript, and hands durable rules to `evergreen`; or stop and report when a
provider's transcript format offers no compaction summaries to extract.

## 2. Problem & Motivation (owner, 2026-10-23)

A three-day Claude Code session in ubunatic.com (17 compactions) produced most of what we know
about running a website through agents, but its lessons lived only in the compacted transcript.
The raw transcript was 94 MB (tool output, screenshots, agent logs), far too expensive to read.

The manual run that worked: `jq` extracted the 17 compaction summaries (178 KB) and the owner's
102 messages (16 KB), about 50k tokens; the host added `git log` counts per repo and the ticket
closes, then wrote a story-shaped study and moved two durable rules into an evergreen doc. Cost
was well under 1% of the transcript.

`story` today gathers ground truth from git only (status, log, diff) and loses everything a
compaction dropped; `evergreen` "recaps the current session" from live context, which after many
compactions is just the last summary. Neither knows the transcript on disk.

## 3. Proposal

1. **Extract (no model tokens).** A harnez command or script, e.g.
   `harnez session extract [--session <id>] [--out <dir>]`, writes two files:
   - every compaction summary with its timestamp;
   - the owner's own messages, with injected text stripped: system reminders, task
     notifications, skill bodies, command wrappers, tool results, the "continued from a
     previous conversation" preamble.
   Keep slash-command arguments (the manual run lost `/lean-sprint 097`-style messages) and mark
   pasted images as `[Image #n]` placeholders. Print sizes and an estimated token count.
2. **Ground truth.** Commits per repo since session start, tickets filed and closed, studies
   written, agents started (`harnez agent list` session footer).
3. **Write.** The `story` section structure (context, summary, what worked, post-mortem, quality
   table, efficiency, learnings, files), plus a "method and gaps" note naming what the extract
   cannot see. Go back to the raw transcript only with targeted `grep` for a specific fact.
4. **Hand off.** Learnings that are durable go to `evergreen`'s targets (repo docs, AGENTS.md);
   harnez-managed docs and skills get proposals, not edits. Proposed harness changes are listed
   for the owner to decide.

Alignment to settle in the design: either `retro` is a mode of `story` ("story from transcript")
that `evergreen` and `wrap` can call, or a separate skill that both reference. Avoid a third copy
of the story template.

## 4. Findings from the manual run (for the design)

- Compaction summaries repeat earlier content; the "Errors and fixes" and "All user messages"
  sections carry most of the signal, so section-wise extraction can shrink input further.
- Owner messages with a leading `<system-reminder>` block must be stripped, not dropped, or real
  messages vanish.
- Claude Code stores transcripts as JSONL under `~/.claude/projects/<dir>/<session>.jsonl`;
  summaries carry `isCompactSummary: true`. Codex and agy formats need their own probe
  (canary first).
- Pasted images are not in the transcript text; a retro cannot judge visual feedback it never saw.

### Second run: loom session (2026-10-23, loom-8m45)

Retro `loom/docs/studies/2026-10-23-session-retro-editor-colorpicker-session.md` (loom 3e7280c),
written from the recipe above, with feedback for this ticket:

- Input: 19.5 MB transcript, 9 summaries (76.5 KB), about 110 owner messages (19 KB). Read all
  owner messages (about 5k tokens) and only the "Errors and fixes" sections (about 3k tokens);
  about 10k tokens in all. The raw transcript was never opened; the errors sections sufficed for
  the post-mortem.
- The host skipped re-reading summaries its live context already held (the last summary, an
  earlier story).
- Format drift: section headings appear as `4. Errors` and `**4. Errors**`; later summaries say
  "as in the prior summary", so content repeats.
- Noise in the owner extract: `/compact`, `/evergreen`, bash-input echoes, tool-rejection
  boilerplate. Signal worth keeping: `[Request interrupted…]` markers and `[Image #N]`
  placeholders (with the owner text after them, the only clue to what an image showed).
- Not in the extract: ticket type counts and per-ticket timings; taken from the tracker, gap stated.
- Commit pitfall: `git commit -- <path>` fails for a new file until it is `git add`ed.

Requirements derived from both runs:

- Extract command options: `--user`, `--summaries`, `--section errors` (heading-format
  tolerant), `--since`; drop slash-command noise and bash echoes but keep slash-command arguments,
  interrupts and image placeholders.
- Output header: session file, size, message count, time span, so the right session is provable.
- Dedupe summaries (later ones carry earlier errors forward).
- Skill text: skip what the live context already holds; extend or cross-link an existing story
  instead of duplicating it; a mandatory "method and gaps" section; `git add` new files before
  committing by explicit path; evergreen changes to harnez-managed docs stay proposals.
- Providers: only Claude tested so far.

### Third run: harnez host session (2026-10-23, harnez-bjbi)

Retro `docs/studies/2026-10-23-session-retro-harnez-host-two-days.md` (6bf5f306). Input: 18.3 MB
transcript, 7 summaries (77 KB), 91 owner messages (20 KB); about 15–20k tokens including writing;
raw transcript never opened. Existing per-sprint studies were linked, not retold (cheapest run).

- **Lost start:** the first extracted owner message came 2h15m after session start because the
  opening slash command was filtered (same loss as run 1).
- **Shared repo:** about 200 commits and the close list include other sessions' work; nothing in
  the extract attributes commits to this session.
- **Owner-side noise:** voice-input duplicates arrive as separate messages; bash-input/stdout
  echoes are noise but once held the key clue (the owner's own `time` output for an open bug).
- **Cutting messages to 300 characters lost decisions;** two long messages had to be inferred from
  summaries. No hard cut, or at least 1000 characters.
- Heading drift again (`4. Errors and fixes:` with a colon); errors sections again sufficed.

Implementation advice from the harnez host:

- Command `harnez session extract` (or `harnez transcript extract`) in a new package
  `internal/transcript`, one provider adapter per agent, Claude first.
- **Do not reuse 635's `internal/decide/compact.go`:** `ParseJSONLTranscript` expects a top-level
  role/content shape, not Claude Code's nested `message.content` with `isCompactSummary`, keeps
  every line as `map[string]any` and caps lines at 10 MB, risky on 18–94 MB files. Use a streaming
  decoder that emits only summaries and owner text.
- Reuse transcript-path discovery from `internal/usage` (`history.go`, `project.go`) and the
  subagent session store for "agents started" instead of parsing the `harnez agent list` footer.
- `--since`/`--until`; attribute commits to the session (session-id trailer, or the commit time
  ranges of this session's tool calls).
- Header adds the first owner message's timestamp; a large gap after session start flags dropped
  messages. Heading regex `^\s*(\*\*)?4\.\s*(\*\*)?Errors`; dedupe repeated summary sections.

## 5. Acceptance

- The extract of the ubunatic.com session `13d4828e-…` reproduces the manual numbers (17
  summaries, about 100 owner messages) and keeps slash-command arguments.
- One retro written with the skill on another long session, reviewed by the owner.

Status check (2026-10-24): not started. No retro skill in `config.yaml`, no `session extract` command.
Evidence base so far: manual retros (harnez study 6bf5f306, loom feedback 52f46749/1a74a670).

## Delivery

- M1 delivered (21beea8b): `harnez session extract`, streaming, internal/transcript; apply_test pipe deadlock fixed.
- M2 delivered (a52d63bf): `retro` skill, user-invocable-only.
- Review (opus:low, `e911-revD`): on session 13d4828e, 18 summaries are all real (one newer than the manual
  count); 137 owner messages include 18 doubled `/compact` records (plain `"/compact"` plus its
  `<command-name>/compact</command-name>` twin, ms apart), so 119 real. No other leaks.
- M3 implementation: deduplicate matching plain/structured slash-command pairs within one second, preserve
  intervening summaries, add UUID/JSONL line provenance to text and JSON output, and assert slash-message counts.
- Installed extract on 13d4828e reports 18 summaries and 128 owner messages. The JSONL has nine `/compact`
  pairs (18 rows); dropping the plain row in each pair retains the structured twin, so the M3 rule reduces 137
  to 128. The pre-work expectation of 119 would remove both rows in every pair and conflicts with retaining the
  `command-name` twin. The owner should resolve which count/behavior to use before closing the ticket.

## M3 Pre-Work / Required Refinements

1. Deduplicate the doubled slash command (`internal/transcript/claude.go:41,112`): drop the plain-string record
   when its `command-name` twin follows within about 1 s. Fixture test for the pair, and a near miss (same text
   typed again later stays).
2. Add the source `uuid` and line number to each `Record` (`claude.go:12`) and to text/`--json` output, so the
   retro can point back to the raw JSONL (`docs/commands/retro.md:46`).
3. `cmd/harnez/session_test.go:43`: assert the slash-command count, not only `Contains`.
4. Rerun on 13d4828e: observed 18 summaries, 128 owner messages after retaining one row per slash-command pair;
   resolve the count discrepancy above.
