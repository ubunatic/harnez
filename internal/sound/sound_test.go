package sound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez"
)

func TestEmbeddedSpecLoads(t *testing.T) {
	s, err := LoadSpec()
	if err != nil {
		t.Fatalf("LoadSpec() failed: %v", err)
	}

	d, err := s.TimeoutDuration()
	if err != nil {
		t.Fatalf("TimeoutDuration() failed: %v", err)
	}
	if d != 3*time.Second {
		t.Errorf("TimeoutDuration() = %v, want 3s", d)
	}

	linux, err := s.ForOS("linux")
	if err != nil {
		t.Fatalf("ForOS(linux) failed: %v", err)
	}
	if linux.SoundFile != "/usr/share/sounds/freedesktop/stereo/window-attention.oga" {
		t.Errorf("linux.SoundFile = %q, want window-attention.oga", linux.SoundFile)
	}
	wantLinuxPlayers := []string{"pw-play", "paplay", "canberra-gtk-play", "aplay", "ffplay"}
	if len(linux.Players) != len(wantLinuxPlayers) {
		t.Fatalf("len(linux.Players) = %d, want %d", len(linux.Players), len(wantLinuxPlayers))
	}
	for i, want := range wantLinuxPlayers {
		if linux.Players[i].Name != want {
			t.Errorf("linux.Players[%d].Name = %q, want %q", i, linux.Players[i].Name, want)
		}
	}

	darwin, err := s.ForOS("darwin")
	if err != nil {
		t.Fatalf("ForOS(darwin) failed: %v", err)
	}
	if darwin.SoundFile != "/System/Library/Sounds/Glass.aiff" {
		t.Errorf("darwin.SoundFile = %q, want Glass.aiff", darwin.SoundFile)
	}
	if len(darwin.Players) != 1 || darwin.Players[0].Name != "afplay" {
		t.Errorf("darwin.Players = %+v, want [afplay]", darwin.Players)
	}
}

