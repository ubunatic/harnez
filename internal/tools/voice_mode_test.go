package tools

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeDirInfo is a minimal os.FileInfo that reports a directory, used to
// satisfy Dependencies.Stat without touching the real filesystem.
type fakeDirInfo struct{}

func (fakeDirInfo) Name() string       { return "parakeet-unified-en-0.6b" }
func (fakeDirInfo) Size() int64        { return 0 }
func (fakeDirInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (fakeDirInfo) ModTime() time.Time { return time.Time{} }
func (fakeDirInfo) IsDir() bool        { return true }
func (fakeDirInfo) Sys() any           { return nil }

// serviceDeps builds Dependencies whose systemctl is-active/start/stop calls
// are backed by an in-memory active-set, so mode switching can be tested
// without a real systemd user session. Sleep is a no-op so waitActive's
// polling loop runs instantly in tests.
func serviceDeps(out *bytes.Buffer, active map[string]bool) Dependencies {
	d := testDeps(out)
	d.Stat = func(string) (os.FileInfo, error) { return fakeDirInfo{}, nil }
	d.Sleep = func(time.Duration) {}
	d.Run = func(_ context.Context, name string, args ...string) error {
		if name != "systemctl" || len(args) < 3 || args[0] != "--user" {
			return errors.New("unexpected command")
		}
		verb, service := args[1], args[len(args)-1]
		switch verb {
		case "is-active":
			if active[service] {
				return nil
			}
			return errors.New("inactive")
		case "start":
			active[service] = true
			return nil
		case "stop":
			active[service] = false
			return nil
		default:
			return errors.New("unexpected verb")
		}
	}
	return d
}

func TestCurrentVoiceInputMode(t *testing.T) {
	cases := []struct {
		name   string
		active map[string]bool
		want   VoiceInputMode
	}{
		{"neither", map[string]bool{}, ModeNeither},
		{"batch", map[string]bool{BatchService: true}, ModeBatch},
		{"streaming", map[string]bool{StreamingService: true}, ModeStreaming},
		{"eager", map[string]bool{EagerService: true}, ModeEager},
		{"inconsistent", map[string]bool{BatchService: true, StreamingService: true}, ModeInconsistent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := serviceDeps(&bytes.Buffer{}, tc.active)
			if got := CurrentVoiceInputMode(context.Background(), d); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestSwitchVoiceInputMode(t *testing.T) {
	t.Run("batch to streaming stops batch then starts streaming", func(t *testing.T) {
		active := map[string]bool{BatchService: true}
		var out bytes.Buffer
		d := serviceDeps(&out, active)
		if err := SwitchVoiceInputMode(context.Background(), d, ModeStreaming); err != nil {
			t.Fatal(err)
		}
		if !active[StreamingService] || active[BatchService] {
			t.Fatalf("unexpected state: %+v", active)
		}
	})

	t.Run("batch to eager stops batch then starts eager", func(t *testing.T) {
		active := map[string]bool{BatchService: true}
		var out bytes.Buffer
		d := serviceDeps(&out, active)
		if err := SwitchVoiceInputMode(context.Background(), d, ModeEager); err != nil {
			t.Fatal(err)
		}
		if !active[EagerService] || active[BatchService] {
			t.Fatalf("unexpected state: %+v", active)
		}
	})

	t.Run("streaming to batch stops streaming then starts batch", func(t *testing.T) {
		active := map[string]bool{StreamingService: true}
		var out bytes.Buffer
		d := serviceDeps(&out, active)
		if err := SwitchVoiceInputMode(context.Background(), d, ModeBatch); err != nil {
			t.Fatal(err)
		}
		if !active[BatchService] || active[StreamingService] {
			t.Fatalf("unexpected state: %+v", active)
		}
	})

	t.Run("already active mode is a no-op", func(t *testing.T) {
		active := map[string]bool{BatchService: true}
		var out bytes.Buffer
		d := serviceDeps(&out, active)
		if err := SwitchVoiceInputMode(context.Background(), d, ModeBatch); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "already active") {
			t.Fatal(out.String())
		}
		if !active[BatchService] {
			t.Fatal("batch should still be active")
		}
	})

	t.Run("streaming rejected when config missing, batch stays up", func(t *testing.T) {
		active := map[string]bool{BatchService: true}
		var out bytes.Buffer
		d := serviceDeps(&out, active)
		d.Stat = func(string) (os.FileInfo, error) { return nil, errors.New("not found") }
		err := SwitchVoiceInputMode(context.Background(), d, ModeStreaming)
		if err == nil || !strings.Contains(err.Error(), "streaming config missing") {
			t.Fatalf("got %v", err)
		}
		if !active[BatchService] {
			t.Fatal("batch must stay running when the switch is rejected before anything is stopped")
		}
	})

	t.Run("start failure restores the other service", func(t *testing.T) {
		active := map[string]bool{BatchService: true}
		var out bytes.Buffer
		d := serviceDeps(&out, active)
		baseRun := d.Run
		d.Run = func(ctx context.Context, name string, args ...string) error {
			if name == "systemctl" && len(args) >= 3 && args[1] == "start" && args[2] == StreamingService {
				return errors.New("boom")
			}
			return baseRun(ctx, name, args...)
		}
		err := SwitchVoiceInputMode(context.Background(), d, ModeStreaming)
		if err == nil || !strings.Contains(err.Error(), "start "+StreamingService) {
			t.Fatalf("got %v", err)
		}
		if !active[BatchService] || active[StreamingService] {
			t.Fatalf("batch must be restored after a failed start: %+v", active)
		}
	})

	t.Run("service that never reports active restores the other service", func(t *testing.T) {
		active := map[string]bool{BatchService: true}
		var out bytes.Buffer
		d := serviceDeps(&out, active)
		baseRun := d.Run
		d.Run = func(ctx context.Context, name string, args ...string) error {
			// start "succeeds" (the process launches) but never actually
			// reports active, simulating an immediate crash loop.
			if name == "systemctl" && len(args) >= 3 && args[1] == "start" && args[2] == StreamingService {
				return nil
			}
			return baseRun(ctx, name, args...)
		}
		err := SwitchVoiceInputMode(context.Background(), d, ModeStreaming)
		if err == nil || !strings.Contains(err.Error(), "did not become active") {
			t.Fatalf("got %v", err)
		}
		if !strings.Contains(err.Error(), "restored "+BatchService) {
			t.Fatalf("expected fallback restore in error, got %v", err)
		}
		if !active[BatchService] || active[StreamingService] {
			t.Fatalf("batch must be restored when streaming never becomes active: %+v", active)
		}
	})

	t.Run("invalid mode is rejected", func(t *testing.T) {
		d := serviceDeps(&bytes.Buffer{}, map[string]bool{})
		err := SwitchVoiceInputMode(context.Background(), d, VoiceInputMode("bogus"))
		if err == nil || !strings.Contains(err.Error(), "invalid voice-input mode") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestDescribeVoiceInputMode(t *testing.T) {
	cases := map[VoiceInputMode]string{
		ModeBatch:        "batch",
		ModeStreaming:    "streaming",
		ModeNeither:      "neither",
		ModeInconsistent: "inconsistent",
	}
	for mode, want := range cases {
		if got := DescribeVoiceInputMode(mode); !strings.Contains(got, want) {
			t.Fatalf("%s: got %q", mode, got)
		}
	}
}
