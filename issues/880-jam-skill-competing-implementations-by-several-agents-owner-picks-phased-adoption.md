# 880 — jam skill: competing implementations by several agents, owner picks, phased adoption

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[878-retro-skill-session-report-from-compaction-summaries-and-key-user-messages]],
[[851-bug-hunt-skill-pick-p1-p2-bugs-re-verify-close-obsolete-ones-fix-the-rest-the-way-forward]],
skill `lean-sprint` (adoption phases), ubunatic.com issues 082 (the jam), 085 (adoption plan),
092 (`make variant`), 094 (retiring snapshots), 095 (canary comparison for headbox)

---

## 1. Goal

/goal A `jam` skill runs a competition where several agents on different models build the same
thing independently, the host pre-assesses, the owner picks, and the winner is adopted in phases;
general in its steps, with web pages as the first fully worked-out comparison profile; or stop and
report when a step needs an owner decision the skill cannot default.

## 2. Origin (owner, 2026-10-23)

ubunatic.com ran one jam by hand (issue 082, 2026-10-20/21): three research agents (opus:low)
gathered portfolio, AI dev-log and data input; three high-tier agents (opus:high, sol:high,
agy argon:high) each built a complete plain-HTML mockup of the site in `jam/<agent>/`, staging
only. The host pre-assessed with a rating table, the owner verified ("argon > opus > sol"), argon
got one improvement round adopting opus's strengths, and the adoption ran as lean-sprint phases
085–093 using rendered variants (`make variant`, 092) compared side by side. The headbox research
(095) used a close relative: ten canaries measured each candidate the same way, one table decided.

The owner asked for a general skill: the steps are the same for any kind of work, only the way of
comparing changes.

## 3. Proposed steps

1. **Brief:** goal, audience, hard rules (data must be real, no lorem ipsum), owner decisions
   known up front, where entries live (chosen per type of work, see §6; never in production),
   budget and models.
2. **Research (optional):** cheap agents gather input; each writes a study.
3. **Build:** N agents on different models get the same brief and data, work independently, and
   each writes a study of what it did and learned.
4. **Compare:** one comparison profile (below) applied identically to every entry.
5. **Pre-assess:** the host fills a rating table with categories agreed with the owner, naming
   the source of every score; the owner verifies and decides.
6. **Improve once:** each remaining entry (or only the winner) gets one round taking the best
   ideas of the others; then the owner picks.
7. **Adopt:** a phased plan (lean-sprint per phase), each phase compared against the winner
   before it goes live; entries kept as references until the owner retires them.

## 4. Comparison profiles

- **Web pages (first, fully specified):** entries served side by side on staging and locally;
  host browser check at desktop and phone width, light and dark via emulated
  `prefers-color-scheme`, console errors, overflow, screenshots the host actually views.
- **CLI tools and libraries:** the same canary measurements per candidate (start time, memory,
  disk, what survives `kill -9`), one table.
- **Designs and docs:** rating table only.

## 5. Lessons from the manual runs (to bake in)

- Name the source of any score at first mention ("quality scores" confused the owner in 082).
- Research prompts must say "fetch and cite each named site", or agents skip them.
- Agents stop every browser and server they start (082/088 left orphan browsers).
- Owner rules about keeping entries (ubunatic.com: `jam/argon`, `jam/opus`, `jam/sol` are never
  removed by agents) belong in the brief.
- A visual winner's look comes from its markup, not only its CSS: adoption phases port both.

## 6. Open points

- Where the skill lives (harnez-managed skills vs `~/.claude/skills`) and how it hands off to
  `lean-sprint` for adoption.
- Entry location is not fixed (owner, 2026-10-23): it depends on the type of work and language.
  `jam/<name>/` plus staging-only publishing fit a static website; a Go or other code project may
  need separate dirs or modules on main, or a scratch dir. We usually avoid branches and
  worktrees (work on the default branch, docs/Git.md), so entries stay as dirs on main. The skill
  asks or derives the location per jam (part of the brief) instead of prescribing `jam/`.
- Worktrees are allowed where they proved useful (owner, 2026-10-23): ubunatic.com promotes from
  a short-lived detached worktree while developer work is uncommitted. The problem seen elsewhere
  was agents running `make install` and similar from worktrees, replacing installed binaries with
  builds from a temporary tree; the skill forbids installs outside the main tree.
