package main

import (
	"bytes"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/usage"
)

func TestFallbackNoticeAndQuotaOverride(t *testing.T) {
	var out bytes.Buffer
	availability := func(provider, model string) usage.ProviderQuotaAvailability {
		if provider == "codex" {
			return usage.ProviderQuotaAvailability{State: "exhausted"}
		}
		return usage.ProviderQuotaAvailability{State: "available"}
	}
	result, err := selectStartModel("developer", "smart", false, availability, &out)
	if err != nil || result.Provider != "claude" || result.Tier != "med" || !strings.Contains(out.String(), "smart → opus:med (sol:med: quota exhausted)") {
		t.Fatalf("selection=%+v err=%v notice=%q", result, err, out.String())
	}
	out.Reset()
	result, err = selectStartModel("developer", "smart", true, availability, &out)
	if err != nil || result.Provider != "codex" || out.Len() != 0 {
		t.Fatalf("override=%+v err=%v notice=%q", result, err, out.String())
	}
}
