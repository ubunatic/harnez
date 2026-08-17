# Fluent, streaming voice-input typing with enter-to-stop

**Status:** Open — research needed, not yet started

## Context

Issue 020 established a working `harnez tools install voice-input` canary: press
`Super+Ctrl+X` to toggle recording, speak, press `Super+Ctrl+X` again, and the full
utterance is transcribed and typed at once via a persistent `dotool`/`dotoold` daemon
(see `docs/VoiceInput.md`). This is a batch experience: nothing appears until the whole
recording is transcribed, and the user must remember to press the toggle shortcut again
to stop.

This issue tracks making dictation feel more like fluent typing:

1. Type words as they are transcribed, not all at once at the end.
2. Optionally backtrack a short distance to correct earlier words once later context
   disambiguates them (e.g. homophones, wrong word boundaries), with a small delay
   before committing text so corrections can still land before the user reads it.
3. Once (1)+(2) work, stop requiring the `Super+Ctrl+X` toggle to end a dictation
   turn — instead, detect an Enter keypress from the user while dictation is active and
   treat it as "stop and submit," similar to a chat input's natural Enter-to-send.

## 1. Streaming partial typing

Voxtype's local default backend (`whisper` `base.en`) transcribes complete utterances in
one batch pass; it does not emit partial/incremental tokens as speech happens. Voxtype's
`docs/CONFIGURATION.md` describes true incremental output only for streaming-capable
backends (Parakeet, Soniox), which call the output driver many times per session — once
per partial token batch. That document also explains why the `dotoold`/`dotoolc` fast
path exists at all: cold `dotool` pays a ~700-800ms uinput device registration cost per
call, which is fine once per utterance but unusable at dozens of partials per session
(40+ seconds of cumulative latency). We already run `dotoold` for the batch case (issue
020); it becomes a hard requirement, not just an optimization, for streaming.

Open questions to research before implementing:

- Does a local (non-cloud) streaming-capable backend exist that runs acceptably on this
  hardware (no GPU), or does streaming require Soniox (cloud, API key) or a GPU-backed
  Parakeet model? Voice input's stated goal is local-only, offline transcription; a
  streaming backend that requires the cloud would need an explicit, separate opt-in and
  should not become the default.
- What does Voxtype consider a "partial"? Confirm whether partials are stable prefixes
  (safe to type immediately) or provisional and frequently revised (need the
  backtracking behavior in the next section regardless).
- Confirm the true per-partial output latency in practice on this workstation with
  `dotoolc`, since the sub-10ms figure in Voxtype's docs is what makes streaming typing
  viable at all.

## 2. Correction via backtracking

Voxtype's configuration docs mention that `dotool` streaming calls "clobber the
held-key state tracker" affecting push-to-talk release handling — this hints Voxtype
already has some internal machinery for revising previously-typed streaming text (likely
backspace-and-retype for a changed suffix). This needs direct source investigation
(`src/output/streaming.rs`) before designing anything new:

- Does Voxtype already implement correction via backspace-and-retype for revised
  partials? If so, this issue may reduce to "expose/tune existing behavior" rather than
  building new logic.
- If not, a naive implementation must: buffer a short trailing window of already-typed
  words, hold them uncommitted for a small delay (order of a few hundred ms) so
  Whisper's context can stabilize, then either commit as-is or backspace and retype the
  revised words. This must be careful never to touch text the user has typed themselves
  outside the dictation session, or text older than the buffered window, else it will
  corrupt the target document.

## 3. Enter-to-stop

Today, ending a dictation turn requires physically pressing the same `Super+Ctrl+X`
shortcut used to start it, because the GNOME custom-shortcut route we chose in issue
020 deliberately avoids requiring `input`-group membership (Voxtype's built-in evdev
hotkey needs it). Detecting an ordinary Enter keypress *while dictation is active* to
auto-stop and submit requires watching global keyboard events, which is exactly the
`evdev`-reading capability we avoided.

This needs a design that does not silently reintroduce the `input`-group requirement
without the user choosing it explicitly:

- If the user is willing to opt into `input`-group membership for this feature only
  (separately from the deferred F9 push-to-talk work), Voxtype's own built-in evdev
  hotkey path already reads raw keyboard events and could plausibly be extended or
  companion-scripted to also watch for Enter without a second, redundant evdev reader.
- Investigate whether a GNOME Shell extension can observe key events for this purpose
  without `input`-group access, e.g. via `Clutter`/`Meta` key-event hooks available to
  in-process Shell code (extensions run inside the compositor and may see events through
  a different, already-privileged path than a user-space `evdev` reader). This would be
  the preferred route if it works, since it avoids a new privileged group grant. Treat
  this as the first thing to prototype since it may make the `input`-group question in
  issue 020's F9 discussion moot for this specific need.
- Whatever mechanism is chosen, it must not swallow ordinary Enter presses meant for the
  target application when dictation is *not* active — the watcher must be scoped tightly
  to the active-dictation window.

## Relationship to issue 022

The streaming/backtracking pieces here are independent of the GNOME transcriber UI in
issue 022, but issue 022's "retype" button assumes text can be typed on demand outside
the original dictation turn — validate that the typing primitive built here (or in issue
020) is reusable for that purpose rather than duplicating it.

## Non-goals

- No cloud transcription becomes a default; a streaming backend that requires network
  access must be a clearly-labeled, explicit opt-in, never silently enabled.
- No silent expansion of privilege (e.g. `input`-group membership) without the user
  explicitly choosing it for this feature.
