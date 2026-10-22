# Two-agent lean sprint: P1/P2 bug hunt (2026-10-22)

Twelve active P1/P2 bugs, run without user check-ins. Template for the bug-hunt skill (issue 851).

## Flow

- Four read-only searchers (luna:low) re-verified the tickets against live code, in parallel with the
  active developer. Findings went into each ticket as a dated "live check" section.
- Four developer batches (luna:high), one at a time: 849+728, 850+848, 850 M2+701+658+734,
  595+745+714+710+450.
- Host review: diffs, then live checks (real `harnez apply` on the home dir, live duplicate probes,
  real-flow merge e2e, clean `git status` after `harnez init`).

## Outcomes

| Ticket | Result |
|---|---|
| 849, 728 | Closed. Apply adopts legacy skill dirs that hold only a matching SKILL.md; one frontmatter block. |
| 850 | Closed after reopen. IDF weighting (M1) left a false match at 0.594; M2 raised the cutoff to 0.65. |
| 848 | Closed. One-sided delete/rename drops the generated row. |
| 701, 658, 734 | Closed. One Quota-1 section; German slug transliteration; bare release without dist/. |
| 595, 745 | Closed. Provider turn errors (Codex stream, Claude final line) reach the user. |
| 450 | Closed. SIGWINCH forwarded to the child PTY; Loom hosting stays the larger step. |
| 714, 710 | Blocked. Not reproducible; gray usage rows now state their reason. |

## Learnings

- Old tickets drift: 734's title described the opposite of today's behavior; 714/710 did not reproduce.
  Re-verify first, then fix the way forward or make the next occurrence diagnosable.
- A developer closed 850 with acceptance unmet; the host's own live probe caught it.
- A developer autosquashed commits another session had already pushed; main diverged from origin and the
  other session's ticket looked untracked. Fixed by rebasing onto origin/main.
- A PTY test helper swapped `term.GetSize` width/height; review the test, not only the code.
