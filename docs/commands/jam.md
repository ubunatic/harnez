# Run an Independent Build Jam

Run a jam where several agents build the same thing independently, the host pre-assesses each entry, the owner chooses, and adoption proceeds in phases. Use only for explicit `/jam` or “jam” requests.

## Role

The host orchestrates the work, applies the same brief and comparison profile to every entry, and prepares a sourced assessment. The host never selects the winner: the owner verifies the assessment and decides. Model choices belong to the user. Stop and report when a necessary decision cannot be defaulted.

## 1. Brief

Resolve these items with the owner or from clear project context before dispatch:

- Goal, audience, and how entries will be judged.
- Hard rules, including that input data must be real; do not use lorem ipsum.
- Owner decisions already known and any constraints on keeping entries.
- Entry location, derived or chosen for this work and language. Keep entries outside production. Ask if the location is unclear.
- Budget, number of entries, and models. The user chooses models.

If any required choice cannot be derived safely, pause and report the decision needed.

## 2. Research (optional)

Use low-cost research agents when useful. Ask them to fetch and cite each named site. Have each write a study of findings. Stop any browser or server started during research.

## 3. Build

Dispatch independent agents with the same brief, data, constraints, and entry rules. Each agent works only in its assigned entry and writes a study describing its work and findings. Keep entries separate and staging-only until adoption. Follow `docs/Git.md` for branch and worktree rules; never run installs outside the main tree.

## 4. Compare

Choose one profile and apply it identically to every entry:

- **Web pages:** serve entries side by side on staging and locally. Check desktop and phone widths, emulated light and dark `prefers-color-scheme`, console errors, overflow, and screenshots the host views.
- **CLI tools and libraries:** use the same canary measurements for every candidate, such as start time, memory, disk, and what survives `kill -9`; record one comparison table. See `docs/Canary.md`.
- **Designs and docs:** use a rating table with the same agreed categories for every entry.

Stop every browser and server you start.

## 5. Pre-assess

The host fills a rating table using categories agreed in the brief. Name the source of each score at first mention. Include enough evidence to verify each rating. Do not hide missing data behind a score. Present the table to the owner to verify and choose; wait for the owner's decision.

## 6. Improve once

After the owner identifies candidates for another round, give each remaining entry—or only the chosen entry—one improvement round using the strongest ideas from the others. Reapply the same comparison profile, update the sourced assessment, and let the owner make the final choice.

## 7. Adopt

Make a phased adoption plan and execute each phase through `lean-sprint`. Compare every phase against the selected entry before it goes live. Keep all entries as references until the owner retires them. For visual work, port the markup as well as the CSS so the chosen look survives adoption.

Do not remove entries protected by the brief. Stop if adoption needs an owner decision the brief did not resolve.
