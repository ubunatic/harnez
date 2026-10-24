# Session Retrospective from Transcript Evidence

Use when the owner invokes `retro` with a Claude Code session ID/prefix or JSONL path.
Write a candid session study from a cheap extract, then hand durable lessons to `evergreen`.
This is a separate workflow from `story`; use story's required sections without copying its template here.

## Extract first

- Only Claude Code transcripts under `~/.claude/projects/<slug>/<id>.jsonl` are supported.
  For another provider, stop and report the unsupported format; do not invent a parser.
- Resolve the requested session with `harnez session extract <id|path> --json`.
  If the owner did not identify a session, ask for its ID or path. Do not guess from the newest file.
- Redirect the extract to a private temporary file and check the command's exit status before reading it:

```bash
retro_extract=$(mktemp)
harnez session extract <id-or-path> --json > "$retro_extract"
```

- JSON contains `source.path`, `source.bytes`, ordered `records` (`kind`, `timestamp`, `text`),
  and `stats` (summary count, owner-message count, text bytes, first/last selected record, truncated tail).
  Default text output is also available. An error can leave a partial extract; discard it and report the error.
- Inspect metadata first to prove the session: resolved path, source size, counts and selected time span.
  State that first/last selected records may differ from the session's actual start/end.
  If `truncated_tail` is true, report that the incomplete final record was omitted.
- Read owner messages completely in bounded chunks, preserving long decisions, slash-command names and arguments,
  interruption markers and `[Image #n]` placeholders. Images themselves are absent; never infer visual quality.
  For example, select `.records[] | select(.kind == "user")` with a local JSON tool into another private file,
  then use line-bounded reads. Do not print the whole JSON extract into model context.
- Read only needed summary sections first (especially errors/fixes and owner decisions). Headings vary:
  `4. Errors`, `**4. Errors**`, `4. Errors and fixes:`. Repeated summaries may repeat or refer to earlier content;
  deduplicate the evidence while reading, and skip material already in live context. The extractor preserves all
  summaries; it does not deduplicate or offer section/date filters.
- If there are no compaction summaries, stop and report that gap instead of silently making a transcript-based retro.
  Tool results, assistant replies, task notifications, system reminders and meta skill bodies are intentionally absent.
  Keep temporary extracts local; do not commit transcripts or extracts. Remove your temporary files when finished.

## Ground the study

- Check current git status, then bounded git history for the selected time span in each relevant repository.
  Verify commit counts, ticket files/closes and studies against git and the tracker.
  On shared branches these are repository totals, not proof this session produced every commit; label attribution gaps.
- Use the Harnez session store for agent-start evidence if available. Do not infer agent counts or durations from prose.
- Search for an existing story/retro covering the same work. Extend or cross-link it instead of duplicating it;
  link per-sprint studies rather than retelling them.
- Return to the raw JSONL only for a specific unresolved fact via a targeted bounded search.
  Never read the whole transcript into context or inspect every tool result.

## Write and hand off

- Read the installed `story` skill's required sections and output-location rules (source: `docs/commands/story.md`
  in Harnez). Follow that shared structure for context, outcome, successes, candid failures, quality, efficiency,
  learnings and file/commit evidence. Respect `docs.studies: false` and the repository's local-docs fallback.
- Add a mandatory **Method and gaps** section: identify session/source, extraction command, counts, evidence read
  and skipped, time range, truncation, unavailable images/tool evidence and any uncertain commit attribution.
  Separate observations from inferences; do not present summary claims as independently verified facts.
- Propose durable rules for `evergreen`'s repo docs/AGENTS.md targets. Harnez-managed docs and skills receive proposals
  for the owner, not direct edits. List harness changes for owner decision; do not invoke an unrequested workflow.
- Present the draft and open questions for owner review. Do not describe it as owner-reviewed before that happens.
  When committing is authorized, `git add` new study files first, then commit only explicit paths; update the study
  index only when the repo enables it. Report the study path, evidence counts, gaps and proposed evergreen handoff.
