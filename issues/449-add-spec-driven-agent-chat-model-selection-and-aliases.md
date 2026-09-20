# 449 — Add spec-driven agent chat model selection and aliases

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Usability

---

## 1. Problem & Motivation

`harnez agent chat` currently requires an explicit provider/model selection,
even when the user wants the provider's configured default or the healthiest
available agent. Model aliases and tier policy are also duplicated in code,
making them difficult to tune as provider quotas and model names change.

## 2. /goal

Make `harnez agent chat` accept provider-only, tier-only, and no-argument forms
and resolve them through an editable spec. With no explicit provider, choose the
eligible agent with the most remaining weekly quota while excluding agents at
or above 80% of their 5-hour quota; use deterministic lower-ranked fallbacks
when needed.

## 3. Scope and Constraints

- `harnez agent chat codex`, `agy`, and `claude` use that provider's last
  configured model and let the provider choose its own default model.
- `harnez agent chat` selects the agent with the highest remaining weekly
  quota among agents below the 80% 5-hour utilization limit, then the next
  eligible candidate, and so on. If no candidate is eligible, report a clear
  error with quota status rather than exceeding the limit.
- `harnez agent chat low` selects the healthiest eligible low-tier option:
  `luna:low`, `haiku`, or `flash37:low`.
- `harnez agent chat med` selects the healthiest eligible medium-tier option:
  `sol:low`, `sonnet`, or `flash37:low`; `flash37:low` is intentionally valid
  here because provider effort does not correspond to a low capability tier.
- `harnez agent chat high` selects the healthiest eligible high-tier option:
  `sol:med`, `opus:low`, or `flash38:low`.
- Add concise aliases such as `opus -> claude:opus:low` and equivalent aliases
  for the configured models.
- Put provider defaults, aliases, tier groups, ranking, quota thresholds, and
  fallback policy in an editable spec file under `spec/`; code should load the
  spec rather than duplicate these values.
- Preserve explicit full model specifications and make selection deterministic
  for equal quota values. Emit the resolved choice and fallback reason.

## 4. Acceptance Criteria

- All listed chat forms parse and start the expected provider/model class.
- Provider-only forms use the provider's configured default without forcing a
  model name.
- No-argument and tier-only selection obey weekly ranking and the strict
  `<80%` 5-hour eligibility rule, with deterministic fallback/error behavior.
- Aliases resolve to their spec-defined canonical models.
- The spec can be edited to change defaults, aliases, tier membership, and
  thresholds without changing Go source.
- Tests cover parsing, quota ordering, the 80% boundary, ties, fallback,
  provider-only defaults, aliases, and no-eligible-agent errors.
