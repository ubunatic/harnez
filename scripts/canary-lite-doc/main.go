// canary-lite-doc — behavioral canary for a lite doc variant.
//
// Spawns a genuinely isolated `claude -p` session (a scratch directory
// outside any harnez-managed project, so no local AGENTS.md/CLAUDE.md is
// auto-injected) whose ONLY context is the given lite doc, hands it a coding
// task exercising that doc's rules, then lints the real output file with
// internal/lint (the same rules `harnez lint` uses) as the mechanical judge
// — no custom LLM-response scoring, no parsing of the model's free-text
// reply. See issue 362.
//
// A prior bash implementation of this canary parsed the agent's chat reply
// for the output file path (`tail -1`), which broke whenever the model
// printed trailing commentary after the path instead of before it. Since
// every fixture task already names its own output file explicitly (e.g.
// "write to ./deploy-check.sh"), this version just checks for that expected
// path directly — no text parsing of model output at all.
//
// Known limitation: if the agent interprets a task's stated filename loosely
// (writes to a subdirectory it creates, or a different name/extension it
// judges more idiomatic), this fails with a generic "did not produce
// expected output file" error rather than distinguishing "ignored
// instructions" from "did something reasonable but different." Strictly
// better than the old failure mode (a silently wrong parsed path), but not
// exhaustively swept — worth revisiting if a fixture ever flakes this way.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"ubunatic.com/harnez/internal/lint"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: canary-lite-doc <lite-doc-path> <task-file> <output-filename>\nexample: canary-lite-doc docs/lang/Bash.lite.md scripts/canary-lite-doc/fixtures/bash-deploy-check.task.md deploy-check.sh")
	}
	docPath, taskPath, outName := args[0], args[1], args[2]

	docAbs, err := filepath.Abs(docPath)
	if err != nil {
		return fmt.Errorf("resolve doc path: %w", err)
	}
	if _, err := os.Stat(docAbs); err != nil {
		return fmt.Errorf("lite doc not found: %w", err)
	}
	taskText, err := os.ReadFile(taskPath)
	if err != nil {
		return fmt.Errorf("read task file: %w", err)
	}

	work, err := os.MkdirTemp("", "canary-lite-doc.*")
	if err != nil {
		return fmt.Errorf("create scratch dir: %w", err)
	}
	defer os.RemoveAll(work)

	if err := copyFile(docAbs, filepath.Join(work, "STYLE.md")); err != nil {
		return fmt.Errorf("copy lite doc into scratch dir: %w", err)
	}

	prompt := fmt.Sprintf(`You are in an empty directory with one file, STYLE.md — that is your
only style guide, and the only context you have. Read STYLE.md, then
complete this task:

%s

Write the file to disk.`, taskText)

	fmt.Printf("Running isolated agent in %s ...\n", work)
	cmd := exec.Command("claude", "-p", "--permission-mode", "bypassPermissions", prompt)
	cmd.Dir = work
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude -p: %w", err)
	}

	outPath := filepath.Join(work, outName)
	content, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("agent did not produce expected output file %s: %w", outPath, err)
	}

	fmt.Printf("\n=== generated file: %s ===\n%s\n", outPath, content)

	findings := lint.DefaultLinter().LintBytes(outName, content, lint.LangAuto)
	fmt.Println("=== lint findings ===")
	if len(findings) == 0 {
		fmt.Printf("PASS: %s (doc=%s, task=%s)\n", outName, docPath, taskPath)
		return nil
	}
	for _, f := range findings {
		fmt.Printf("%s:%d:%d: %s [%s]\n", outName, f.Line, f.Col, f.Message, f.RuleID)
	}
	return fmt.Errorf("FAIL: %d lint finding(s) for %s (doc=%s, task=%s)", len(findings), outName, docPath, taskPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
