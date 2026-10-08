# 755 — Add Claude Haiku 5.5 to the harnez model list

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `spec/agent.yaml`, `spec/pricing.yaml` (planned by 445), `harnez agent models`

---

> `/goal`: Add Claude Haiku 5.5 (`claude-haiku-5-5`) as a selectable harnez agent model with a verified cost tier and role, or stop and report when blocked on a user decision (for example, a price figure that cannot be confirmed) or a denied permission.

## 1. Problem & Motivation

harnez lists Claude Haiku only as `claude:haiku`, which points at the Claude CLI alias `haiku`. Claude Haiku 5.5 (`claude-haiku-5-5`) is now available, but harnez cannot select it by name, so it does not appear in `harnez agent models`. Users who want the cheapest Claude tier for mechanical work cannot pick the 5.5 generation.

## 2. Technical Specification / Findings

- Current entry: `spec/agent.yaml` `claude:haiku` with `name: haiku`, `tier: low`, `cost: 12`, role `mechanical`. It is the CLI alias, so it does not pin 5.5.
- Sibling 5.5 entries (`agy:opus`, `agy:sonnet`) use the full model ID (`claude-opus-5-5`, `claude-sonnet-5-5`). Haiku 5.5 should follow that pattern.
- Cost: `cost` is a relative score, not dollars. Haiku 5.5 price is unknown. Do not set a value from the Haiku 4.5 figures below.
- Pricing research (web search, 2026-10-08): no source mentions Haiku 5.5. Third-party aggregators list Haiku 4.5 at about $1.00 per million input tokens and $5.00 per million output tokens, with cache reads around $0.10 per million. Sources disagree, and Anthropic's pricing page is the authority to check: platform.claude.com/docs/en/about-claude/pricing.
- Capabilities: no 5.5 benchmarks were found. Haiku 4.5 reported 73.3% on SWE-bench Verified and 41% on Terminal-Bench, with a 200K context window and 64K max output. Treat these as 4.5 figures only.
- Open question: keep the existing `claude:haiku` alias and add a new entry, or repoint it to `claude-haiku-5-5`. Repointing changes what existing users get from `haiku`.
- Related: #445 plans rate cards for `haiku` (latest); the 5.5 price should feed that table once confirmed.

## 3. Implementation & Verification Plan

1. Confirm Haiku 5.5 pricing and capabilities on Anthropic's official pages; record the source here.
2. Add a spec entry in `spec/agent.yaml` for `claude-haiku-5-5` following the `agy:opus` pattern, with aliases and a cost tier chosen from the confirmed data. Use the user's decision on whether `haiku` repoints.
3. Run `make install`, then `harnez agent models` and confirm the new row appears with the right model ID.
4. Run the spec tests and any model-list tests, and confirm no Go code duplicates the spec values.
