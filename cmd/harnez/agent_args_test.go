package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestAssemblePrompt(t *testing.T) {
	dir := t.TempDir()
	one := dir + "/one"
	two := dir + "/two"
	if err := os.WriteFile(one, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name               string
		files, words, tail []string
		stdin, want, err   string
	}{
		{"order", []string{one, two}, []string{"word", "two"}, []string{"tail", "--weird"}, "", "first\n\nsecond\n\nword two\n\ntail --weird", ""},
		{"stdin", []string{"-"}, nil, nil, "from stdin", "from stdin", ""},
		{"missing", []string{dir + "/missing"}, nil, nil, "", "", "read prompt file"},
		{"empty", nil, nil, nil, "", "", "no prompt given"},
		{"dash tail", nil, nil, []string{"-p", "--literal"}, "", "-p --literal", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assemblePrompt(tt.files, tt.words, tt.tail, strings.NewReader(tt.stdin))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, err %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestAgentSessionVerbsRejectPositionalSessions(t *testing.T) {
	for _, args := range [][]string{
		{"stop", "worker"}, {"delete", "worker"}, {"status", "worker"},
		{"compact", "worker"}, {"chat", "attach", "worker"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			cmd := newAgentCmd()
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("positional session unexpectedly accepted")
			}
		})
	}
}

func TestAgentErrorsNameTheFixWithoutUsageDump(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"stop", "foo"}, "session is now --name <session>"},
		{[]string{"delete", "foo"}, "session is now --name <session>"},
		{[]string{"status", "foo"}, "session is now --name <session>"},
		{[]string{"compact", "foo"}, "session is now --name <session>"},
		{[]string{"chat", "codex:luna"}, "model is now --model <spec>"},
		{[]string{"start"}, "no prompt given"},
	} {
		var out, errOut bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs(append(tc.args, "--store-dir", t.TempDir()))
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%v: err = %v, want %q", tc.args, err, tc.want)
		}
		if strings.Contains(out.String()+errOut.String(), "Usage:") {
			t.Fatalf("%v: usage dump in output:\n%s%s", tc.args, out.String(), errOut.String())
		}
	}
}
