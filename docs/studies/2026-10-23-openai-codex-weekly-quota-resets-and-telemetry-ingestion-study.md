# Study: OpenAI Codex Weekly Quota Resets and Telemetry Ingestion Hypotheses

**Date:** 2026-10-23  
**Status:** Complete  
**Authors:** AGY Orchestrator & Harnez Advisor (`luna:med` / `codex:gpt-6-luna`)  

---

## 1. Executive Summary

This study documents an investigation into unexpected OpenAI Codex weekly quota reloads observed around October 22–23, 2026. Two specific questions were evaluated:
1. Whether an official or widespread weekly quota reset occurred on or around October 22, 2026.
2. Whether OpenAI selectively grants unexpected quota resets to users performing heavy coding or agentic workflows to incentivize sessions and harvest high-value code telemetry.

**Key Findings:**
- **Confirmed Public Reset:** OpenAI Codex leadership (Tibo Sottiaux) publicly announced a Codex limit update/reset on October 22, 2026, targeted to land by the morning of October 23 PST. Independent trackers (e.g., LimitTracker) and community developer threads corroborate the delivery.
- **Selective Quota Grants:** OpenAI's terms and help documentation explicitly disclose that promotional and automatic resets are selectively distributed without guaranteed schedules or published criteria.
- **Data Harvesting Hypothesis:** The hypothesis that OpenAI selectively rewards heavy coding users to harvest code telemetry is **unsupported by available public evidence**. For personal accounts (Plus/Pro), OpenAI already retains model-training rights by default unless opted out; for business/enterprise tiers, training is contractually excluded. Telemetry logging occurs uniformly across all tiers regardless of quota manipulation.

---

## 2. Weekly Reset Timeline & Discovery (October 2026)

### 2.1 The Event
On October 22–23, 2026, developers running heavy coding workflows noted that exhausted or near-depleted weekly Codex usage allowances suddenly returned to full (100% capacity) prior to their scheduled rolling weekly expiry.

### 2.2 Public Announcement & Verification
- **Public Post:** On October 22, 2026, OpenAI Codex lead Tibo Sottiaux announced:
  > *"Lands tomorrow before 10am PST."*
- **Scope:** This was specifically scoped to **Codex usage limits**, rather than a broad ChatGPT web quota reload.
- **Community Corroboration:** Community trackers (including LimitTracker) and discussion threads on Reddit (`r/codex`) confirmed that a rollout wave refreshed Codex limits between late October 22 and October 23.
- **Precedent:** Similar unannounced or abruptly announced resets occurred in September 2026 (e.g., the September 3–4 banked promotional drops and September 7 automatic resets during the GPT-6 Astra rollout, along with a September 9 incident where unexpected resets were reported on the OpenAI status page).

---

## 3. Quota Mechanics: Automatic vs. Banked vs. Rolling

OpenAI maintains several distinct reset mechanisms for Codex and ChatGPT subscription tiers:

| Reset Type | User Action Required | Visibility in History | Notes |
| :--- | :--- | :--- | :--- |
| **Regular Rolling Refreshes** | None | No (Timer in UI) | Rolling windows (5-hour and 7-day weekly allowances). |
| **Banked Promotional Resets** | Manual redemption | Yes (30-day log) | Granted during major launches; user must click "Redeem" or pass `/usage`. |
| **Automatic Promotional Resets** | None | No | OpenAI injects full quota resets across eligible cohorts without notice. |
| **Paid Instant Resets** | Purchased | Yes | Advances the weekly allowance immediately for personal Plus/Pro plans. |

When an automatic promotional reset lands, the user's weekly allowance immediately jumps to full, and the reset countdown timer restarts, creating the impression of a surprise early refill.

---

## 4. Evaluation of the "Quota-for-Code-Telemetry" Hypothesis

### 4.1 The Hypothesis
Heavy agentic coders observed that when running intensive multi-turn programming sessions, sudden resets appeared to prolong their activity. This raised the suspicion: *Is OpenAI actively detecting heavy coding sessions and granting quota extensions to keep developers coding so it can monitor or ingest high-value coding data?*

### 4.2 Analysis & Evidence

1. **Lack of Corroboration:**
   - There are no leaks, whistleblower disclosures, developer audits, or reverse-engineering findings indicating that OpenAI's quota controllers selectively inject allowance into individual high-value session streams.
   - The timing of unexpected resets aligns with broad cohort releases (e.g., Astra rollout events, Codex model updates, or capacity-driven rebalances).

2. **Legal & Contractual Data Rights:**
   - **Personal Plans (Plus / Pro):** Under OpenAI's Terms of Use, prompt inputs and code outputs may be used to train models by default, unless the user explicitly toggles off *“Improve the model for everyone”* in Data Controls (and the corresponding *“Include environments”* setting in Codex). If training is enabled, OpenAI already possesses the legal right and technical ingestion pipeline for that data. There is no incentive to covertly manipulate quotas to obtain data they already have access to.
   - **Team, Business, & Enterprise Plans:** Data is explicitly **excluded from model training by default**. Using business accounts for clandestine training would violate enterprise commercial terms and SOC 2 / compliance certifications.

3. **Operational Telemetry vs. Training Ingestion:**
   - OpenAI's Privacy Policy specifies that operational logs, usage metrics, diagnostic telemetry, and safety-screening data are processed across all tiers to maintain service reliability and enforce abuse protections.
   - A training opt-out does not disable operational telemetry, but operational telemetry is standard across the entire platform and does not depend on dynamic quota bonuses.

4. **Why Selective Resets Occur:**
   - OpenAI's documentation confirms that promotional eligibility is selective: OpenAI targets specific account cohorts, geographies, subscription ages, or capacity bands.
   - During major model iterations or capacity expansions, OpenAI frequently opens the throttle on active developer cohorts to load-test infrastructure and gather macro evaluation benchmarks.

---

## 5. Practical Implications & Recommendations

1. **Quota Tracking:**
   - Subscription limits do not provide an external webhook or push notification when promotional resets occur.
   - For predictable scheduling, rely on the countdown timestamp in **Settings → Usage** or run `harnez usage` / `/usage` within the Codex harness.
   - Local tracking tools (such as `codex-monitor`) can monitor authenticated local session state and provide desktop alerts when the weekly window resets.

2. **Data Hygiene for Heavy Agent Sessions:**
   - Developers working on sensitive or proprietary codebases using personal Plus/Pro subscriptions should verify that **Model Training** and **Codex Environment Sharing** are disabled in settings.
   - Use Business, Team, or Enterprise workspaces where zero-data-retention and training-exclusion terms are legally enforced.

---

## 6. References
- OpenAI Usage & Limit Guide: `https://help.openai.com/en/articles/20001516-managing-usage-with-gpt-6-astra-in-work-and-codex`
- Codex Reset Mechanics: `https://help.openai.com/en/articles/20001498-how-resets-work-in-codex`
- OpenAI Privacy Policy & Data Controls: `https://openai.com/policies/privacy-policy/`
- OpenAI Business Privacy Commitments: `https://openai.com/business-data/`
