# Jules one-shot reports (ticket 1075)

These are one-shot Google Jules runs from 2026-11-01. A report is a lead, not a fact, so check it against the code before relying on it. All references were checked at `c7429fb7` (the pilot) or `c6832711` (everything else).

"Held" counts the `path:line` references that say what the report claims. A placeholder counts as a failure.

## Ticket audits

| Ticket | Verdict | Held | What the check found |
| --- | --- | --- | --- |
| [923](923.md) | still valid | 3/3 | Its correction is right: the watch view now uses loom. |
| [927](927.md) | partly done | 4/8 | Below the 80% bar: cites `Makefile:1` and a missing file as placeholders, and an unrelated `Makefile:101`. The errcheck lines are right. |
| [944](944.md) | still valid | 4/4 | |
| [950](950.md) | still valid | 0/0 | Gives no references. The searches it reports hold up. Names generated doc copies, not the sources. |
| [951](951.md) | already done | 5/5 | Confirmed: the mutex came in `87ad04a8`, and the test passes under `-race`. Ticket closed. |
| [959](959.md) | partly done | 4/5 | `spec/agent.yaml:104` is wrong; the text is at line 341. |
| [965](965.md) | still valid | 4/4 | Two references are one line above the code they mean. |
| [966](966.md) | unclear | 1/1 | Thin, but right that the ticket waits on an owner decision. |
| [971](971.md) | partly done | 3/3 | Its correction is right: `agent status --reply` exists. |
| [985](985.md) | still valid | 6/6 | |
| [989](989.md) | partly done | 5/5 | |
| [1012](1012.md) | partly done | 4/5 | `.harnez/rules/Tools.md:88-93` is wrong; the text is at line 96. |
| [1019](1019.md) | partly done | 3/3 | Its next step names `cmd/harnez/agent_start.go`, which does not exist. |
| [1026](1026.md) | partly done | 3/3 | |
| [1031](1031.md) | still valid | 1/1 | Thin: it missed the existing `auto_compact` settings. |
| [1042](1042.md) | partly done | 6/6 | |
| [1045](1045.md) | still valid | 6/6 | |
| [1053](1053.md) | already done | 2/2 | Confirmed. Ticket closed. |
| [1061](1061.md) | still valid | 14/14 | |
| [1072](1072.md) | still valid | 2/2 | The run stopped to ask whether its plan was right, so it opened no PR. Taken from its last message. |

## Other uses

| Report | What came of it |
| --- | --- |
| [Code audit of `internal/issues/maintenance`](code-audit-issues-maintenance.md) | One real but minor finding, confirmed by reading the code. It missed the bigger defect that the test drafts exposed. |
| [Docs check of `docs/CLIDesign.md`](docs-check-CLIDesign.md) | Its 5 mismatches hold; one "missing" flag (`--quota-1`) was wrong and is corrected in the file. The run stopped to ask a question, so it opened no PR. Taken from its last message, without its line references. |
| [Failing test drafts for 1017](drafts/) | Three independent drafts, and the most useful result of the day: see ticket 1017. Draft `pr59` is the one to start from. These are `.txt` files because they fail on purpose. |
