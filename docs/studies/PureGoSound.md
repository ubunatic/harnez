# Pure-Go Audio Playback Assessment for CLI Notifications

Assessment of pure-Go (no CGo) audio playback architectures for notification sounds in Harnez (evaluating Issue 641 against the external research report and the existing Issue 640 subprocess player chain).

## Options Comparison

| Platform | Approach | CGo | Maintenance | Effort | Binary Size | Failure Modes & Trade-offs |
|:---|:---|:---:|:---|:---:|:---:|:---|
| **macOS & Linux** | **Subprocess Chain** (Harnez 640: `pw-play`, `paplay`, `canberra-gtk-play`, `aplay`, `afplay`) | No | Zero (OS system utilities) | S | 0 KB | Missing utility on bare-bones Linux; requires system audio file or disk asset. Fast fail via `LookPath`; clean isolation via process-group `SIGKILL` on timeout. |
| **Linux** | **Native Socket Client** (`github.com/jfreymuth/pulse` + embedded WAV) | No | Low / Active (external lib) | M (~200 LOC) | ~350–450 KB | Fails if PulseAudio/PipeWire socket is absent (e.g. bare ALSA or minimal containers). Requires network dial timeouts. |
| **Linux** | **In-Tree Native Socket Client** (custom PA v32 protocol implementation) | No | High (bespoke maintenance) | L (~1k LOC) | ~150–250 KB | Significant maintenance burden (cookie parsing, stream auth, sample specs, drain handshake). Fails without PA/PipeWire socket. |
| **macOS & Linux** | **Game Audio Framework** (`github.com/ebitengine/oto` v3 + embedded WAV) | No | Medium (heavy third-party stack) | M (~200 LOC) | ~900–1,400 KB | Heavyweight runtime (background mixing goroutines, ring buffers). On Linux falls back to `purego` dynamic loading of `libasound.so.2` if PA absent; deadlock risk on unready channels. |
| **macOS** | **In-Process FFI** (`purego` + `AudioToolbox` / `AudioServicesPlayAlertSound`) | No | Low / Active (external lib) | S (~100 LOC) | ~180–260 KB | In-process dynamic symbol resolution. Custom sounds require writing to a temporary file for `CFURLRef`; limited to <5s alert sounds. |
| **Linux** | **Direct Kernel ALSA** (`github.com/yobert/alsa` via `/dev/snd` ioctls) | No | Abandoned upstream | L (Complex) | ~200–300 KB | Fails with `EBUSY` when PipeWire/PulseAudio holds hardware; lacks userspace software mixing (dmix) and sample rate conversion; unmapped in containers. |

---

## Audio Asset Packaging Economics (WAV vs Ogg Vorbis)

- **Uncompressed PCM / WAV (22.05 kHz, 16-bit Mono, 1.0s)**: ~44 KB embedded slice. Requires only ~30 lines of Go to validate the 44-byte RIFF header and stream raw PCM.
- **Compressed Ogg Vorbis (.oga, ~64 kbps, 1.0s)**: ~8 KB audio data, but requires bundling a pure-Go Vorbis decoder (e.g. `jfreymuth/oggvorbis`), adding ~160–220 KB of compiled machine code (IMDCT, floor decoding, codebooks).
- **Conclusion**: Embedding uncompressed WAV achieves a smaller net binary footprint (~44 KB net vs ~170–230 KB net) and zero CPU decoding overhead during CLI shutdown.

---

## Strategic Recommendation

**Recommendation: Keep the Issue 640 Subprocess Player Fallback Chain.**

### Deciding Reasons

1. **Zero Binary Overhead & Dependency Footprint**:
   Harnez ships 0 KB added binary size and 0 external third-party audio dependencies. Adding `jfreymuth/pulse` (~400 KB) or `ebitengine/oto` (~1.2 MB) imposes substantial binary bloat for a transient hook notification.
2. **Superior Isolation & Hang Protection**:
   Subprocess invocation with `exec.CommandContext`, `Setpgid: true`, and `syscall.Kill(-pid, SIGKILL)` on timeout (implemented in `internal/sound/play.go`) isolates Harnez from audio daemon lockups, ALSA device stalls, driver crashes, and dynamic library linking faults.
3. **Headless & CI Resilience**:
   In headless servers, Docker containers, and SSH sessions lacking audio infrastructure, `exec.LookPath` and quick-fail subprocesses exit in <1 ms without socket dial timeouts or runtime warnings.
4. **Platform Ubiquity**:
   macOS ships `/usr/bin/afplay` and `/System/Library/Sounds/Glass.aiff` on 100% of installations. Modern Linux desktop environments ship PipeWire/PulseAudio utilities (`pw-play`, `paplay`) or fallback players (`canberra-gtk-play`, `aplay`, `ffplay`) and standard freedesktop sound themes.

---

## Assessment of External Research & Doubtful Claims

1. **`oto` CGo Requirement (Clarified)**:
   The ticket question asked whether `oto` requires CGo for ALSA on Linux. The research correctly identifies that modern `oto` v3 (v3.0+) no longer requires CGo at build time—it uses `jfreymuth/pulse` for PulseAudio and dynamically loads `libasound.so.2` via `purego` for ALSA. However, `oto` introduces excessive runtime weight for simple CLI alert chimes.
2. **Version Tag Precision (Doubtful / Unverified)**:
   The report references `ebitengine/oto v3.5+` specifically. The transition to purego/pure-Go PulseAudio occurred with the `oto/v3` major release; specific minor release tag numbering in the report could not be verified against upstream release notes.
3. **macOS In-Process purego Utility (Overstated Benefit)**:
   The research proposes a hybrid approach using in-process `purego` on macOS. However, custom sound playback through `AudioServicesCreateSystemSoundID` still requires a file path on disk (`CFURLRef`), negating any memory-only playback advantage while introducing dynamic FFI loading risks.
4. **Hybrid Approach Value (Rejected)**:
   The external report recommends a hybrid strategy (pure-Go socket for Linux, subprocess for macOS). For Harnez CLI, this adds unnecessary dependency and configuration complexity without tangible user-facing benefit over the existing configurable fallback chain in `spec/sound.yaml`.
