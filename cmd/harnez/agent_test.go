package main

import (
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentCommandSurface(t *testing.T) {
	c := newAgentCmd()
	if len(c.Commands()) == 0 {
		t.Fatal("agent command has no children")
	}
	for _, name := range []string{"start", "resume", "list", "status", "compact", "stop", "delete"} {
		found := false
		for _, child := range c.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("missing agent subcommand %q", name)
		}
	}
}

func TestAgentHaikuAliases(t *testing.T) {
	for _, spec := range []string{"claude:haiku", "claude:haiku:latest"} {
		m, err := subagent.ResolveModel(spec)
		if err != nil || m.Provider != "claude" || m.Name != "haiku" || m.Tier != "low" {
			t.Fatalf("%s resolved to %#v (%v)", spec, m, err)
		}
	}
}
