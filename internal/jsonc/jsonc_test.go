// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package jsonc

import (
	"slices"
	"testing"
)

func TestUnionStrings(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: []string{},
		},
		{
			name: "duplicates in a",
			a:    []string{"foo", "foo", "bar"},
			b:    []string{"baz"},
			want: []string{"foo", "bar", "baz"},
		},
		{
			name: "duplicates in b",
			a:    []string{"foo"},
			b:    []string{"bar", "bar", "baz", "baz"},
			want: []string{"foo", "bar", "baz"},
		},
		{
			name: "overlap between a and b",
			a:    []string{"foo", "bar"},
			b:    []string{"bar", "baz"},
			want: []string{"foo", "bar", "baz"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := UnionStrings(tc.a, tc.b)
			if !slices.Equal(got, tc.want) {
				t.Errorf("UnionStrings(%v, %v) = %v; want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestToStrings(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := ToStrings(nil); got != nil {
			t.Errorf("ToStrings(nil) = %v, want nil", got)
		}
	})
	t.Run("string slice", func(t *testing.T) {
		in := []string{"a", "b"}
		if got := ToStrings(in); !slices.Equal(got, in) {
			t.Errorf("ToStrings(%v) = %v, want %v", in, got, in)
		}
	})
	t.Run("any slice", func(t *testing.T) {
		in := []any{"a", 123, "b"}
		want := []string{"a", "b"}
		if got := ToStrings(in); !slices.Equal(got, want) {
			t.Errorf("ToStrings(%v) = %v, want %v", in, got, want)
		}
	})
}
