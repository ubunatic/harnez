# Fluent, streaming voice-input typing with enter-to-stop

**Status:** Research spike complete — local streaming proven viable and working on this workstation (opt-in, not shipped as a `harnez tools` feature). See Findings below.

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


## Findings (2026-08-17 canary session on this workstation)

**Status:** Streaming partial typing (part 1) proven working end-to-end, opt-in,
local-only. Backtracking (part 2) confirmed already implemented by Voxtype, not
built here. Enter-to-stop (part 3) investigated at the API level only, not
prototyped. Research spike, not a shipped `harnez tools` feature yet.

### Part 1 — local streaming is viable, proven on this hardware

- Hardware: AMD Ryzen 5 PRO 5650U (Zen 3), AVX2 present, no AVX-512, no NVIDIA GPU
  (`nvidia-smi` absent; iGPU is AMD Vega via `lspci`). Parakeet via ONNX Runtime is
  CPU-only and does not need a GPU — confirmed both from Voxtype's `PARAKEET.md`
  and by actually running it here.
- Installed `voxtype-0.7.5-linux-x86_64-onnx-avx2` (SHA-256 verified against the
  release's `SHA256SUMS.txt`) in place of the plain `avx2` binary at
  `~/.local/bin/voxtype`, after backing up the previous binary to
  `~/.local/bin/voxtype.avx2.bak-021`. This binary is a strict superset —
  `whisper-rs` is an unconditional Cargo dependency, not feature-gated — so the
  swap alone does not change the default batch `base.en` behavior. Verified by
  restarting `voxtype.service` unchanged and confirming the journal shows the same
  `ggml-base.en.bin` load path with no config changes.
- Downloaded the streaming-capable model (`parakeet-unified-en-0.6b`, ~2.66GB,
  `bobNight/parakeet-unified-en-0.6b-onnx` via Voxtype's own sha256-verified R2
  mirror) with `voxtype setup --download --model parakeet-unified-en-0.6b --quiet`.
  Not all Parakeet models support streaming — Voxtype's source
  (`src/setup/model.rs`) explicitly marks only this one `streaming_compatible: true`
  among 5 registered Parakeet variants; the docs-page default
  (`parakeet-tdt-0.6b-v3`) is NOT streaming-capable and fails at first-chunk
  inference if you try.
- **Footgun found and worked around:** `voxtype setup --download --model ... --quiet`
  silently rewrote the live `~/.config/voxtype/config.toml`, setting
  `engine = "parakeet"` and adding a `[parakeet]` block — i.e. the model-download
  helper has a side effect of switching the active engine, exactly the kind of
  unsolicited local-to-something-else change this project avoids. Caught and
  reverted immediately (confirmed `voxtype.service` reloads `base.en` cleanly after
  revert). All streaming configuration now lives in a separate,
  never-auto-loaded `~/.config/voxtype/config-streaming.toml`, started manually
  with `voxtype -c ~/.config/voxtype/config-streaming.toml daemon`. This is not
  wired into `harnez tools` or any systemd unit that starts automatically.
- **Footgun found and worked around:** Voxtype's own documented default streaming
  timing knobs (`streaming_chunk_secs = 0.5`, `streaming_left_context_secs = 1.5`,
  `streaming_right_context_secs = 0.5`) are rejected at daemon startup by
  `parakeet-rs` with "must map to a mel-frame count divisible by 8" (at 100
  frames/sec, values must be multiples of 0.08s). Adjusted to the nearest valid
  values: chunk=0.48s, left=1.6s, right=0.48s. Not documented anywhere in
  Voxtype's `CONFIGURATION.md`/`PARAKEET.md` as of 0.7.5.
- **Guided live test result (human-in-the-loop, real microphone, real GNOME
  session):** with the daemon started via a transient `systemd-run --user` unit
  carrying the same `PATH=/home/uwe/.local/bin:...` override that
  `voxtype.service`/`dotoold.service` set explicitly (dotool lives outside
  systemd's default PATH — a second footgun that produced silent
  clipboard-only fallback with zero visible output until diagnosed via `-v` debug
  logs and `journalctl`), a real dictation pass produced `Text typed via dotoolc`
  log lines and — per the user — dictated text "works with 2-3s delay",
  confirming: local Parakeet streaming, typed incrementally through the existing
  `dotoolc` fast path, using the same GNOME `Super+Ctrl+X` shortcut from issue 020
  unmodified (it talks to whichever daemon owns the runtime socket).
- Net verdict: **local streaming typing works on this workstation today**, fully
  offline (no cloud, no API key). It requires ~2.7GB of additional disk (binary +
  model), a separate config file, and manual daemon start/stop — not yet a
  one-command `harnez tools` experience. No sign-off needed for cloud use since
  none was used; Soniox was not touched.

### Part 2 — backtracking already exists in Voxtype, no new logic needed

Read `src/output/streaming.rs` (0.7.5) directly, as instructed, before designing
anything:

- `StreamingSession::commit_segment` types finalized text.
- `StreamingSession::replace_and_commit` backspaces a caller-given character count
  then types replacement text — this is exactly the backspace-and-retype
  correction machinery the issue speculated might exist. It is driven by a
  `StreamingEvent::Replace { backspace, text }` event from the transcriber.
- `StreamingSession::rewind` (plus `emit_backspaces` trying `wtype` →
  `dotoolc`/`dotool` → `ydotool` in order) handles full-session cancel, counting in
  Unicode scalar values (not bytes) so multi-byte UTF-8 characters backspace
  correctly.
- Voxtype's own default policy is **commit-only typing**: only `Final` segments are
  guaranteed non-revised; raw `Partial` events are typed too via
  `type_partial_delta`, but the code comments explicitly document that
  revision-style backends (unlike Parakeet) would need `type_partials`-style gating
  to avoid visible churn — this exists today only for Soniox
  (`[soniox] type_partials`), not Parakeet.
- For the specific Parakeet streaming pipeline used here
  (`src/transcribe/parakeet_streaming.rs`), there are two contradictory doc
  comments in the 0.7.5 source about whether `Partial` events carry a delta or a
  cumulative transcript. This was flagged before empirical testing (per the
  Canary-first convention) and resolved by observation: the debug log for a 4-word
  test phrase showed 5 separate `dotoolc` calls of increasing-but-small char counts
  (6/6/5/1/9), consistent with word-sized deltas being typed directly — matching
  `streaming.rs`'s "delta, not cumulative" comment, not the `parakeet_streaming.rs`
  module doc's "cumulative" claim. **Conclusion: it's a delta, the
  `parakeet_streaming.rs` module doc comment is stale/wrong.** No duplicate-text
  bug was observed in the guided test.
- **Net verdict: nothing to build for part 2.** If/when this ships, the work is
  "expose Voxtype's existing streaming config", not new backtracking logic.

### Part 3 — Enter-to-stop: investigated, not prototyped

- GNOME Shell's `Shell.Introspect.GetWindows` D-Bus method is access-denied to
  external callers by default (confirmed empirically: `AccessDenied: GetWindows is
  not allowed`), and `Shell.Eval` is likewise unavailable outside unsafe/dev mode —
  this workstation's GNOME session is properly locked down, which is good security
  posture but means window/focus introspection from outside the compositor process
  is not viable without the user separately enabling unsafe mode.
- The suggested approach (a GNOME Shell extension using `Meta`/`Clutter` key-event
  hooks, running in-process inside `gnome-shell`/Mutter) remains architecturally
  sound and was not contradicted by anything found here: GNOME Shell/Mutter is
  itself the privileged compositor process with its own seat/logind-granted input
  access, categorically different from a userspace `evdev` reader needing
  `input`-group `uaccess`. An extension could use `Main.wm.addKeybinding`/
  `removeKeybinding` to dynamically grab a bare `Return` accelerator *only* while
  Voxtype's state file shows `recording`/`streaming`, and release the grab
  immediately after — satisfying the "must not swallow ordinary Enter when
  dictation is inactive" requirement.
- **Not prototyped in this session** — writing, loading, and testing an actual
  GNOME Shell extension is real additional work and this session's priority was
  the streaming/backtracking questions per the parent's explicit request to check
  in "before spending significant time on parts 2-4" once part 1's viability was
  known. Deferred to a follow-up session/issue if streaming typing itself gets
  productized.

### What's proven vs. uncertain vs. rejected

- **Proven (real hardware, human-in-the-loop):** local Parakeet ONNX streaming
  boots, transcribes, and types incrementally via the existing `dotoolc` fast path
  on this exact workstation; the default batch `base.en` flow is unaffected and
  was re-verified after every mutation.
- **Proven (source read, cross-checked against live debug logs):** Voxtype already
  implements backspace-and-retype correction machinery; Parakeet's partials here
  are deltas, not cumulative — no duplicate-text bug.
- **Uncertain / not yet exercised:** actual mid-word backtracking/correction
  behavior in a longer, more ambiguous dictation (the guided tests used short,
  clear phrases that likely didn't trigger any `Replace` events); exact
  word-boundary spacing correctness beyond the user's informal "works" report;
  whether the 2-3s latency is warm-up-only or a steady-state characteristic (not
  independently confirmed against the debug log's own ~0.5-1.5s inter-chunk
  timing).
- **Rejected / explicitly not done:** Soniox or any cloud engine (never touched);
  `input`-group membership or any other privilege expansion (never requested);
  making Parakeet/streaming the default (kept strictly opt-in via a separate
  config file and manual daemon invocation, not wired into `voxtype.service` or
  `harnez tools`).

### Reusable artifacts from this session

- `scripts/canary-voice-streaming.sh` — guided manual canary for the opt-in
  streaming path, mirroring `scripts/canary-voice-input.sh`'s pattern.
- `~/.local/bin/voxtype.avx2.bak-021` — pre-swap binary backup (host state, not
  committed to the repo).
- `~/.config/voxtype/config-streaming.toml` — opt-in streaming config (host
  state, not committed; not auto-loaded by any systemd unit).

### Follow-ups if this becomes a real feature

1. Decide whether `harnez tools` should manage the streaming opt-in (a second
   systemd unit + config, mutually exclusive with the batch one since both bind
   the same runtime socket) or leave it a manual/documented procedure.
2. File upstream findings against Voxtype: the `--download`/config-mutation side
   effect, the undocumented mel-frame-divisible-by-8 constraint on streaming
   timing knobs, and the stale delta-vs-cumulative doc comment in
   `parakeet_streaming.rs`.
3. Prototype the GNOME Shell extension for Enter-to-stop before considering
   `input`-group membership.
4. Re-test backtracking specifically with ambiguous/homophone-heavy speech to
   observe a real `StreamingEvent::Replace`.
