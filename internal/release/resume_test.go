package release

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
				if err == nil || len(uploads) != 0 || creates != 0 {
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
			if (creates == 1) != (scenario == "absent") {
				t.Fatalf("creates = %d", creates)
			}
			if scenario != "absent" && !strings.Contains(out.String(), "exists, skipped") {
				t.Fatal(out.String())
			}
		})
	}
}
