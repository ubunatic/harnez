# 020 — `harnez tools`: guided OS-level tool installation

**Status:** Open — GNOME voice-input canary passed; installer implementation pending

## Context

`apply` manages global agent harness files and `init` manages project files. Neither
command should install operating-system packages or services. We need a separate,
explicit path for optional workstation capabilities, starting with system-wide voice
input: speak from any application and insert the local transcription at the focused
cursor.

Omarchy 4.0.0 is a useful current reference because it ships this as an optional OS-level
feature rather than embedding speech recognition into each application.

## How Omarchy 4.0.0 does it

Omarchy delegates dictation to [Voxtype](https://github.com/peteonrails/voxtype) and
only orchestrates the desktop integration:

1. A one-time notification invites the user to install dictation; installation is not
   automatic.
2. The confirmed installer adds the Arch packages `wtype` and `voxtype-bin`.
3. It copies a Voxtype config with local `base.en` Whisper, the default microphone,
   direct typing with clipboard fallback, and Voxtype's own hotkey disabled.
4. `voxtype setup --download --no-post-install` downloads the roughly 150 MB model.
5. Vulkan acceleration is enabled when Omarchy's hardware probe succeeds; failure is
   non-fatal.
6. `voxtype setup systemd` installs/enables the user service.
7. Hyprland owns the global shortcuts, conditionally binding them only when `voxtype`
   exists: hold `F9` for push-to-talk, or toggle with `Super+Ctrl+X`.
8. The shell follows `voxtype status --follow` to show recording/transcribing state.
   Removal disables the user service, removes the package/config/model, and reloads
   Hyprland.

The useful boundary is: **Voxtype owns audio, local inference, models, and its user
service; the desktop owns shortcuts and status; the installer only converges setup.**
Harnez should copy that boundary, not implement speech recognition.

## Local canary findings

This workstation is Fedora 44, GNOME, Wayland. `wtype`, `wl-copy`, PipeWire/Pulse tools,
and systemd are present; `voxtype` is absent and Fedora's repositories do not provide it.
Upstream publishes a Fedora RPM plus `SHA256SUMS.txt` and detached signatures.

A portability trap must be resolved before implementation: `wtype` works on wlroots
compositors such as Hyprland and Sway, but not GNOME/KDE Wayland. Voxtype recommends
`eitype` (libei) for GNOME/KDE, then `dotool`/`ydotool`, with clipboard as the final
fallback. Its built-in evdev hotkey also requires membership in the privileged `input`
group and a logout/login; a GNOME custom shortcut can instead call
`voxtype record toggle` without that group access.

Prefer a no-sudo canary first: install the signed/checksummed upstream AVX2 binary to
`~/.local/bin`, install `eitype` into the user's Cargo bin directory if needed, use a
GNOME custom toggle shortcut, and generate a systemd **user** service. This avoids both
an RPM transaction and `input`-group membership. Escalate only if a required runtime is
missing or the user explicitly selects a system package install. Fingerprint-authorized
`sudo` is acceptable for those disclosed steps, but it is not the default route.

The live Fedora 44 GNOME Wayland canary passed on 2026-08-17:

- Voxtype captured the default PipeWire microphone,
- local `base.en` transcription completed successfully,
- `eitype` injected Unicode text into the focused application,
- a GNOME `Super+Ctrl+X` shortcut toggled recording globally.

The working route uses Voxtype's built-in evdev hotkey disabled, so it does not require
`input`-group membership. GNOME custom shortcuts provide press-only activation, hence
toggle mode rather than push-to-talk. Push-to-talk remains possible through Voxtype's
built-in evdev hotkey, but requires `input`-group access and logout/login.

## Proposed command

Keep OS mutation separate from `apply` and `init`:

```text
harnez tools                              # read-only list/status
harnez tools status [voice-input]         # no sudo, network, or writes
harnez tools install voice-input          # show plan, confirm, converge
harnez tools install voice-input --dry-run
harnez tools install voice-input --scope user    # default; avoid sudo
harnez tools install voice-input --scope system  # verified RPM; may use sudo
harnez tools install voice-input -y        # explicit noninteractive approval
```

`voice-input` is the stable capability name; Voxtype is its initial provider. A bare
`harnez tools` must remain read-only. Do not add installation to `harnez apply`.

The old, unimplemented `PLAN.md` Phase 5 `deps` command should be superseded by this
command so harnez does not grow two package-install mechanisms.

## Lightweight implementation

- Add an embedded tool catalog, initially containing only `voice-input`.
- Put package names, provider version, artifact patterns/checksums, supported platforms,
  and readiness probes in a small YAML tool spec. Go interprets typed actions; the YAML
  must not contain arbitrary shell snippets.
- Keep provider-specific orchestration behind a small interface rather than building a
  universal package manager. Support Fedora 44 GNOME/Wayland first after the canary;
  report other combinations as unsupported until tested.
- Default to a user-local install: resolve a pinned Voxtype release, download the
  architecture-specific official binary to a temporary file, verify its pinned SHA-256,
  then atomically install it under `~/.local/bin`. Never use `curl | sh` or execute an
  unverified download.
- On GNOME Wayland, prefer user-local `eitype` plus a GNOME custom toggle shortcut. Do
  not add the user to `input` merely to obtain a global hotkey.
- Offer the verified upstream RPM through `dnf` only as an explicit system-install mode
  or when the no-sudo route cannot satisfy a dependency. Keep each `sudo` command
  isolated so normal PAM/fingerprint authorization can approve it.
- Use privilege only for explicitly approved OS packages or group membership. Model
  download, config, shortcut, and systemd user-service setup run as the user.
- Let Voxtype create/manage its config and service where possible. Harnez should write
  only explicitly managed settings and preserve user customizations.
- Print payload size, model size, destinations, required privileges, services, keyboard
  injection choice, and whether logout/login is required before confirmation.
- Treat GPU setup as an optional, separately reported optimization; CPU transcription
  is the reliable baseline.

Suggested tree:

```text
internal/tools/               # planner, probes, runner, voice-input provider
spec/tools/voice-input.yaml   # declarative metadata and platform recipe
```

The tool payload is not lightweight (current RPMs bundle multiple backends and are
hundreds of MB); the **harnez install path** should be lightweight, auditable, and easy
to decline.

## Desired-state flow

```text
harnez tools install voice-input
        │
        ├── detect OS, architecture, desktop/session, audio, GPU
        ├── build and print an exact action plan
        ├── confirm (unless -y); stop here for --dry-run
        ├── install verified user-local provider binary (sudo fallback is explicit)
        ├── create/download the selected local model
        ├── configure tested GNOME text injection and global toggle shortcut
        ├── enable/start the Voxtype systemd user service
        ├── run readiness probes
        └── report ready, pending logout, or partial failure
```

Every action needs a probe. A second install on a ready machine must print `No changes.`
A partial failure must list completed and pending actions rather than claiming a
transactional rollback.

## Safety and privacy

- Default to local/offline transcription; do not enable cloud providers or telemetry.
- Do not retain recordings beyond Voxtype's normal transient processing.
- Explain microphone access and synthetic keyboard-input implications.
- Fail closed on unsupported distro/session/architecture combinations.
- `--dry-run` must perform no network download, sudo invocation, service change, group
  change, or filesystem write.
- Do not implement uninstall until harnez records ownership (for example under
  `$XDG_STATE_HOME/harnez/tools/`) and can distinguish its files/packages from the
  user's.

## Acceptance criteria

- `harnez tools` reports `voice-input` as ready, missing, partial, or unsupported.
- Fedora/GNOME defaults to a pinned, checksum-verified user-local Voxtype binary and
  requires no sudo when existing audio/session dependencies are sufficient.
- A system RPM path is separately selectable and uses a verified official artifact.
- The plan and confirmation expose every privileged or persistent action.
- The service is active after login and local dictation reaches the focused application
  through a proven GNOME Wayland backend.
- A global shortcut starts/stops recording without application-specific setup.
- Re-running install is idempotent and preserves user-owned configuration.
- Unit tests use injected runner/filesystem/network/prompt interfaces; they never call
  real `sudo`, `dnf`, audio hardware, or the network.
- Tests cover unsupported systems, refusal, `-y`, mutation-free dry-run, checksum
  mismatch, partial failure, unmanaged-file preservation, and second-run idempotency.
- A separate opt-in hardware smoke test covers microphone → transcription → focused
  text; it is never part of ordinary `go test`.

## Non-goals for the first version

- Installing tools implicitly from `apply` or `init`.
- Supporting every distro, compositor, speech engine, or GPU backend.
- A generic arbitrary-command installer DSL.
- Cloud transcription, LLM cleanup, meeting transcription, shell status widgets, or an
  uninstall command.

## Sources

- [Omarchy 4.0.0 release](https://github.com/basecamp/omarchy/releases/tag/v4.0.0)
- [Omarchy Voxtype installer](https://github.com/basecamp/omarchy/blob/v4.0.0/bin/omarchy-voxtype-install)
- [Omarchy Hyprland bindings](https://github.com/basecamp/omarchy/blob/v4.0.0/default/hypr/bindings/voxtype.lua)
- [Omarchy Voxtype defaults](https://github.com/basecamp/omarchy/blob/v4.0.0/default/voxtype/config.toml)
- [Omarchy dictation manual](https://github.com/basecamp/omarchy/blob/v4.0.0/manual/11-text-extraction-dictation.md)
- [Voxtype installation guide](https://github.com/peteonrails/voxtype/blob/main/docs/INSTALL.md)
- [Voxtype GNOME/KDE output guidance](https://github.com/peteonrails/voxtype/blob/main/docs/USER_MANUAL.md#output-modes)
