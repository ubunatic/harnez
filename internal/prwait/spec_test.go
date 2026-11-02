package prwait

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"ubunatic.com/harnez"
)

func TestLoadSpecEmbedded(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	if spec.DefaultRemote != "origin" {
		t.Errorf("DefaultRemote = %q, want origin", spec.DefaultRemote)
	}
	if spec.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", spec.PollInterval)
	}
	if spec.DefaultTimeout != 12*time.Hour {
		t.Errorf("DefaultTimeout = %v, want 12h", spec.DefaultTimeout)
	}
	if spec.RequestTimeout != 15*time.Second {
		t.Errorf("RequestTimeout = %v, want 15s", spec.RequestTimeout)
	}
	if spec.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want main", spec.DefaultBranch)
	}
	if spec.MaxConsecutiveErrors != 3 {
		t.Errorf("MaxConsecutiveErrors = %d, want 3", spec.MaxConsecutiveErrors)
	}
	if spec.OpenPRScan.Timeout != 5*time.Second || spec.OpenPRScan.PageSize != 50 || spec.OpenPRScan.MaxPages != 4 {
		t.Errorf("OpenPRScan = %+v, want 5s/50/4", spec.OpenPRScan)
	}
	cb, ok := spec.ForgeFor("Codeberg.org")
	if !ok || cb.Kind != "forgejo" || cb.APIBase != "https://codeberg.org/api/v1" {
		t.Errorf("codeberg forge = %+v, %v", cb, ok)
	}
	gh, ok := spec.ForgeFor("github.com")
	if !ok || gh.Kind != "github" || gh.APIBase != "https://api.github.com" {
		t.Errorf("github forge = %+v, %v", gh, ok)
	}
	if _, ok := spec.ForgeFor("example.org"); ok {
		t.Errorf("unlisted host should not resolve to a forge")
	}
}

func TestLoadSpecRejectsInvalid(t *testing.T) {
	schema, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/pr.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"bad poll interval": "default_remote: origin\npoll_interval: 30 seconds\ndefault_timeout: 12h\nmax_consecutive_errors: 3\nforges:\n  - {host: github.com, kind: github, api_base: 'https://api.github.com', token_env: []}\n",
		"unknown key":       "default_remote: origin\npoll_interval: 30s\ndefault_timeout: 12h\nmax_consecutive_errors: 3\nextra: true\nforges:\n  - {host: github.com, kind: github, api_base: 'https://api.github.com', token_env: []}\n",
		"unknown kind":      "default_remote: origin\npoll_interval: 30s\ndefault_timeout: 12h\nmax_consecutive_errors: 3\nforges:\n  - {host: github.com, kind: gitlab, api_base: 'https://api.github.com', token_env: []}\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"spec/pr.yaml":                {Data: []byte(body)},
				"spec/schemas/pr.schema.json": {Data: schema},
			}
			if _, err := loadSpecFrom(fsys, "spec/pr.yaml", "spec/schemas/pr.schema.json"); err == nil {
				t.Fatalf("loadSpecFrom accepted an invalid spec")
			}
		})
	}
}
