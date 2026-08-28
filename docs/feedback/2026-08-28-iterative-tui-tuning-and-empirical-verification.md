# Retrospective: Iterative TUI Tuning, Empirical Verification, and Honest Uncertainty

**Date**: 2026-08-28
**Author**: Claude (Pair Programming with User)
**Context**: Building the `[L] Load` panel in `harnez usage --watch` (CPU/GPU metrics), commits
`90cea91`..`64c3e07`. See [2026-08-28-load-box-cpu-gpu-kernel-metrics.md](../studies/2026-08-28-load-box-cpu-gpu-kernel-metrics.md)
for the technical implementation this retrospective is about.

---

## 1. What This Session Actually Was

Not one feature request — a long, iterative visual-tuning loop with 12+ round trips, each one a
small correction on top of a working implementation: add a CPU box → make it less boring → add
GPU → speed up the refresh → too fast, calm it down → verify a hunch about core-hopping → restyle
to match the rest of the UI → dim the sparkline → no, background-shade it instead → fill history
at startup → remove one vendor tool → remove the other vendor tool too → write it all down. Each
step shipped (built, tested, `make install`, committed) before the next request arrived. That
cadence — small commit, wait for reaction, small commit — is what let 15 commits of visual
back-and-forth stay coherent instead of turning into rework.

## 2. What Worked

- **Verify a claim before answering it.** When the user asked whether CPU cores "hopping" around
  in the sparkline was a bug in our per-core indexing, the answer wasn't obvious from reading the
  code again — so a `taskset -c 3` busy-loop was pinned to one core and `/proc/stat` deltas
  checked before and after. `cpu3` showed `idle_ticks=0` while every other core stayed idle,
  proving the parsing was correct and the hopping was genuine Linux scheduler behavior, not our
  bug. A confident-sounding answer without that test would have been a guess dressed as
  certainty; the 90-second `taskset` experiment turned it into a verified fact. Same principle
  applied to the GPU cost question earlier: instead of asserting "`rocm-smi` is slow," it was
  timed (`time (for i in $(seq 5); do rocm-smi ...; done)`) against direct sysfs reads and got
  actual numbers (~123ms vs ~0.3ms) to reason from and later write into an ADR.
- **Refuse to fabricate data that looks authoritative.** The AMD GPU codename table
  (`amdCodenameByDeviceID`) could have shipped with a dozen plausible-looking PCI device IDs from
  memory (Renoir, Rembrandt, Phoenix, Strix Point...). Only one entry went in — `0x1638` →
  `"Cezanne"` — because that's the only one actually confirmed against this machine's `lspci`
  output in-session. A wrong entry in that table would silently mislabel real hardware and look
  exactly as confident as a correct one; a missing entry just falls through to a harmless generic
  "AMD GPU". The comment on the table says this explicitly, so a future agent extending it
  understands why it's sparse instead of assuming it was an oversight.
- **Reflect back an ambiguous scope decision before implementing it.** When asked to restyle the
  Load box to a template that implied dropping previously-requested data (raw L1/L5/L15 load
  averages, VRAM%), the interpretation was stated in one paragraph *before* writing code, rather
  than silently deciding either way. This wasn't a blocking question — Auto Mode was active and
  the response proceeded to implement immediately after stating it — but it left a clear record
  of what was assumed and why, cheap insurance against redoing a whole restyle pass.
- **Answer the actual question before touching code.** Several turns were pure questions ("is
  this expensive?", "are we using a standard kernel API?", "will this work for other device
  classes?") with no implementation request attached. Answering those directly and stopping,
  rather than proactively "helpfully" also changing code nobody asked to change yet, kept scope
  matched to what was actually asked — the follow-up ADR/removal request came in its own turn,
  once the answer had been absorbed.
- **Live-verify a TUI change instead of trusting the build.** `go build`/`go test` passing proves
  the code compiles and unit-level logic holds; it says nothing about whether a `--watch` frame
  actually redraws at the intended cadence or whether ANSI codes render as intended. This session
  captured real frames via `harnez usage --watch --interval 30s > file.txt` (backgrounded with
  `timeout`, since no real pty was available) and grepped the Load panel out of consecutive
  frames to confirm cadence, alignment, and color codes empirically — catching, for instance,
  that the very first burst-seeded frame really did render a full 10-glyph timeline rather than
  assuming the burst logic worked because it compiled.

## 3. What Could Improve

- **The visual-tuning loop had knowable failure modes that got discovered live instead of
  anticipated.** 100ms redraw → "too noisy" → 1s. A background-color-vs-dim-text ambiguity that
  took two attempts ("dim the sparkline" got implemented as dimmed foreground; the correction was
  "the graphs stay in foreground color... background color"). Neither is a big deal in an
  iterative pairing session — the cost of a wrong guess here is a 2-minute redo, not a
  production incident — but a slightly more specific clarifying question up front ("dim as in
  faded text, or a shaded background panel behind it?") could have saved one round trip. Worth
  noting as a pattern: pure aesthetic requests with words like "dim," "contrasty," or "muted" are
  genuinely ambiguous between foreground and background treatment, and it's cheap to ask.
- **ASR/voice-input garbling should be flagged, not silently guessed past.** "Rokkum SMI tool"
  for "rocm-smi" was resolved correctly from context (per the global `AGENTS.md` guidance on
  phonetic homophones), but silently — no acknowledgment that the input was garbled. For a
  destructive-ish request (removing a fallback path), a one-line "reading that as `rocm-smi`" at
  the top of the response would have been free insurance against a wrong guess going unnoticed by
  the user.
- **The technical/ADR doc and this retrospective doc should probably have been written
  incrementally, not entirely at the end.** The ADR (kernel-standard-sourcing policy) was written
  when the user explicitly asked for it mid-session, which worked well as a natural checkpoint;
  but the broader session-learnings consolidation only happened because it was explicitly
  requested at the very end. A future version of this workflow might benefit from writing a
  short-lived scratch note per major pivot (e.g. right after the 100ms→1s cadence reversal) and
  folding those into a final doc, rather than reconstructing the arc from `git log` after the
  fact — the reconstruction worked fine here since commit messages were detailed, but that's
  relying on commit-message discipline as the only memory of *why*, not just *what*.

## 4. Takeaway for Future Agents Working on `harnez usage`

If you're extending the Load panel or adding a new device class: read
[2026-08-28-kernel-standard-metrics-sourcing-policy.md](../studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md)
first — it's a hard constraint (kernel-exposed data only, no vendor CLI/SDK, not even as a
fallback), not a preference. If you're touching the visual styling of any TUI panel, prefer
capturing a real `--watch` frame (see §2 above) over trusting that a build pass means the
terminal output looks right — ANSI escape sequences and column alignment are exactly the kind of
thing that compiles fine and looks wrong.
