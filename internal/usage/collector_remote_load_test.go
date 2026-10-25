package usage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolatedRemoteLoadEnv points every XDG base directory at a temp dir, so the
// collector neither reads a developer's real local.yaml nor writes a real cache.
func isolatedRemoteLoadEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	return home
}

// scriptedRemoteLoad answers polls from results (nil means success; the
// reading's NumCPU is the poll number, so the cache shows which poll won).
// When the script runs out it cancels the poller and fails. Waits are recorded
// and return at once, so the schedule is checked without sleeping.
type scriptedRemoteLoad struct {
	results []error
	calls   int
	waits   []time.Duration
	cancel  context.CancelFunc
}

func (s *scriptedRemoteLoad) collect(context.Context, string) (*LoadSnapshot, error) {
	i := s.calls
	s.calls++
	if i >= len(s.results) {
		s.cancel()
		return nil, errors.New("script exhausted")
	}
	if s.results[i] != nil {
		return nil, s.results[i]
	}
	return &LoadSnapshot{CPU: CPULoad{NumCPU: i + 1}}, nil
}

func (s *scriptedRemoteLoad) wait(ctx context.Context, d time.Duration, _ <-chan struct{}) {
	if ctx.Err() == nil {
		s.waits = append(s.waits, d)
	}
}

func scriptedPoll(t *testing.T, results ...error) (*scriptedRemoteLoad, remoteLoadPollOptions, context.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	script := &scriptedRemoteLoad{results: results, cancel: cancel}
	opt := remoteLoadPollOptions{
		Host:          "x600",
		PollAttached:  10 * time.Second,
		PollDetached:  5 * time.Minute,
		Initial:       10 * time.Second,
		Max:           40 * time.Second,
		ActiveClients: func() int { return 1 },
		Collect:       script.collect,
		Wait:          script.wait,
		Out:           io.Discard,
	}
	return script, opt, ctx
}

func TestRemoteLoadPollerBacksOffThenRecovers(t *testing.T) {
	isolatedRemoteLoadEnv(t)
	script, opt, ctx := scriptedPoll(t, errors.New("down"), errors.New("down"), errors.New("down"), nil, nil)

	runRemoteLoadPoller(ctx, opt)

	want := []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second, 10 * time.Second, 10 * time.Second}
	if !equalDurations(script.waits, want) {
		t.Fatalf("poll waits = %v, want %v", script.waits, want)
	}
	cached := ReadCachedRemoteLoad("x600")
	if cached == nil || cached.CPU.NumCPU != 5 {
		t.Fatalf("cached remote load = %+v, want the last successful poll (NumCPU 5)", cached)
	}
}

func TestRemoteLoadPollerResetsBackoffAfterSuccess(t *testing.T) {
	isolatedRemoteLoadEnv(t)
	script, opt, ctx := scriptedPoll(t, errors.New("down"), errors.New("down"), nil, errors.New("down"), nil)

	runRemoteLoadPoller(ctx, opt)

	want := []time.Duration{10 * time.Second, 20 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second}
	if !equalDurations(script.waits, want) {
		t.Fatalf("poll waits = %v, want %v", script.waits, want)
	}
}

func TestRemoteLoadPollerSwitchesCadenceOnClientHeartbeat(t *testing.T) {
	isolatedRemoteLoadEnv(t)
	tracker := NewClientTracker(time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var waits []time.Duration
	var calls int
	opt := remoteLoadPollOptions{
		Host:          "x600",
		PollAttached:  10 * time.Second,
		PollDetached:  5 * time.Minute,
		Initial:       10 * time.Second,
		Max:           time.Minute,
		ActiveClients: tracker.Count,
		WakeOnAttach:  tracker.FirstClientEvents(),
		Collect: func(context.Context, string) (*LoadSnapshot, error) {
			calls++
			return &LoadSnapshot{CPU: CPULoad{NumCPU: calls}}, nil
		},
		Wait: func(_ context.Context, d time.Duration, wake <-chan struct{}) {
			waits = append(waits, d)
			if len(waits) == 1 {
				tracker.Heartbeat("watcher")
				<-wake
				return
			}
			cancel()
		},
		Out: io.Discard,
	}

	runRemoteLoadPoller(ctx, opt)

	want := []time.Duration{5 * time.Minute, 10 * time.Second}
	if !equalDurations(waits, want) {
		t.Fatalf("remote load poll waits = %v, want detached then attached cadence %v", waits, want)
	}
	if calls != 2 {
		t.Fatalf("remote load poll calls = %d, want a second collection after client attach", calls)
	}
}

func TestRemoteLoadPollerStopsOnCancel(t *testing.T) {
	isolatedRemoteLoadEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	polling := make(chan struct{})
	done := make(chan struct{})
	opt := remoteLoadPollOptions{
		Host:         "x600",
		PollAttached: time.Minute,
		PollDetached: 5 * time.Minute,
		Initial:      time.Second,
		Max:          time.Second,
		Collect: func(ctx context.Context, _ string) (*LoadSnapshot, error) {
			close(polling)
			<-ctx.Done()
			return nil, ctx.Err()
		},
		Wait: func(context.Context, time.Duration, <-chan struct{}) {},
		Out:  io.Discard,
	}
	go func() {
		defer close(done)
		runRemoteLoadPoller(ctx, opt)
	}()
	<-polling
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("remote load poller did not return within 5s of cancellation")
	}
}

