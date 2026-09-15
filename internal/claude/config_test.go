// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import "testing"

func TestLanguageSourceFor(t *testing.T) {
	cases := []struct {
		name    string
		lang    Language
		variant string
		want    string
	}{
		{"lite set, variant lite", Language{Source: "full.md", LiteSource: "lite.md"}, "lite", "lite.md"},
		{"lite set, variant full", Language{Source: "full.md", LiteSource: "lite.md"}, "full", "full.md"},
		{"lite unset, variant lite falls back", Language{Source: "full.md"}, "lite", "full.md"},
		{"lite unset, variant full", Language{Source: "full.md"}, "full", "full.md"},
		{"lite unset, empty variant", Language{Source: "full.md"}, "", "full.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.lang.SourceFor(c.variant)
			if got != c.want {
				t.Errorf("SourceFor(%q) = %q, want %q", c.variant, got, c.want)
			}
		})
	}
}
