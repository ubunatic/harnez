package main

import (
	"reflect"
	"testing"
)

func TestRewriteBareNumericLimit(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "bare -N rewritten to --limit N",
			in:   []string{"issues", "-3"},
			want: []string{"issues", "--limit", "3"},
		},
		{
			name: "bare -N among other args",
			in:   []string{"is:open", "-2"},
			want: []string{"is:open", "--limit", "2"},
		},
		{
			name: "explicit -n wins, no rewrite",
			in:   []string{"issues", "-n", "5", "-3"},
			want: []string{"issues", "-n", "5", "-3"},
		},
		{
			name: "explicit --limit wins, no rewrite",
			in:   []string{"issues", "--limit", "5", "-3"},
			want: []string{"issues", "--limit", "5", "-3"},
		},
		{
			name: "explicit --limit= wins, no rewrite",
			in:   []string{"issues", "--limit=5", "-3"},
			want: []string{"issues", "--limit=5", "-3"},
		},
		{
			name: "two bare -N candidates left untouched (ambiguous)",
			in:   []string{"-2", "-3"},
			want: []string{"-2", "-3"},
		},
		{
			name: "hyphenated substring is not a bare -N token",
			in:   []string{"engine-2", "crash"},
			want: []string{"engine-2", "crash"},
		},
		{
			name: "multi-char flag-shaped token is not a bare -N token",
			in:   []string{"-2x"},
			want: []string{"-2x"},
		},
		{
			name: "no bare -N token, no-op",
			in:   []string{"issues", "open"},
			want: []string{"issues", "open"},
		},
		{
			name: "empty args",
			in:   []string{},
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteBareNumericLimit(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rewriteBareNumericLimit(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRewriteArgsForBareLimit(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "find issues -N",
			in:   []string{"find", "issues", "-3"},
			want: []string{"find", "issues", "--limit", "3"},
		},
		{
			name: "find issues query -N",
			in:   []string{"find", "issues", "is:open", "-2"},
			want: []string{"find", "issues", "is:open", "--limit", "2"},
		},
		{
			name: "issues list -N",
			in:   []string{"issues", "list", "-3"},
			want: []string{"issues", "list", "--limit", "3"},
		},
		{
			name: "issues list filter -N",
			in:   []string{"issues", "list", "is:open", "-2"},
			want: []string{"issues", "list", "is:open", "--limit", "2"},
		},
		{
			name: "issues close <n> untouched: not find or issues list",
			in:   []string{"issues", "close", "32"},
			want: []string{"issues", "close", "32"},
		},
		{
			name: "issues mv untouched",
			in:   []string{"issues", "mv", "32", "-5"},
			want: []string{"issues", "mv", "32", "-5"},
		},
		{
			name: "other command untouched",
			in:   []string{"status", "-3"},
			want: []string{"status", "-3"},
		},
		{
			name: "empty args",
			in:   []string{},
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteArgsForBareLimit(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rewriteArgsForBareLimit(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
