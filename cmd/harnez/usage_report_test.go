package main

import (
	"testing"
	"time"
)

func TestParseReportSince(t *testing.T) {
	cases := map[string]time.Duration{
		"7d":  7 * 24 * time.Hour,
		"36h": 36 * time.Hour,
	}
	for in, want := range cases {
		got, err := parseReportSince(in)
		if err != nil || got != want {
			t.Fatalf("parseReportSince(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "0d", "-1d", "xd", "soon", "0s"} {
		if _, err := parseReportSince(bad); err == nil {
			t.Fatalf("parseReportSince(%q) accepted a bad value", bad)
		}
	}
}
