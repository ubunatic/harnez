# 348 — Measure disableBundledSkills real context savings in this repo

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research

---

## Summary

Follow-up from issue 347's research pass. The `docs/studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md`
study measured the `~/.claude` `Skills` category at ~3.1k tokens, but that
measurement used a clean, isolated test project directory — harnez's own
project skills (`sprint`, `lean-sprint`, `issue`, `review`, `commit`,
`tool-feedback-protocol`, etc.) were not loaded in that baseline. Issue 347's
research agent could only infer, not confirm, that most of the 3.1k is
bundled-skill weight.

## Live validation (2026-09-15)

Tested the installed Claude Code 2.1.273 from `/home/uwe/projects/harnez` with
the real settings confirmed to contain neither `disableBundledSkills` nor
`skillOverrides`. Three alternating runs of each configuration produced the
same category values every time:

| Category | Baseline | `disableBundledSkills: true` | Delta |
|---|---:|---:|---:|
| Total reported tokens | 38.3k | 38.3k | no reported change |
| System tools | 7.7k | 9.7k | +2.0k |
| System tools (deferred) | 8.2k | 8.2k | 0 |
| Skills | 3.1k | 1.2k | -1.9k |
| Free space | 928.7k | 928.7k | no reported change |

The toggle therefore **works functionally** on 2.1.273: all 12 skills marked
`Built-in` disappeared, while all 28 skills marked `User` survived. The test
used the one-off command-line setting, so it did not modify the real settings
file:

```text
claude --settings '{"disableBundledSkills":true}' -p '/context'
```

The result corrects issue 347's inference: only about 1.9k of the 3.1k Skills
row was bundled-skill weight in this repo; 1.2k belongs to user/project
skills. More importantly, the reproducible 2.0k increase in System tools
offsets the Skills-row reduction at `/context`'s reporting precision. We know
the toggle removes bundled skills, but do **not** yet have evidence that it
reduces net context.

## Remaining task

- Explain or characterize the reproducible System-tools increase under the
  toggle. Determine whether this is genuine prompt/tool-schema growth,
  category reclassification, or a `/context` accounting artifact.
- Record the live result and explanation in the 2026-09-15 debloat study (or
  a new dated study entry).
- Make the go/no-go recommendation using net context impact and capability
  tradeoffs, not the Skills row alone.

## Definition of done

- Version-stamped, repeated A/B result is documented in a durable study,
  including Skills, System tools, and net reported context rather than
  assuming that a smaller Skills row means net savings.
- The 2.0k System-tools increase is explained or explicitly bounded as a
  reporting artifact/unknown, with enough evidence for a go/no-go decision.
- Confirms that bundled skills disappear while user/project skills remain,
  and corrects issue 347's inference about the 3.1k Skills baseline.
- Feeds into a go/no-go for adding `disableBundledSkills` as a `--debloat`
  toggle default (it already exists as an opt-in flag in
  `internal/claude/debloat.go`'s `DebloatOptions` — this ticket is about
  whether the measured **net** benefit justifies promoting it, not about
  building the flag itself, which already exists). A no-go result is valid.

## Related

- Issue 316 — `harnez apply --debloat` (the flag this measurement feeds a
  recommendation into).
- Issue 347 — disableBundledSkills scope/replacement research (source of
  this follow-up).
