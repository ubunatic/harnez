// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCmd_ZeroDefaultDocs(t *testing.T) {
	dir := t.TempDir()
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"apply", "-t", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	goDoc := filepath.Join(dir, "docs", "Go.md")
	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md not to be installed by default on bare apply")
	}
}

func TestApplyCmd_OptInDocs(t *testing.T) {
	dir := t.TempDir()
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"apply", "-t", dir, "-d", "golang"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply -d golang failed: %v", err)
	}

	goDoc := filepath.Join(dir, "docs", "Go.md")
	if _, err := os.Stat(goDoc); err != nil {
		t.Fatalf("expected Go.md to be installed when -d golang passed: %v", err)
	}

	bashDoc := filepath.Join(dir, "docs", "Bash.md")
	if _, err := os.Stat(bashDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Bash.md not to be installed")
	}

	// Now run apply with --clean-docs and no doc flags -> Go.md should be cleaned
	cmd2 := newRootCmd()
	var stdout2, stderr2 bytes.Buffer
	cmd2.SetOut(&stdout2)
	cmd2.SetErr(&stderr2)
	cmd2.SetArgs([]string{"apply", "-t", dir, "--clean-docs"})

	if err := cmd2.Execute(); err != nil {
		t.Fatalf("apply --clean-docs failed: %v", err)
	}

	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md to be cleaned up after apply --clean-docs")
	}
}

func TestApplyCmd_RemoveDocsFlag(t *testing.T) {
	dir := t.TempDir()
	// First install a doc
	cmd := newRootCmd()
	cmd.SetArgs([]string{"apply", "-t", dir, "-d", "golang"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply -d golang failed: %v", err)
	}

	goDoc := filepath.Join(dir, "docs", "Go.md")
	if _, err := os.Stat(goDoc); err != nil {
		t.Fatalf("expected Go.md to exist: %v", err)
	}

	// Now remove with --remove-docs
	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"apply", "-t", dir, "--remove-docs"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("apply --remove-docs failed: %v", err)
	}

	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md to be removed by --remove-docs")
	}
}
