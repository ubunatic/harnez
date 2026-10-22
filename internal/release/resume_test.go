package release

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func signedDist(t *testing.T) (string, string, []string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	name := "app_1.2.3_linux_amd64.tar.gz"
	sums := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte("archive")), name)
	for name, data := range map[string]string{name: "archive", "SHA256SUMS": sums, "SHA256SUMS.minisig": "signature"} {
		if err := os.WriteFile(filepath.Join(dir, "dist", name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, sums, []string{filepath.Join("dist", name), "dist/SHA256SUMS", "dist/SHA256SUMS.minisig"}
}

func TestCompleteDist(t *testing.T) {
	for _, tc := range []struct {
		name, remove, change, version string
		complete, wantErr             bool
	}{
		{name: "complete", version: "1.2.3", complete: true},
		{name: "missing minisig rebuilds", remove: "SHA256SUMS.minisig", version: "1.2.3"},
		{name: "missing archive rebuilds", remove: "app_1.2.3_linux_amd64.tar.gz", version: "1.2.3"},
		{name: "wrong version rebuilds", version: "1.2.4"},
		{name: "checksum mismatch fails", change: "app_1.2.3_linux_amd64.tar.gz", version: "1.2.3", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, _, _ := signedDist(t)
			if tc.remove != "" {
				if err := os.Remove(filepath.Join(dir, "dist", tc.remove)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.change != "" {
				if err := os.WriteFile(filepath.Join(dir, "dist", tc.change), []byte("changed"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := completeDist(dir, tc.version)
			if got != tc.complete || (err != nil) != tc.wantErr {
				t.Fatalf("completeDist = %v, %v", got, err)
			}
		})
	}
}

func TestCompleteDistResolvesGoReleaserArtifactPaths(t *testing.T) {
	dir := t.TempDir()
	dist := filepath.Join(dir, "dist")
	for _, subdir := range []string{"app-linux_linux_amd64_v1", "app-linux_linux_arm64_v8.0"} {
		if err := os.MkdirAll(filepath.Join(dist, subdir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	amd64 := "app-linux_linux_amd64_v1/app"
	arm64 := "app-linux_linux_arm64_v8.0/app"
	files := map[string]string{
		amd64: "amd64 binary", arm64: "arm64 binary",
		"app-1.2.3-linux-amd64.tar.gz": "archive",
	}
	var sums strings.Builder
	for path, data := range files {
		if err := os.WriteFile(filepath.Join(dist, filepath.FromSlash(path)), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		if strings.Contains(path, "/") {
			if path == amd64 {
				name = "app-x86_64-linux"
			} else {
				name = "app-aarch64-linux"
			}
		}
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256([]byte(data)), name)
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte(sums.String()), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS.minisig"), []byte("signature"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `[{"name":"app-x86_64-linux","path":"dist/app-linux_linux_amd64_v1/app"},{"name":"app-aarch64-linux","path":"dist/app-linux_linux_arm64_v8.0/app"}]`
	if err := os.WriteFile(filepath.Join(dist, "artifacts.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := completeDist(dir, "1.2.3")
	if err != nil || !got {
		t.Fatalf("completeDist = %v, %v; expected real-shaped signed dist to be complete", got, err)
	}
}

func TestPublishAssetResume(t *testing.T) {
	for _, scenario := range []string{"absent", "partial", "complete", "mismatch", "local mismatch", "auth", "network", "missing sums"} {
		t.Run(scenario, func(t *testing.T) {
			dir, sums, files := signedDist(t)
			t.Setenv("TMPDIR", t.TempDir())
			if scenario == "local mismatch" {
				if err := os.WriteFile(filepath.Join(dir, files[0]), []byte("corrupt"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			var uploads []string
			creates := 0
			fake := func(args ...string) ([]byte, error) {
				switch {
				case args[0] == "--style":
					if scenario == "absent" {
						return []byte("could not find release app v1.2.3"), fmt.Errorf("exit 1")
					}
					if scenario == "auth" {
						return []byte("api error: token was already used"), fmt.Errorf("exit 1")
					}
					if scenario == "network" {
						return []byte("connection refused"), fmt.Errorf("exit 1")
					}
					names := []string{"SHA256SUMS", filepath.Base(files[0])}
					if scenario == "missing sums" {
						names = names[1:]
					}
					if scenario == "complete" {
						names = append(names, "SHA256SUMS.minisig")
					}
					return []byte(fmt.Sprintf("release header\n\n%d assets\n- %s\n- source.zip\n- source.tar.gz\n", len(names)+2, strings.Join(names, "\n- "))), nil
				case args[1] == "create":
					creates++
					if scenario == "auth" {
						return []byte("api error: token was already used"), fmt.Errorf("exit 1")
					}
					if scenario == "network" {
						return []byte("connection refused"), fmt.Errorf("exit 1")
					}
					return nil, nil
				case args[2] == "download":
					data := sums
					if scenario == "mismatch" {
						data = strings.ReplaceAll(sums, sums[:64], strings.Repeat("0", 64))
					}
					return nil, os.WriteFile(args[len(args)-1], []byte(data), 0600)
				case args[2] == "create":
					uploads = append(uploads, filepath.Base(args[len(args)-1]))
					return nil, nil
				default:
					t.Fatalf("unexpected command %q", args)
				}
				return nil, nil
			}
			var out bytes.Buffer
			err := publishForgejoRelease(dir, "v1.2.3", "app v1.2.3", files, &out, fake)
			if scenario == "mismatch" || scenario == "local mismatch" || scenario == "auth" || scenario == "network" || scenario == "missing sums" {
				wantCreates := 0
				if scenario == "auth" || scenario == "network" {
					wantCreates = 1
				}
				if err == nil || len(uploads) != 0 || creates != wantCreates {
					t.Fatalf("error=%v uploads=%v creates=%d", err, uploads, creates)
				}
				if scenario == "auth" && !strings.Contains(err.Error(), "fj auth login") {
					t.Fatal(err)
				}
				if scenario == "mismatch" && !strings.Contains(err.Error(), "checksum mismatch") {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]int{"absent": 3, "partial": 1, "complete": 0}[scenario]
			if len(uploads) != want {
				t.Fatalf("uploads = %v, want %d", uploads, want)
			}
			wantCreates := scenario == "absent" || scenario == "auth" || scenario == "network"
			if (creates == 1) != wantCreates {
				t.Fatalf("creates = %d", creates)
			}
			if scenario != "absent" && !strings.Contains(out.String(), "exists, skipped") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestPublishRetriesUntilReleaseIsVisible(t *testing.T) {
	dir, _, files := signedDist(t)
	var uploads []string
	var waits []time.Duration
	assetAttempts := 0
	fake := func(args ...string) ([]byte, error) {
		switch {
		case args[0] == "--style":
			return []byte("could not find release app v1.2.3"), fmt.Errorf("exit 1")
		case args[1] == "create":
			return nil, nil
		case args[2] == "create":
			assetAttempts++
			if len(uploads) == 0 && assetAttempts < 3 {
				return []byte("Error: could not find release app v1.2.3"), fmt.Errorf("exit 1")
			}
			uploads = append(uploads, filepath.Base(args[len(args)-2]))
			return nil, nil
		default:
			t.Fatalf("unexpected command %q", args)
			return nil, nil
		}
	}
	var out bytes.Buffer
	cfg, err := loadPublishConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.UploadRetryAttempts = 3
	cfg.UploadRetryDelay = time.Second
	err = publishForgejoReleaseWithConfig(dir, "v1.2.3", "app v1.2.3", files, &out, fake, cfg, func(delay time.Duration) { waits = append(waits, delay) })
	if err != nil {
		t.Fatal(err)
	}
	if assetAttempts != len(files)+2 || len(uploads) != 3 || len(waits) != 2 {
		t.Fatalf("attempts=%d uploads=%v waits=%v", assetAttempts, uploads, waits)
	}
}

func TestPublishIgnoresListingFailureAndExistingAssetConflict(t *testing.T) {
	dir, _, files := signedDist(t)
	var uploaded []string
	fake := func(args ...string) ([]byte, error) {
		switch {
		case args[0] == "--style":
			return []byte("temporary listing failure"), fmt.Errorf("exit 1")
		case args[1] == "create":
			return []byte("release already exists"), fmt.Errorf("exit 1")
		case args[2] == "create":
			name := args[len(args)-1]
			uploaded = append(uploaded, name)
			if name == filepath.Base(files[0]) {
				return []byte("asset already exists"), fmt.Errorf("exit 1")
			}
			return nil, nil
		default:
			t.Fatalf("unexpected command %q", args)
			return nil, nil
		}
	}
	var out bytes.Buffer
	err := publishForgejoReleaseWithConfig(dir, "v1.2.3", "app v1.2.3", files, &out, fake, PublishConfig{UploadRetryAttempts: 2, UploadRetryDelay: time.Second}, func(time.Duration) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(uploaded) != len(files) {
		t.Fatalf("uploaded=%v, want every asset after failed listing", uploaded)
	}
}

func TestPublishAssetListUsesManifestPathAndName(t *testing.T) {
	dir := t.TempDir()
	dist := filepath.Join(dir, "dist")
	if err := os.MkdirAll(filepath.Join(dist, "harnez-linux_linux_arm64_v8.0"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "harnez-linux_linux_arm64_v8.0", "harnez"), []byte("binary"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "harnez-0.2.0-aarch64-linux.tar.gz"), []byte("archive"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `[{"name":"harnez-aarch64-linux","path":"dist/harnez-linux_linux_arm64_v8.0/harnez"},{"name":"harnez-0.2.0-aarch64-linux.tar.gz","path":"dist/harnez-0.2.0-aarch64-linux.tar.gz"}]`
	if err := os.WriteFile(filepath.Join(dist, "artifacts.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := releaseAssetsFromDist(dir, "harnez", "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"harnez-0.2.0-aarch64-linux.tar.gz": "dist/harnez-0.2.0-aarch64-linux.tar.gz",
	}
	if len(got) != len(want) {
		t.Fatalf("assets=%+v, want %v", got, want)
	}
	for _, asset := range got {
		if want[asset.Name] != asset.Path {
			t.Errorf("asset %q path=%q, want %q", asset.Name, asset.Path, want[asset.Name])
		}
	}
}
