package usage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteLoadSSHConfig(t *testing.T) {
	for _, config := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "present"}[config], func(t *testing.T) {
			home := isolatedRemoteLoadEnv(t)
			configPath := filepath.Join(home, ".ssh", "config")
			if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
				t.Fatal(err)
			}
			if config {
				if err := os.WriteFile(configPath, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			argsPath := filepath.Join(home, "args")
			t.Setenv("SSH_TEST_ARGS", argsPath)
			t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SSH_TEST_ARGS\"\nprintf '  ssh diagnostic  \\n' >&2\nexit 255\n"
			if err := os.WriteFile(filepath.Join(home, "ssh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			_, err := CollectRemoteLoadSnapshot(context.Background(), "example-host")
			if err == nil || !strings.Contains(err.Error(), "failed: ssh diagnostic") {
				t.Fatalf("error = %v, want trimmed SSH stderr", err)
			}
			data, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			args := string(data)
			if strings.Contains(args, "-F\n"+configPath+"\n") != config {
				t.Fatalf("SSH args = %q, config present = %v", args, config)
			}
			if !config && strings.Contains(args, "-F\n") {
				t.Fatalf("unexpected config flag: %q", args)
			}
			if strings.Contains(args, "-q\n") || !strings.Contains(args, "BatchMode=yes\n") || !strings.Contains(args, "ConnectTimeout=") {
				t.Fatalf("unexpected SSH options: %q", args)
			}
		})
	}
}

func TestRemoteLoadLogsStateChanges(t *testing.T) {
	isolatedRemoteLoadEnv(t)
	_, opt, ctx := scriptedPoll(t, errors.New("  ssh unavailable \n"), errors.New("again"), nil, nil)
	var out bytes.Buffer
	opt.Out = &out
	runRemoteLoadPoller(ctx, opt)
	want := "harnez server: remote load x600 failed: ssh unavailable\nharnez server: remote load x600 recovered\n"
	if out.String() != want {
		t.Fatalf("log = %q, want %q", out.String(), want)
	}
}
