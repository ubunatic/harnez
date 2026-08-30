package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalConfigDir_XDGSet(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	got := LocalConfigDir("")
	want := filepath.Join(tmp, "harnez")
	if got != want {
		t.Fatalf("LocalConfigDir() = %q, want %q", got, want)
	}
}

func TestLocalConfigDir_XDGUnsetFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	got := LocalConfigDir(home)
	want := filepath.Join(home, ".config", "harnez")
	if got != want {
		t.Fatalf("LocalConfigDir() = %q, want %q", got, want)
	}
}

func TestLocalConfigPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	got := LocalConfigPath("")
	want := filepath.Join(tmp, "harnez", "local.yaml")
	if got != want {
		t.Fatalf("LocalConfigPath() = %q, want %q", got, want)
	}
}

func TestLoadLocalConfig_Absent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	cfg, path, err := LoadLocalConfig("")
	if err != nil {
		t.Fatalf("expected no error for absent file, got %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil cfg for absent file, got %+v", cfg)
	}
	wantPath := filepath.Join(tmp, "harnez", "local.yaml")
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}
}

func TestLoadLocalConfig_Valid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir := filepath.Join(tmp, "harnez")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "usage:\n  default_host: um760\n"
	if err := os.WriteFile(filepath.Join(dir, "local.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadLocalConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil cfg")
	}
	if cfg.Usage.DefaultHost != "um760" {
		t.Fatalf("DefaultHost = %q, want %q", cfg.Usage.DefaultHost, "um760")
	}
}

func TestLoadLocalConfig_WithLoadWatchHost(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir := filepath.Join(tmp, "harnez")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "usage:\n  default_host: um760\nload:\n  watch_host: llm-box\n"
	if err := os.WriteFile(filepath.Join(dir, "local.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadLocalConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil cfg")
	}
	if cfg.Usage.DefaultHost != "um760" {
		t.Fatalf("DefaultHost = %q, want %q", cfg.Usage.DefaultHost, "um760")
	}
	if cfg.Load.WatchHost != "llm-box" {
		t.Fatalf("Load.WatchHost = %q, want %q", cfg.Load.WatchHost, "llm-box")
	}
}

func TestLoadLocalConfig_LoadSectionOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir := filepath.Join(tmp, "harnez")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "load:\n  watch_host: llm-box\n"
	if err := os.WriteFile(filepath.Join(dir, "local.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadLocalConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil cfg")
	}
	if cfg.Usage.DefaultHost != "" {
		t.Fatalf("DefaultHost = %q, want empty", cfg.Usage.DefaultHost)
	}
	if cfg.Load.WatchHost != "llm-box" {
		t.Fatalf("Load.WatchHost = %q, want %q", cfg.Load.WatchHost, "llm-box")
	}
}

func TestLoadLocalConfig_Malformed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir := filepath.Join(tmp, "harnez")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "usage: [this is not a mapping\n"
	if err := os.WriteFile(filepath.Join(dir, "local.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := LoadLocalConfig("")
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if cfg != nil {
		t.Fatalf("expected nil cfg on error, got %+v", cfg)
	}
}