func TestSchemaValidatesEmbeddedSpec(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/sound.schema.json")
	if err != nil {
		t.Fatalf("ReadFile(schema) failed: %v", err)
	}
	var schema struct {
		Required   []string `json:"required"`
		Properties struct {
			Timeout   map[string]any `json:"timeout"`
			Platforms map[string]any `json:"platforms"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Unmarshal(schema) failed: %v", err)
	}
	if len(schema.Required) != 2 {
		t.Errorf("schema.Required = %v, want [timeout, platforms]", schema.Required)
	}
}

func TestParseSpecRejects(t *testing.T) {
	tests := map[string]string{
		"unknown field": `
timeout: 3s
bogus: true
platforms:
  linux:
    sound_file: /a.oga
    players:
      - name: p1
        command: ["p1", "{file}"]
`,
		"missing timeout": `
platforms:
  linux:
    sound_file: /a.oga
    players:
      - name: p1
        command: ["p1", "{file}"]
`,
		"invalid timeout": `
timeout: not-a-duration
platforms:
  linux:
    sound_file: /a.oga
    players:
      - name: p1
        command: ["p1", "{file}"]
`,
		"zero timeout": `
timeout: 0s
platforms:
  linux:
    sound_file: /a.oga
    players:
      - name: p1
        command: ["p1", "{file}"]
`,
		"empty platforms": `
timeout: 3s
platforms: {}
`,
		"empty sound_file": `
timeout: 3s
platforms:
  linux:
    sound_file: "  "
    players:
      - name: p1
        command: ["p1", "{file}"]
`,
		"empty players": `
timeout: 3s
platforms:
  linux:
    sound_file: /a.oga
    players: []
`,
		"empty player name": `
timeout: 3s
platforms:
  linux:
    sound_file: /a.oga
    players:
      - name: ""
        command: ["p1", "{file}"]
`,
		"empty player command": `
timeout: 3s
platforms:
  linux:
    sound_file: /a.oga
    players:
      - name: p1
        command: []
`,
		"empty command token": `
timeout: 3s
platforms:
  linux:
    sound_file: /a.oga
    players:
      - name: p1
        command: ["p1", "  "]
`,
	}

	for name, yml := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSpec([]byte(yml))
			if err == nil {
				t.Errorf("ParseSpec() succeeded, want error for case %q", name)
			}
		})
	}
}

func createFakePlayer(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := fmt.Sprintf("#!/bin/sh\n%s\n", body)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake player %s: %v", name, err)
	}
	return path
}

func TestFallbackChainFirstPlayerSucceeds(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "played.log")

	createFakePlayer(t, tempDir, "fake-pw", fmt.Sprintf("echo \"$@\" > %q\nexit 0", logFile))

	spec := &Spec{
		Timeout: "1s",
		Platforms: map[string]PlatformSpec{
			"linux": {
				SoundFile: "/test/sound.oga",
				Players: []PlayerSpec{
					{Name: "fake-pw", Command: []string{"fake-pw", "--sound", "{file}"}},
					{Name: "fake-pa", Command: []string{"fake-pa", "{file}"}},
				},
			},
		},
	}

	t.Setenv("PATH", tempDir+":"+os.Getenv("PATH"))

	err := PlayWithOptions(context.Background(), Options{
		Spec: spec,
		OS:   "linux",
	})
	if err != nil {
		t.Fatalf("PlayWithOptions() failed: %v", err)
	}

	logged, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", logFile, err)
	}
	if !strings.Contains(string(logged), "--sound /test/sound.oga") {
		t.Errorf("logged args = %q, want to contain '--sound /test/sound.oga'", string(logged))
	}
}

func TestFallbackChainSkipsMissingAndFailing(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "canberra.log")
	unreachableMarker := filepath.Join(tempDir, "aplay_called")

	// fake-missing is not created
	// fake-fail exits immediately with 1
	createFakePlayer(t, tempDir, "fake-fail", "exit 1")
	// fake-ok succeeds
	createFakePlayer(t, tempDir, "fake-ok", fmt.Sprintf("echo \"$@\" > %q\nexit 0", logFile))
	// fake-unreached should never be called
	createFakePlayer(t, tempDir, "fake-unreached", fmt.Sprintf("touch %q\nexit 0", unreachableMarker))

	spec := &Spec{
		Timeout: "2s",
		Platforms: map[string]PlatformSpec{
			"linux": {
				SoundFile: "/sound.oga",
				Players: []PlayerSpec{
					{Name: "fake-missing", Command: []string{"fake-missing", "{file}"}},
					{Name: "fake-fail", Command: []string{"fake-fail", "{file}"}},
					{Name: "fake-ok", Command: []string{"fake-ok", "-f", "{file}"}},
					{Name: "fake-unreached", Command: []string{"fake-unreached", "{file}"}},
				},
			},
		},
	}

	t.Setenv("PATH", tempDir+":"+os.Getenv("PATH"))

	err := PlayWithOptions(context.Background(), Options{
		Spec: spec,
		OS:   "linux",
	})
	if err != nil {
		t.Fatalf("PlayWithOptions() failed: %v", err)
	}

	logged, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", logFile, err)
	}
	if !strings.Contains(string(logged), "-f /sound.oga") {
		t.Errorf("logged args = %q, want '-f /sound.oga'", string(logged))
	}

	if _, err := os.Stat(unreachableMarker); !os.IsNotExist(err) {
		t.Errorf("fake-unreached was called but shouldn't have been")
	}
}

func TestFallbackChainTimeoutStopsImmediately(t *testing.T) {
	tempDir := t.TempDir()
	unreachableMarker := filepath.Join(tempDir, "player2_called")

	// fake-hung sleeps forever to simulate a hung audio sink
	createFakePlayer(t, tempDir, "fake-hung", "sleep 30")
	// fake-next should NOT be tried when fake-hung times out
	createFakePlayer(t, tempDir, "fake-next", fmt.Sprintf("touch %q\nexit 0", unreachableMarker))

	spec := &Spec{
		Timeout: "50ms",
		Platforms: map[string]PlatformSpec{
			"linux": {
				SoundFile: "/sound.oga",
				Players: []PlayerSpec{
					{Name: "fake-hung", Command: []string{"fake-hung", "{file}"}},
					{Name: "fake-next", Command: []string{"fake-next", "{file}"}},
				},
			},
		},
	}

	t.Setenv("PATH", tempDir+":"+os.Getenv("PATH"))

	start := time.Now()
	err := PlayWithOptions(context.Background(), Options{
		Spec: spec,
		OS:   "linux",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("PlayWithOptions() succeeded; want timeout error")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("err = %v, want ErrTimeout", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("elapsed = %v, want quick timeout (< 1s)", elapsed)
	}

	if _, err := os.Stat(unreachableMarker); !os.IsNotExist(err) {
		t.Errorf("second player was called after first player timed out")
	}
}

func TestAllPlayersFail(t *testing.T) {
	tempDir := t.TempDir()
	createFakePlayer(t, tempDir, "fake-p1", "exit 1")
	createFakePlayer(t, tempDir, "fake-p2", "exit 2")

	spec := &Spec{
		Timeout: "1s",
		Platforms: map[string]PlatformSpec{
			"linux": {
				SoundFile: "/sound.oga",
				Players: []PlayerSpec{
					{Name: "fake-p1", Command: []string{"fake-p1", "{file}"}},
					{Name: "fake-p2", Command: []string{"fake-p2", "{file}"}},
					{Name: "fake-missing", Command: []string{"fake-missing", "{file}"}},
				},
			},
		},
	}

	t.Setenv("PATH", tempDir+":"+os.Getenv("PATH"))

	err := PlayWithOptions(context.Background(), Options{
		Spec: spec,
		OS:   "linux",
	})
	if err == nil {
		t.Fatal("PlayWithOptions() succeeded; want error")
	}
	if !errors.Is(err, ErrNoPlayer) {
		t.Errorf("err = %v, want ErrNoPlayer", err)
	}
}

func TestUnsupportedOS(t *testing.T) {
	spec := &Spec{
		Timeout: "1s",
		Platforms: map[string]PlatformSpec{
			"linux": {
				SoundFile: "/sound.oga",
				Players:   []PlayerSpec{{Name: "fake-p1", Command: []string{"fake-p1", "{file}"}}},
			},
		},
	}

	err := PlayWithOptions(context.Background(), Options{
		Spec: spec,
		OS:   "plan9",
	})
	if err == nil {
		t.Fatal("PlayWithOptions() succeeded; want ErrUnsupportedOS")
	}
	if !errors.Is(err, ErrUnsupportedOS) {
		t.Errorf("err = %v, want ErrUnsupportedOS", err)
	}
}

func TestCustomSoundFileOverride(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "custom.log")
	createFakePlayer(t, tempDir, "fake-afplay", fmt.Sprintf("echo \"$@\" > %q\nexit 0", logFile))

	spec := &Spec{
		Timeout: "1s",
		Platforms: map[string]PlatformSpec{
			"darwin": {
				SoundFile: "/default/glass.aiff",
				Players:   []PlayerSpec{{Name: "fake-afplay", Command: []string{"fake-afplay", "{file}"}}},
			},
		},
	}

	t.Setenv("PATH", tempDir+":"+os.Getenv("PATH"))

	err := PlayWithOptions(context.Background(), Options{
		Spec: spec,
		OS:   "darwin",
		File: "/custom/ping.wav",
	})
	if err != nil {
		t.Fatalf("PlayWithOptions() failed: %v", err)
	}

	logged, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", logFile, err)
	}
	if !strings.Contains(string(logged), "/custom/ping.wav") {
		t.Errorf("logged args = %q, want '/custom/ping.wav'", string(logged))
	}
}