func TestRemoteLoadPollerCacheWriteFailureIsNonfatal(t *testing.T) {
	home := isolatedRemoteLoadEnv(t)
	// A file where the state directory must be makes the cache write fail.
	if err := os.WriteFile(filepath.Join(home, "state"), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	script, opt, ctx := scriptedPoll(t, nil, nil)
	var out bytes.Buffer
	opt.Out = &out

	runRemoteLoadPoller(ctx, opt)

	if script.calls != 3 {
		t.Fatalf("polls = %d, want 3: a failed cache write must not stop polling", script.calls)
	}
	if !strings.Contains(out.String(), "remote load cache write failed") {
		t.Fatalf("output %q does not report the cache write failure", out.String())
	}
}

func TestConfiguredRemoteLoadPollReadsWatchHost(t *testing.T) {
	cases := []struct {
		name      string
		localYAML string // empty means no local.yaml
		wantNil   bool
		wantMsg   string
	}{
		{name: "no local config", wantNil: true},
		{name: "empty watch host", localYAML: "load:\n  watch_host: ''\n", wantNil: true},
		{name: "malformed local config", localYAML: "load: [\n", wantNil: true, wantMsg: "remote load poll disabled"},
		{name: "watch host configured", localYAML: "load:\n  watch_host: x600\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolatedRemoteLoadEnv(t)
			if tc.localYAML != "" {
				dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "harnez")
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "local.yaml"), []byte(tc.localYAML), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			got := configuredRemoteLoadPoll("", &out)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("configuredRemoteLoadPoll() = %+v, want nil", got)
				}
			} else {
				if got == nil {
					t.Fatal("configuredRemoteLoadPoll() = nil, want a poll for x600")
				}
				if got.Host != "x600" || got.PollAttached != 10*time.Second || got.PollDetached != 5*time.Minute || got.Initial != remoteLoadRetryInitial || got.Max != remoteLoadRetryMax || got.Collect == nil || got.Wait == nil {
					t.Fatalf("configuredRemoteLoadPoll() = %+v, want host x600, attached 10s, detached 5m and the M1 retry policy", got)
				}
			}
			if tc.wantMsg != "" && !strings.Contains(out.String(), tc.wantMsg) {
				t.Fatalf("output %q does not contain %q", out.String(), tc.wantMsg)
			}
		})
	}
}

// Quota polling has no clients here, so the remote poll must still start; the
// offline flag only gates the quota API and must not stop it.
func TestRunCollectorPollsRemoteLoadWithoutClientsOrQuota(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(map[bool]string{false: "online", true: "offline"}[offline], func(t *testing.T) {
			home := isolatedRemoteLoadEnv(t)
			polled := make(chan struct{}, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deps := collectorDeps{remote: &remoteLoadPollOptions{
				Host:         "x600",
				PollAttached: time.Hour,
				PollDetached: time.Hour,
				Initial:      time.Second,
				Max:          time.Second,
				Collect: func(context.Context, string) (*LoadSnapshot, error) {
					select {
					case polled <- struct{}{}:
					default:
					}
					return nil, errors.New("down")
				},
				Wait: func(ctx context.Context, _ time.Duration, _ <-chan struct{}) { <-ctx.Done() },
				Out:  io.Discard,
			}}
			done := make(chan error, 1)
			go func() {
				done <- runCollectorWithDeps(ctx, home, filepath.Join(home, "usage.sqlite"), time.Hour, io.Discard, offline,
					func(context.Context) (UsageSummary, error) { return UsageSummary{Timestamp: time.Now()}, nil },
					nil, nil, nil, deps)
			}()

			select {
			case <-polled:
			case <-time.After(5 * time.Second):
				t.Fatal("remote load was not polled at collector start")
			}
			cancel()
			if err := <-done; err != nil {
				t.Fatalf("runCollectorWithDeps() = %v", err)
			}
		})
	}
}

// The poll is joined before the collector returns, so no SSH child can outlive it.
func TestRunCollectorJoinsRemoteLoadPollOnCancel(t *testing.T) {
	home := isolatedRemoteLoadEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	polling := make(chan struct{})
	stopped := make(chan struct{})
	deps := collectorDeps{remote: &remoteLoadPollOptions{
		Host:         "x600",
		PollAttached: time.Hour,
		PollDetached: time.Hour,
		Initial:      time.Second,
		Max:          time.Second,
		Collect: func(ctx context.Context, _ string) (*LoadSnapshot, error) {
			close(polling)
			<-ctx.Done()
			close(stopped)
			return nil, ctx.Err()
		},
		Wait: func(ctx context.Context, _ time.Duration, _ <-chan struct{}) { <-ctx.Done() },
		Out:  io.Discard,
	}}
	done := make(chan error, 1)
	go func() {
		done <- runCollectorWithDeps(ctx, home, filepath.Join(home, "usage.sqlite"), time.Hour, io.Discard, false,
			func(context.Context) (UsageSummary, error) { return UsageSummary{Timestamp: time.Now()}, nil },
			nil, nil, nil, deps)
	}()
	<-polling
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("runCollectorWithDeps() = %v", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("collector returned before the remote load poll stopped")
	}
}

func equalDurations(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
