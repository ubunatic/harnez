# 300 — TUIDesign.md gap: raw-mode input has no guidance, SetReadDeadline-on-pty trap is undocumented

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: [docs/TUIDesign.md](../docs/TUIDesign.md)

---

## 1. Problem & Motivation

`docs/TUIDesign.md` ("consult this when adding or changing a timeout gauge,
progress/usage bar, sparkline, spinner, or other repeatedly redrawn
terminal indicator") already captures hard-won terminal-width wisdom for
*output* — including the exact problem class of "emoji/wide glyphs render
inconsistently across terminals" (its "Avoid emoji clocks... their
presentation and width vary by terminal and font" and sextant-mosaic
font-coverage notes). It has zero coverage of raw-mode *input* handling —
confirmed by grepping the whole `docs/` tree for `SetReadDeadline`,
`MakeRaw`, `bufio.Reader`, "escape sequence", "CSI": no hits anywhere.

A sibling project (`dai`, a hand-rolled stdlib+`x/term` TUI with no
framework) hit a real, non-obvious trap in exactly this gap this session:
disambiguating a lone `Escape` keypress from the start of a CSI sequence
(`ESC [ A` for arrow keys, etc.) requires bounding how long the reader
waits for a possible follow-up byte, since a lone Escape sends exactly one
byte and an unbounded `bufio.Reader.Peek(1)` blocks until the *next*
keypress arrives. The obvious-looking fix — `os.File.SetReadDeadline` — was
planned, implemented, and looked plausible in code review, but **does not
work against a real pty in this environment at all**: it returns `"file
type does not support deadline"` every time, confirmed via a standalone
probe under `script` (a real pty, not a regular file or `/dev/null`). The
actual fix needed `golang.org/x/sys/unix.Poll` on the raw file descriptor
instead (gated on `bufio.Reader.Buffered() == 0`, so it only polls when the
reader's own buffer is genuinely empty).

This cost real time to discover specifically *because* it looked like a
sound plan through code review alone — the bug only surfaced once actually
driven through a pseudo-tty, and even then initially looked like "the fix
did nothing" rather than "the fix silently isn't applying at all." Any
other harnez-managed project building a raw-mode terminal reader from
scratch (rather than a full framework like bubbletea, which handles this
internally) would plausibly hit the exact same trap and the exact same
false confidence from `SetReadDeadline` compiling and returning no error
path that looks obviously wrong at a glance (it *does* return an error —
just one easy to not check/log prominently, since `SetReadDeadline`
failing is not commonly expected to fail at all on Unix).

## 2. Detailed Technical Specification

Not proposing a new doc — `docs/TUIDesign.md`'s existing framing ("consult
this when adding... a repeatedly redrawn terminal indicator") is close but
input-side, so either broaden its scope slightly or add a short adjacent
section. Concretely:

- Add a "Raw-mode input" section (or a new `docs/TUIInput.md` if scope
  should stay narrow) covering:
  - `os.File.SetReadDeadline` does not work on a pty/tty file descriptor on
    Linux — confirmed empirically, not from documentation (the Go stdlib
    doesn't clearly state which file types support deadlines; it's a
    per-platform, per-fd-type runtime behavior). Always check and log/handle
    its returned error rather than assuming it silently succeeds.
  - The working alternative for disambiguating a lone Escape from a CSI
    sequence's start: poll the raw fd with `golang.org/x/sys/unix.Poll`
    (short timeout, e.g. 20-50ms) — but only when the `bufio.Reader`'s own
    buffer is empty (`Buffered() == 0`); if a full multi-byte sequence
    already arrived in one read() burst, it's already buffered and there's
    nothing to wait for, so no timeout logic should run in that case.
  - Point to `dai`'s `internal/tui/keys.go` (`peekByte`, `readEscapeSequence`)
    as a working reference implementation, the way other harnez docs
    reference concrete project code for a pattern.

## 3. Implementation & Verification Plan

1. Decide placement: extend `docs/TUIDesign.md`'s scope note plus a new
   section, or a sibling `docs/TUIInput.md` referenced alongside it.
2. Write the section per the concrete findings above (this is transcribing
   an already-verified finding, not new research — no additional canary
   needed).
3. No code changes. Verify by re-reading for a future agent building a
   raw-mode reader from scratch: could they find this *before* spending
   time on the `SetReadDeadline` dead end, if they'd read the doc first?
4. Low priority: `dai` already has the working fix in its own tree, so
   nothing is currently broken anywhere — this is purely about not making
   the next project pay the same discovery cost.
