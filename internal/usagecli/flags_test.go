package usagecli

import (
	"testing"

	"ubunatic.com/harnez/internal/usage"
)

// TestResolveUsageHost_FlagWinsOverLocalConfig is issue 109's acceptance
// criterion 3: an explicit --host always overrides usage.default_host from
// ~/.config/harnez/local.yaml.
func TestResolveUsageHost_FlagWinsOverLocalConfig(t *testing.T) {
	cfg := &usage.LocalConfig{Usage: usage.LocalUsageConfig{DefaultHost: "fromlocal"}}
	got := ResolveUsageHost("fromflag", cfg)
	if got != "fromflag" {
		t.Fatalf("ResolveUsageHost() = %q, want %q", got, "fromflag")
	}
}

func TestResolveUsageHost_FallsBackToLocalConfig(t *testing.T) {
	cfg := &usage.LocalConfig{Usage: usage.LocalUsageConfig{DefaultHost: "fromlocal"}}
	got := ResolveUsageHost("", cfg)
	if got != "fromlocal" {
		t.Fatalf("ResolveUsageHost() = %q, want %q", got, "fromlocal")
	}
}

func TestResolveUsageHost_NilConfigNoFlag(t *testing.T) {
	got := ResolveUsageHost("", nil)
	if got != "" {
		t.Fatalf("ResolveUsageHost() = %q, want empty", got)
	}
}

func TestResolveUsageHost_EmptyLocalDefault(t *testing.T) {
	cfg := &usage.LocalConfig{}
	got := ResolveUsageHost("", cfg)
	if got != "" {
		t.Fatalf("ResolveUsageHost() = %q, want empty", got)
	}
}

// TestValidateUsageFlags_NoFlagsOK: a bare `harnez usage` uses the spec's
// declared default view and must not be rejected.
func TestValidateUsageFlags_NoFlagsOK(t *testing.T) {
	if err := ValidateUsageFlags(false, false, false, false, false, false); err != nil {
		t.Fatalf("expected no flags to be valid, got error: %v", err)
	}
}

// TestValidateUsageFlags_CompactAloneOK: `--compact` selects the reduced
// panel set without requiring --watch.
func TestValidateUsageFlags_CompactAloneOK(t *testing.T) {
	if err := ValidateUsageFlags(false, false, false, true, false, false); err != nil {
		t.Fatalf("--compact alone should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_CompactAllowedWithWatch(t *testing.T) {
	if err := ValidateUsageFlags(true, false, false, true, false, false); err != nil {
		t.Fatalf("--watch --compact should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_MinimalAllowedWithWatch(t *testing.T) {
	if err := ValidateUsageModeFlags(true, false, false, false); err != nil {
		t.Fatalf("--minimal --watch should be allowed, got error: %v", err)
	}
	if err := ValidateUsageFlags(true, false, false, true, false, false); err != nil {
		t.Fatalf("--minimal's compact panel preset should be allowed with --watch, got error: %v", err)
	}
}

func TestValidateUsageModeFlagsWithNonDashboardOutputRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  bool
		json bool
		loom bool
	}{
		{name: "raw", raw: true},
		{name: "json", json: true},
		{name: "loom", loom: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateUsageModeFlags(true, tc.raw, tc.json, tc.loom); err == nil {
				t.Fatalf("explicit usage mode with --%s should be rejected", tc.name)
			}
		})
	}
}

func TestValidateUsageFlags_SplashAllowedWithWatch(t *testing.T) {
	if err := ValidateUsageFlags(true, false, false, false, false, true); err != nil {
		t.Fatalf("--watch --splash should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_SplashWithoutWatchRejected(t *testing.T) {
	if err := ValidateUsageFlags(false, false, false, false, false, true); err == nil {
		t.Fatalf("expected --splash alone to be rejected")
	}
}

func TestValidateUsageFlags_WatchAndJSONRejected(t *testing.T) {
	if err := ValidateUsageFlags(true, false, true, false, false, false); err == nil {
		t.Fatalf("expected --watch and --json together to be rejected")
	}
}

func TestValidateUsageFlags_WatchAndRawRejected(t *testing.T) {
	if err := ValidateUsageFlags(true, true, false, false, false, false); err == nil {
		t.Fatalf("expected --watch and --raw together to be rejected")
	}
}

func TestValidateUsageFlags_RawAndJSONRejected(t *testing.T) {
	if err := ValidateUsageFlags(false, true, true, false, false, false); err == nil {
		t.Fatalf("expected --raw and --json together to be rejected")
	}
}

func TestValidateUsageFlags_CompactWithRawRejected(t *testing.T) {
	if err := ValidateUsageFlags(false, true, false, true, false, false); err == nil {
		t.Fatalf("expected --compact and --raw together to be rejected (--raw has no panel concept)")
	}
}

func TestSelectUsageViewMode(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		normal, compact, minimal bool
		want                     string
		wantExplicit             bool
		wantErr                  bool
	}{
		{name: "default", wantExplicit: false},
		{name: "normal", normal: true, want: "normal", wantExplicit: true},
		{name: "compact", compact: true, want: "compact", wantExplicit: true},
		{name: "minimal", minimal: true, want: "minimal", wantExplicit: true},
		{name: "conflicting", normal: true, compact: true, wantExplicit: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, explicit, err := SelectUsageViewMode(tc.normal, tc.compact, tc.minimal)
			if (err != nil) != tc.wantErr || got != tc.want || explicit != tc.wantExplicit {
				t.Fatalf("SelectUsageViewMode() = (%q, %t, %v), want (%q, %t, err=%t)", got, explicit, err, tc.want, tc.wantExplicit, tc.wantErr)
			}
		})
	}
}

func TestValidateUsageFlags_RawAlone_OK(t *testing.T) {
	if err := ValidateUsageFlags(false, true, false, false, false, false); err != nil {
		t.Fatalf("--raw alone should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_LoomAlone_OK(t *testing.T) {
	if err := ValidateUsageFlags(false, false, false, false, true, false); err != nil {
		t.Fatalf("--loom alone should be allowed, got error: %v", err)
	}
}

func TestValidateUsageFlags_LoomWithRawRejected(t *testing.T) {
	if err := ValidateUsageFlags(false, true, false, false, true, false); err == nil {
		t.Fatalf("expected --loom and --raw together to be rejected")
	}
}

func TestValidateUsageFlags_LoomWithJSONRejected(t *testing.T) {
	if err := ValidateUsageFlags(false, false, true, false, true, false); err == nil {
		t.Fatalf("expected --loom and --json together to be rejected")
	}
}
