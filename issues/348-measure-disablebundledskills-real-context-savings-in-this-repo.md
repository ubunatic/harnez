# 348 — Measure disableBundledSkills real context savings in this repo

**Status**: Closed — implemented and live-verified
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
offsets the Skills-row reduction at `/context`'s reporting precision. The
API-usage test below establishes that this is a `/context` accounting defect,
not a real loss of the saving.

## Server-reported API validation (2026-09-16)

`/context` is a local command, not an API measurement. Running it with
`--output-format json` reported `duration_api_ms: 0`, `num_turns: 0`, and zero
input, cache, and output tokens for both settings. Its category table is an
estimate generated inside Claude Code.

To measure the real request, ran the same minimal prompt from `/tmp`, outside
`~/projects`, with session persistence disabled:

```text
claude --settings '{"disableBundledSkills":false}' --no-session-persistence \
  -p 'Reply with exactly OK.' --output-format json
claude --settings '{"disableBundledSkills":true}' --no-session-persistence \
  -p 'Reply with exactly OK.' --output-format json
```

Two alternating off/on pairs returned identical server-reported input counts
for each setting. Total input is the sum of `input_tokens`,
`cache_creation_input_tokens`, and `cache_read_input_tokens`, per Anthropic's
prompt-caching usage definition:

| Setting | Sonnet input | Auxiliary Haiku input | Combined input |
|---|---:|---:|---:|
| `false` | 37,498 | 897 | 38,395 |
| `true` | 35,378 | 897 | 36,275 |
| Delta | -2,120 | 0 | **-2,120 (-5.5%)** |

The comparable cache-hit pair produced the same four-token model output and
cost $0.0084952 with the toggle off versus $0.0080712 with it on. This removes
output variation and cache-write pricing as explanations for the input delta.

Conclusion: `disableBundledSkills` both removes the 12 bundled skills and
saves 2,120 actual input tokens for this clean minimal prompt. `/context`
incorrectly moves approximately the same amount from its Skills row into its
System-tools row, hiding the real saving while leaving its displayed total
unchanged. The clean-repo and real harnez-repo `/context` runs reproduce the
same display defect, so it is upstream behavior rather than project config.

## Decision (2026-09-16)

Promote `disableBundledSkills: true` into both debloat presets. The measured
recurring saving justifies making it part of `harnez apply --debloat`; if a
bundled playbook proves load-bearing later, add a lean harnez-authored variant
for that demonstrated need rather than paying for the entire bundled catalogue
on every prompt. Plain `harnez apply` remains unchanged, and the standalone
`--debloat-disable-bundled-skills` flag remains useful when the user wants this
toggle without a deny-list preset.

## Completion

- Recorded the live result and `/context` accounting defect in the 2026-09-15
  debloat study.
- Added the config-driven bundled-skill default to both debloat presets while
  preserving the standalone toggle flag.
- Added regression coverage for the preset default, config opt-out, and
  standalone flag; `make check` passes.
- Installed the updated binary and live-verified `harnez apply --debloat`,
  `harnez status --debloat`, and Claude's surviving user-skill list.

## Definition of done

- Version-stamped, repeated A/B result is documented in a durable study,
  including Skills, System tools, and net reported context rather than
  assuming that a smaller Skills row means net savings.
- The 2.0k System-tools increase is identified as a `/context` accounting
  artifact using server-reported API usage, with enough evidence for a
  go/no-go decision.
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
