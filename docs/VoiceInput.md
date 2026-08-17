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
- The Fedora 44 GNOME Wayland hardware canary passed with the default PipeWire
  microphone, local `base.en`, Unicode injection through `eitype`, and a global GNOME
  toggle shortcut. The command still stops before installation because the converging
  recipe has not been implemented yet.

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

## Hardware canary

The canary passed on Fedora 44 GNOME Wayland on 2026-08-17. The verified setup uses a
GNOME `Super+Ctrl+X` toggle shortcut with Voxtype's evdev hotkey disabled, avoiding
`input`-group membership. `scripts/canary-voice-input.sh` can repeat the guided manual
test; it does not itself install or change settings. Ordinary `go test` never runs it.
