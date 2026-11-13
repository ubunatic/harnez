# Harnez Cleanup

Shrink the agent-facing rules, docs and skills in scope, and show with cheap test sessions that
agents still follow them. Triggers: only on explicit `/harnez-cleanup`.

Scope is the argument, read as prose:

- none: this repository's rules (`AGENTS.md`, `.harnez/rules/`, the docs `AGENTS.md` lists);
- skill names: those skills;
- "full sweep": every instruction file, doc and skill this session can see, global ones included.

## 1. Measure

Record sizes before changing anything; in a Claude session `/context` shows what loads at start.

- **Hotspots**: the largest files, and whatever loads into every session.
- **Cutoffs**: docs of 100 lines or more (the harnez guard blocks a native read), and reads long
  enough that a cheap agent truncates them on its own.
- **Misconfiguration**: an `@path` that inlines a whole doc, or is meant to and does not (Claude
  Code imports only a clean `@path`; a trailing comma or a code span stops it); installed copies
  that differ from their source; rules that contradict each other; a skill description that
  triggers on the wrong work.
- **Stale content**: commands, flags, files and tickets that no longer exist. Check each
  `harnez` command against `harnez <verb> --help`.

## 2. Baseline

Before the first edit, write one fixed task that makes an agent use the rules in scope, and a
list of checks you can observe in its output files and executed commands. Pick checks the
current text sometimes fails; a baseline with full marks cannot show a loss. Prepare fixtures
yourself so every session starts from the same state. The task forbids repository changes;
sessions write to a scratch directory. Run it on a cheap model at `low` and
at `high` effort (`harnez agent models` shows cost); add a second vendor when the rules reach
more than one harness.

```bash
HTO=0 harnez agent start --name <n> --role developer --model <model>:<effort> -f <prompt-file>
harnez agent resume --name <n> -- "<question>"
```

Start sessions as background tasks. Score from the output files and the harness's command log,
never from a summary or transcript the session writes. A check you could not observe is not
applicable, not a pass. Then ask each session which rules it read, where, and which it set
aside; resume a Claude session with `/context` for its loaded sizes.

## 3. Rewrite

Write for a capable agent that knows git, Go, Make and shell.

- Keep what is the owner's own: house style, harnez commands, invariants, limits. Drop standard
  practice, motivation, and whatever `harnez <verb> --help` prints.
- One short imperative per rule, stated once. No answer formats, no templates.
- Add a `harnez` command only in a form you ran successfully; for one already in the text the
  `--help` check is enough.
- Edit sources, not generated content (`harnez:begin/end` blocks, bundled doc copies, installed
  skills). In the harnez repository change the source and its lite variant, then reinstall as
  `AGENTS.md` says; anywhere else file an issue for harnez.
- Before deleting a phrase, grep the tests for it. If a test asserts the prose, leave that
  passage as it was and file a ticket to loosen the test; never bend new wording to fit it.
- One pass per run. Stop when the next cut would need a design decision; file it.

## 4. Retest

Rerun the baseline task after each change and compare check by check. A lost check means fix
or revert before the next cut. One run is a hypothesis: when a result decides something, repeat
it three times on the cheapest tier. The session budget wins over reruns: say which results are
single runs and which change went untested.

## 5. Record

- One ticket per bigger finding: `harnez issues new -d . "<title>"`.
- In the work ticket: sizes before and after, the score per session, what each rerun caught.
- A study of the run with `/study`.
- Delete the test sessions: `harnez agent delete --force --name <n>`.

## Limits

- Say how many test sessions you will run before starting; eight is the default ceiling.
- Files outside this repository are reported, and edited only with the owner's go.
- A role that cannot edit hands the rewrite to one developer helper through the ticket. A role
  that cannot start agents stops after Measure and reports.
