// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestRunInit_GeneratesHarnezRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(agents), cfg.AgentsMD.Rules.Header+"\n\n") {
		t.Fatalf("AGENTS.md missing rules header:\n%s", agents)
	}
	for _, name := range []string{"Index.md", "Tools.md", "Issues.md", "Subagents.md", "Output.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", name)); err != nil {
			t.Errorf("missing generated rule %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Quota.md")); !os.IsNotExist(err) {
		t.Errorf("Quota.md exists without Quota-1 opt-in: %v", err)
	}
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), ".harnez/rules/Local.md\n") {
		t.Fatalf("Local.md missing from git exclude:\n%s", exclude)
	}
	first := string(agents)
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != first {
		t.Fatal("second init changed AGENTS.md")
	}
}

func TestRunInit_MigratesManagedBlocksLosslessly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/migrate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agentPrefix := []byte("OWNER-AGENT-PREFIX\r\nKeep tabs\t and spaces  \r\n")
	agentSuffix := []byte("OWNER-AGENT-SUFFIX\r\nLast owner byte. \t\r\n")
	agents := append([]byte(nil), agentPrefix...)
	agents = append(agents, []byte("\n<!-- harnez:begin Quota-1 Guardrails -->\nquota sticky payload\n<!-- harnez:end Quota-1 Guardrails -->\n\n")...)
	agents = append(agents, []byte("<!-- harnez:begin Repo Setup -->\nrepo sticky payload\n<!-- harnez:end Repo Setup -->\n")...)
	agents = append(agents, agentSuffix...)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), agents, 0o644); err != nil {
		t.Fatal(err)
	}
	localPrefix := []byte("OWNER-LOCAL-PREFIX\r\nPreserve exactly.\r\n")
	localSuffix := []byte("OWNER-LOCAL-SUFFIX\r\nEnd exactly.\r\n")
	local := append([]byte(nil), localPrefix...)
	local = append(local, []byte("\n<!-- harnez:begin Concise Mode -->\nconcise sticky payload\n<!-- harnez:end Concise Mode -->\n\n")...)
	local = append(local, []byte("<!-- harnez:begin Subagent Policy -->\nsubagent sticky payload\n<!-- harnez:end Subagent Policy -->\n")...)
	local = append(local, localSuffix...)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), local, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	run := func() {
		t.Helper()
		if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
			t.Fatal(err)
		}
	}
	run()
	agentsPath := filepath.Join(dir, "AGENTS.md")
	agentsAfter, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, ownerBytes := range [][]byte{agentPrefix, agentSuffix} {
		if !bytes.Contains(agentsAfter, ownerBytes) {
			t.Errorf("AGENTS.md owner bytes changed or disappeared: %q\n%s", ownerBytes, agentsAfter)
		}
	}
	if bytes.Contains(agentsAfter, []byte("quota sticky payload")) || bytes.Contains(agentsAfter, []byte("repo sticky payload")) {
		t.Fatalf("managed payload remains in AGENTS.md:\n%s", agentsAfter)
	}
	localPath := filepath.Join(dir, "AGENTS.local.md")
	localAfter, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, ownerBytes := range [][]byte{localPrefix, localSuffix} {
		if !bytes.Contains(localAfter, ownerBytes) {
			t.Errorf("AGENTS.local.md owner bytes changed or disappeared: %q\n%s", ownerBytes, localAfter)
		}
	}
	if bytes.Contains(localAfter, []byte("concise sticky payload")) || bytes.Contains(localAfter, []byte("subagent sticky payload")) {
		t.Fatalf("known local managed payload remains in AGENTS.local.md:\n%s", localAfter)
	}
	quota, err := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Quota.md"))
	if err != nil {
		t.Fatal(err)
	}
	localRules, err := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Local.md"))
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{"Quota.md": string(quota), "Local.md": string(localRules)} {
		for _, want := range []string{"quota sticky payload", "repo sticky payload", "concise sticky payload", "subagent sticky payload"} {
			if name == "Quota.md" && want == "quota sticky payload" || name == "Local.md" && want != "quota sticky payload" {
				if !strings.Contains(got, want) {
					t.Errorf("%s missing migrated content %q:\n%s", name, want, got)
				}
			}
		}
	}
	firstAgents, firstLocal := append([]byte(nil), agentsAfter...), append([]byte(nil), localAfter...)
	firstLocalRules, firstQuota := append([]byte(nil), localRules...), append([]byte(nil), quota...)
	run()
	secondAgents, _ := os.ReadFile(agentsPath)
	secondLocal, _ := os.ReadFile(localPath)
	secondLocalRules, _ := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Local.md"))
	secondQuota, _ := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Quota.md"))
	for name, pair := range map[string][2][]byte{
		"AGENTS.md": {firstAgents, secondAgents}, "AGENTS.local.md": {firstLocal, secondLocal},
		"Local.md": {firstLocalRules, secondLocalRules}, "Quota.md": {firstQuota, secondQuota},
	} {
		if !bytes.Equal(pair[0], pair[1]) {
			t.Errorf("second init changed %s", name)
		}
	}
}

func TestRunInit_LeavesMalformedNestedMarkersUntouched(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/malformed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	malformed := "OWNER\n<!-- harnez:begin Local Overlays -->\nouter\n<!-- harnez:begin Language Conventions -->\nnested\n<!-- harnez:end Language Conventions -->\n<!-- harnez:end Local Overlays -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(malformed)) {
		t.Fatalf("malformed nested markers or their surrounding bytes changed:\n%s", data)
	}
}

func TestRunInit_GeneratesRulesOutsideGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/nongit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := claude.RunInit(dir, cfg, nil, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Index.md")); err != nil {
		t.Fatalf("missing rules outside git: %v", err)
	}
}
