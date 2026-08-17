package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"ubunatic.com/harnez"
)

func testTool(verified bool) Tool {
	return Tool{ID: "voice-input", Title: "Voice input", Description: "Local dictation", Provider: Provider{Name: "Voxtype", Version: "1", Model: "base.en", ModelSizeMiB: 150},
		Platforms: []Platform{{OS: "linux", Arch: "amd64", Distro: "fedora", Version: "44", Desktop: "GNOME", Session: "wayland", CanaryVerified: verified, CanaryNote: "canary pending"}},
		Artifacts: map[string]Artifact{
			"user":   {Filename: "voxtype", URL: "https://example.invalid/user", SHA256: strings.Repeat("a", 64), SizeBytes: 1024},
			"system": {Filename: "voxtype.rpm", URL: "https://example.invalid/system", SHA256: strings.Repeat("b", 64), SizeBytes: 2048},
		},
		Probes: Probes{Commands: []string{"voxtype", "eitype"}, UserService: "voxtype.service"}}
}
func testDeps(out *bytes.Buffer) Dependencies {
	return Dependencies{GOOS: "linux", GOARCH: "amd64", Getenv: func(k string) string {
		if k == "XDG_CURRENT_DESKTOP" {
			return "GNOME"
		}
		if k == "HOME" {
			return "/home/test"
		}
		return "wayland"
	},
		ReadFile: func(string) ([]byte, error) { return []byte("ID=fedora\nVERSION_ID=44\n"), nil },
		Stat:     func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		LookPath: func(string) (string, error) { return "", errors.New("missing") },
		Run:      func(context.Context, string, ...string) error { return errors.New("inactive") },
		RunStdin: func(context.Context, string, string, ...string) error { return errors.New("not run in tests") },
		Sleep:    func(time.Duration) {}, Stdin: strings.NewReader("n\n"), Stdout: out}
}

func TestProbeStates(t *testing.T) {
	h := Host{OS: "linux", Arch: "amd64", Distro: "fedora", Version: "44", Desktop: "GNOME", Session: "wayland"}
	cases := []struct {
		name   string
		tool   Tool
		mutate func(*Dependencies)
		want   State
	}{
		{"canary pending", testTool(false), func(*Dependencies) {}, Unsupported},
		{"missing", testTool(true), func(*Dependencies) {}, Missing},
		{"partial", testTool(true), func(d *Dependencies) {
			d.LookPath = func(n string) (string, error) {
				if n == "voxtype" {
					return "/bin/voxtype", nil
				}
				return "", errors.New("missing")
			}
		}, Partial},
		{"ready", testTool(true), func(d *Dependencies) {
			d.LookPath = func(string) (string, error) { return "/bin/tool", nil }
			d.Run = func(context.Context, string, ...string) error { return nil }
		}, Ready},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := testDeps(&bytes.Buffer{})
			tc.mutate(&d)
			if got := Probe(context.Background(), tc.tool, h, d).State; got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	if got := Probe(context.Background(), testTool(true), Host{OS: "darwin"}, testDeps(&bytes.Buffer{})).State; got != Unsupported {
		t.Fatalf("unsupported host: %s", got)
	}
}

func TestInstallSafety(t *testing.T) {
	h := Host{OS: "linux", Arch: "amd64", Distro: "fedora", Version: "44", Desktop: "GNOME", Session: "wayland"}
	t.Run("dry run never invokes operations", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		calls := 0
		d.Run = func(context.Context, string, ...string) error { calls++; return nil }
		if err := Install(context.Background(), testTool(false), h, d, InstallOptions{Scope: "user", DryRun: true}); err != nil {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatalf("ran %d operations", calls)
		}
		if !strings.Contains(out.String(), "no network, files, services") {
			t.Fatal(out.String())
		}
	})
	t.Run("refusal", func(t *testing.T) {
		d := testDeps(&bytes.Buffer{})
		err := Install(context.Background(), testTool(true), h, d, InstallOptions{Scope: "user"})
		if err == nil || !strings.Contains(err.Error(), "declined") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("yes cannot bypass canary", func(t *testing.T) {
		d := testDeps(&bytes.Buffer{})
		err := Install(context.Background(), testTool(false), h, d, InstallOptions{Scope: "user", Yes: true})
		if err == nil || !strings.Contains(err.Error(), "canary pending") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestVerifyAndInstall(t *testing.T) {
	payload := []byte("verified")
	sum := sha256.Sum256(payload)
	artifact := Artifact{SHA256: hex.EncodeToString(sum[:])}

	t.Run("checksum mismatch preserves existing file", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "voxtype")
		if err := os.WriteFile(dest, []byte("owned by user"), 0600); err != nil {
			t.Fatal(err)
		}
		changed, err := VerifyAndInstall([]byte("new"), Artifact{SHA256: "bad"}, dest)
		if err == nil || changed {
			t.Fatalf("got changed=%v err=%v", changed, err)
		}
		data, readErr := os.ReadFile(dest)
		if readErr != nil || string(data) != "owned by user" {
			t.Fatalf("existing file changed: %q %v", data, readErr)
		}
	})

	t.Run("valid payload refuses unmanaged destination", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "voxtype")
		if err := os.WriteFile(dest, []byte("owned by user"), 0600); err != nil {
			t.Fatal(err)
		}
		changed, err := VerifyAndInstall(payload, artifact, dest)
		if err == nil || changed || !strings.Contains(err.Error(), "refusing overwrite") {
			t.Fatalf("got changed=%v err=%v", changed, err)
		}
		data, _ := os.ReadFile(dest)
		if string(data) != "owned by user" {
			t.Fatalf("existing file changed: %q", data)
		}
	})

	t.Run("matching destination performs zero writes", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "voxtype")
		changed, err := VerifyAndInstall(payload, artifact, dest)
		if err != nil || !changed {
			t.Fatalf("first install changed=%v err=%v", changed, err)
		}
		before, err := os.Stat(dest)
		if err != nil {
			t.Fatal(err)
		}
		changed, err = VerifyAndInstall(payload, artifact, dest)
		if err != nil || changed {
			t.Fatalf("second install changed=%v err=%v", changed, err)
		}
		after, err := os.Stat(dest)
		if err != nil {
			t.Fatal(err)
		}
		if !after.ModTime().Equal(before.ModTime()) {
			t.Fatalf("second install mutated mtime: %v -> %v", before.ModTime(), after.ModTime())
		}
	})
}

func TestCatalogRejectsInvalidAndUnknownFields(t *testing.T) {
	cases := map[string]string{
		"missing artifact metadata": "id: voice-input\ntitle: Voice\ndescription: Test\nprovider: {name: Voxtype, version: '1', model: base.en, model_size_mib: 1}\nplatforms: [{os: linux}]\nartifacts: {}\nprobes: {commands: [voxtype], user_service: voxtype.service}\n",
		"unknown field":             "id: voice-input\ntitle: Voice\ndescription: Test\nunknown: true\nprovider: {name: Voxtype}\nplatforms: [{os: linux}]\n",
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			fsys := fstest.MapFS{"spec/tools/voice.yaml": {Data: []byte(spec)}}
			if _, err := LoadCatalog(fsys); err == nil {
				t.Fatal("expected invalid catalog")
			}
		})
	}
}

func TestEmbeddedSpecAndSchema(t *testing.T) {
	catalog, err := LoadCatalog(harnez.DefaultFS)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Tools) != 1 || catalog.Tools[0].ID != "voice-input" {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	data, err := harnez.DefaultFS.ReadFile("spec/schemas/tool.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("invalid schema: %v", err)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("tool schema must reject undeclared top-level properties")
	}
}
