package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// Voice-input streaming/batch modes run as two mutually exclusive systemd
// user services (they share the same runtime socket, so only one can hold
// it at a time). BatchService is installed by issue 020; StreamingService
// is an opt-in unit pointed at the streaming config, documented in
// docs/VoiceInput.md and issue 021.
const (
	BatchService     = "voxtype.service"
	StreamingService = "voxtype-streaming.service"

	streamingConfigRelPath = ".config/voxtype/config-streaming.toml"
	streamingModelRelPath  = ".local/share/voxtype/models/parakeet-unified-en-0.6b"
)

// VoiceInputMode is a stable identifier for the active voice-input mode.
type VoiceInputMode string

const (
	ModeBatch        VoiceInputMode = "batch"
	ModeStreaming    VoiceInputMode = "streaming"
	ModeNeither      VoiceInputMode = "neither"
	ModeInconsistent VoiceInputMode = "inconsistent"
)

func serviceActive(ctx context.Context, d Dependencies, service string) bool {
	return d.Run(ctx, "systemctl", "--user", "is-active", "--quiet", service) == nil
}

// CurrentVoiceInputMode reports which voice-input service, if any, is
// currently active. It never mutates the host.
func CurrentVoiceInputMode(ctx context.Context, d Dependencies) VoiceInputMode {
	batch := serviceActive(ctx, d, BatchService)
	streaming := serviceActive(ctx, d, StreamingService)
	switch {
	case batch && streaming:
		return ModeInconsistent
	case batch:
		return ModeBatch
	case streaming:
		return ModeStreaming
	default:
		return ModeNeither
	}
}

// DescribeVoiceInputMode renders CurrentVoiceInputMode for terminal output.
func DescribeVoiceInputMode(m VoiceInputMode) string {
	switch m {
	case ModeBatch:
		return "batch (voxtype.service active, base.en whisper, typed at end of utterance)"
	case ModeStreaming:
		return "streaming (voxtype-streaming.service active, local Parakeet, typed incrementally)"
	case ModeNeither:
		return "neither (both services stopped)"
	case ModeInconsistent:
		return "inconsistent (both voxtype.service and voxtype-streaming.service are active; stop one manually)"
	default:
		return string(m)
	}
}

// checkStreamingPreconditions validates that the opt-in streaming config and
// model are present before attempting to switch into streaming mode, so a
// failed switch gives an actionable error instead of a service that fails to
// start.
func checkStreamingPreconditions(d Dependencies) error {
	home := d.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("cannot resolve $HOME to locate voice-input streaming config")
	}
	configPath := filepath.Join(home, streamingConfigRelPath)
	if _, err := d.Stat(configPath); err != nil {
		return fmt.Errorf("streaming config missing at %s (see docs/VoiceInput.md streaming section): %w", configPath, err)
	}
	modelDir := filepath.Join(home, streamingModelRelPath)
	info, err := d.Stat(modelDir)
	if err != nil {
		return fmt.Errorf("streaming model missing at %s; run: voxtype setup --download --model parakeet-unified-en-0.6b --quiet: %w", modelDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", modelDir)
	}
	return nil
}

// waitActive polls is-active for up to ~10s (Parakeet's streaming model load
// takes a few seconds; base.en whisper loads in well under a second), so a
// slow-but-successful start isn't mistaken for a failure.
func waitActive(ctx context.Context, d Dependencies, service string) bool {
	const attempts = 20
	for i := 0; i < attempts; i++ {
		if serviceActive(ctx, d, service) {
			return true
		}
		d.Sleep(500 * time.Millisecond)
	}
	return false
}

// SwitchVoiceInputMode switches between the batch and streaming voice-input
// services. The two share a single voxtype runtime lock/socket and cannot
// run concurrently, so the only safe order is stop-other, start-wanted,
// verify: starting the wanted service first (to avoid a stopped gap) doesn't
// work here because it would immediately fail to acquire the lock while the
// other service still holds it. If the wanted service fails to become
// active, this restarts the other service as a fallback so voice input isn't
// left fully stopped, and reports the failure clearly either way.
func SwitchVoiceInputMode(ctx context.Context, d Dependencies, target VoiceInputMode) error {
	var wanted, other string
	switch target {
	case ModeStreaming:
		if err := checkStreamingPreconditions(d); err != nil {
			return err
		}
		wanted, other = StreamingService, BatchService
	case ModeBatch:
		wanted, other = BatchService, StreamingService
	default:
		return fmt.Errorf("invalid voice-input mode %q (want streaming or batch)", target)
	}

	if serviceActive(ctx, d, wanted) {
		fmt.Fprintf(d.Stdout, "%s already active; nothing to do\n", wanted)
		if serviceActive(ctx, d, other) {
			return fmt.Errorf("%s is also active; stop it manually (systemctl --user stop %s)", other, other)
		}
		return nil
	}

	otherWasActive := serviceActive(ctx, d, other)
	if otherWasActive {
		if err := d.Run(ctx, "systemctl", "--user", "stop", other); err != nil {
			return fmt.Errorf("stop %s: %w", other, err)
		}
	}

	if err := d.Run(ctx, "systemctl", "--user", "start", wanted); err != nil {
		if otherWasActive {
			_ = d.Run(ctx, "systemctl", "--user", "start", other)
		}
		return fmt.Errorf("start %s: %w (restored %s if it was running)", wanted, err, other)
	}
	if !waitActive(ctx, d, wanted) {
		restoreErr := ""
		if otherWasActive {
			if err := d.Run(ctx, "systemctl", "--user", "start", other); err != nil {
				restoreErr = fmt.Sprintf("; failed to restore %s too: %v -- voice input is fully stopped, run: systemctl --user start %s", other, err, other)
			} else {
				restoreErr = fmt.Sprintf("; restored %s as a fallback", other)
			}
		}
		return fmt.Errorf("%s did not become active within 10s%s", wanted, restoreErr)
	}
	fmt.Fprintf(d.Stdout, "switched voice input to %s (%s active, %s stopped)\n", target, wanted, other)
	return nil
}
