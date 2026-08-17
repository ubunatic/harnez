# Voice Input

`harnez tools` is an explicit boundary for optional operating-system capabilities. It is
separate from global harness `apply` and project `init`; neither installs packages or
services.

## Current MVP

- `harnez tools` performs a read-only listing and always succeeds when the catalog loads.
- `harnez tools status [voice-input]` performs read-only probes; a named capability exits
  nonzero unless it is ready, making it suitable for scripts.
- `harnez tools install voice-input --dry-run` prints payload/model sizes, destinations,
  privileges, persistent changes, injection choice, and privacy implications.
- User scope is the default and never requires `sudo`; system scope is explicit.
- The Voxtype 0.7.5 AVX2 and RPM artifacts and their official GitHub release SHA-256
  digests are pinned in a strictly decoded embedded YAML catalog with a companion JSON Schema.
- A checksum verifier is implemented for the future user-local recipe. It installs atomically
  only when the destination is absent, returns no-change for identical bytes, and refuses to
  overwrite any other existing file. The gated command does not invoke it yet.
- The Fedora 44 GNOME Wayland canary fully passed: microphone capture, local `base.en`
  transcription, the global GNOME toggle shortcut, and direct text injection at the
  focused cursor (correct on a German QWERTZ layout) via a user-level `dotool`+`dotoold`
  daemon — not `eitype` (portal-dialog authorization) or `ydotool` (no XKB awareness).

## Commands

```text
harnez tools
harnez tools status voice-input
harnez tools install voice-input --dry-run
harnez tools install voice-input --scope user     # default; recipe pending
harnez tools install voice-input --scope system   # explicit privilege; recipe pending
```

The command never uses cloud transcription or requests membership in the `input` group.
There is no uninstall command until ownership records can distinguish harnez-managed
files from user-owned files.

## Security note: uinput access is not gated by voice input

`dotool` (and `ydotool`) write to `/dev/uinput` to synthesize keyboard input. On this
workstation `/dev/uinput` already carries a `udev` `uaccess` tag, which is systemd-logind's
standard mechanism for granting the active local desktop session read/write access to
input devices (the same mechanism used for `/dev/dri`, `/dev/snd`, webcams, Steam Input,
and accessibility tools). This means **any process running as the logged-in user already
has direct, silent, kernel-level keyboard/mouse injection capability, independent of
whether voice input or its typing backend is installed.** A start/stop toggle for
`dotoold` (e.g. a GNOME Quick Settings button) would not close this: `systemctl --user`
requires no privilege beyond the same user account, so anything that could abuse the
running daemon could equally re-enable it or bypass it via `/dev/uinput` directly. Do not
build or ship such a toggle as a security control; it provides no real boundary and only
gives false comfort. The one mechanism here that requires genuine per-use human consent is
`eitype`, which routes through Wayland's XDG RemoteDesktop portal — rejected in this setup
because of the recurring authorization dialog, a deliberate convenience-over-consent
tradeoff the user accepted knowingly. Meaningfully closing this gap would require removing
the `uaccess` tag from `/dev/uinput` system-wide, which is out of scope for `harnez tools`
and would break other legitimate uses (Steam Input, accessibility tools) unless done
carefully.

## Hardware canary

The Fedora 44 GNOME Wayland canary fully passed as of 2026-08-17. Microphone capture,
local transcription, the `Super+Ctrl+X` toggle, and direct text injection (verified in
a terminal and in Prime Agent's own input field) all work. GNOME text injection needed
a user systemd `dotoold` daemon (`DOTOOL_XKB_LAYOUT=de`) for the fast, reliable
`dotoolc` path, plus `language_to_layout = {}` in Voxtype's config to stop it
auto-forcing `layout=us` for English speech regardless of the physical keyboard layout.
The script can repeat the guided manual test; ordinary `go test` never runs it.

## Streaming (opt-in, research spike — see issue 021)

Local streaming partial-typing (Parakeet via ONNX Runtime, no GPU required) was proven
working on this same workstation as of 2026-08-17: words appear incrementally during
dictation via the existing `dotoolc` fast path, still fully offline. It is **not** the
default and is **not** wired into `harnez tools` — it requires a separate
~2.7GB one-time download, a second config file
(`~/.config/voxtype/config-streaming.toml`) that is never auto-loaded, and manually
stopping `voxtype.service` before starting the streaming daemon (both bind the same
runtime socket). See `scripts/canary-voice-streaming.sh` and issue 021's Findings
section for the full setup, the footguns hit along the way (a `voxtype setup
--download` side effect that silently switches the live engine, an undocumented
streaming-timing constraint, and a PATH gap in ad hoc systemd units), and what's still
open (Enter-to-stop, longer-form backtracking behavior).
