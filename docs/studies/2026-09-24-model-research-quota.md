# Model quota attribution research — 2026-09-24

## Scope and method
- Read quota-history.jsonl, ~/.harnez/agents/*.json, Codex rollouts, and Claude project transcripts; all timestamps interpreted as UTC.
- Normalized provider window labels to 5h/weekly. Null rate_limits excluded.
- Claude: fitted 5h used_percent step changes against per-model transcript new-token sums (input + output + thinking), ordinary least squares through origin; cached input excluded from new tokens.
- Claude readings' reset_at values vary by polling milliseconds, so reset identity was reduced to minute. Window: 2026-09-20 00:00 through 2026-09-23 22:20 UTC.
- 3,579 within-reset adjacent reading intervals; skipped 126 reset-boundary intervals and 2,695 intervals without transcript messages. Fit used 884 activity intervals; used_percent is integer-rounded, so all estimates are coarse and sample-dependent.
- Transcript totals count assistant message records as turns; retries/tool continuations may make this differ from user-visible turns. Cache = cache-read + cache-creation tokens.

## Claude estimates

| Model | Assistant records | New tokens (incl. reasoning) | Cached tokens | Fit sample | Estimated points / 100k new | Estimated points in window |
|---|---:|---:|---:|---:|---:|---:|
| claude-haiku-4-5-20251001 | 4,393 | 1.312M | 313.5M | 884 intervals | 5.013 | 65.8 |
| claude-opus-5 | 1,449 | 1.108M | 143.0M | 884 intervals | 8.393 | 93.1 |
| claude-opus-5-5 | 1,046 | 0.702M | 108.5M | 884 intervals | 7.230 | 50.8 |
| claude-sonnet-5 | 4,226 | 3.232M | 470.6M | 884 intervals | 5.966 | 192.8 |

Reasoning tokens are included in new tokens; the per-model reasoning split was not separately reconciled in this bounded pass. Claude has no gpt-6-luna model, so same-provider cost multiples against that baseline are undefined.

## Other providers and limits

- Codex: found 10,637 non-null token_count events in the recent inspected rollouts and 194 recent 5h quota transitions. These are account-wide; a per-rollout pair is clean only when no other rollout has a token_count event between its endpoints. No reliable per-model clean-pair accounting was completed, so Codex token/quota/cost rows are withheld rather than inferred from overlapping activity.
- agy: five agent JSON records were present. Session-level attribution would require matching each record's actual time span to quota points and confirming no unrecorded activity; no defensible per-model estimate was established.
- Percentage estimates have integer rounding error at every reading. Reported coefficients are regression estimates, not provider billing rates; do not interpret them as monetary cost.
