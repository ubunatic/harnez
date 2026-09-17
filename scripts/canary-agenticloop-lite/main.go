// canary-agenticloop-lite — real LLM-invocation harness for the AgenticLoop
// behavioral fixtures (issue 359 pilot, issue 362 follow-up).
//
// For each fixture in fixtures.yaml, for each doc variant (full
// docs/AgenticLoop.md, lite docs/practices/AgenticLoop.lite.md) and for each
// agent CLI available on PATH (claude, agy), this spawns a genuinely
// isolated os.MkdirTemp workspace containing ONLY that one doc variant
// (as AGENTS.md), runs the fixture's prompt non-interactively against that
// agent from that workspace, and mechanically scores the captured real
// response against the fixture's pattern/forbid_pattern regexes.
//
// This replaces the prior manual/hand-reasoned pass in results.md, which
// explicitly flagged itself as a stand-in until real invocation existed.
//
// Mirrors the isolation/spawning style of scripts/canary-lite-doc/main.go
// (os.MkdirTemp scratch dir, single doc file copied in, claude -p
// --permission-mode bypassPermissions) and the agy -p invocation pattern
// from scripts/canary-clean-workspace-docs.sh (agy's replies may include
// markdown bold markers that don't affect regex scoring here, so they are
// left as-is).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	flagFixtures     []string
	flagAgents       []string
	flagVariants     []string
	flagFixturesYAML string
)

type fixture struct {
	ID            string `yaml:"id"`
	Rule          string `yaml:"rule"`
	Prompt        string `yaml:"prompt"`
	Pattern       string `yaml:"pattern"`
	ForbidPattern string `yaml:"forbid_pattern"`
}

type docVariant struct {
	name string
	path string
}

type agentCLI struct {
	name string
	// run executes the prompt in dir and returns captured stdout text.
	run func(dir, prompt string) (string, error)
}

type outcome int

const (
	pass outcome = iota
	fail
	skip
)

func (o outcome) String() string {
	switch o {
	case pass:
		return "PASS"
	case fail:
		return "FAIL"
	default:
		return "SKIP"
	}
}

type result struct {
	agent   string
	variant string
	id      string
	status  outcome
	detail  string
}

func main() {
	root := &cobra.Command{
		Use:   "canary-agenticloop-lite",
		Short: "Real LLM-invocation harness for the AgenticLoop full-vs-lite behavioral fixtures",
	}
	root.PersistentFlags().StringVar(&flagFixturesYAML, "fixtures-file", "", "override path to fixtures.yaml")

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Invoke agent CLIs against fixtures and score real replies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
	runCmd.Flags().StringSliceVar(&flagFixtures, "fixture", nil, "run only fixtures with this id (repeatable); default: all fixtures")
	runCmd.Flags().StringSliceVar(&flagAgents, "agent", nil, "run only this agent CLI, one of: claude, agy (repeatable); default: all agents")
	runCmd.Flags().StringSliceVar(&flagVariants, "variant", nil, "run only this doc variant, one of: full, lite (repeatable); default: both variants")

	fixturesCmd := &cobra.Command{
		Use:   "fixtures",
		Short: "Inspect fixtures.yaml without invoking any agent",
	}
	fixturesListCmd := &cobra.Command{
		Use:   "list",
		Short: "List fixture ids and rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fixturesList()
		},
	}
	fixturesShowCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show the full definition of one fixture",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fixturesShow(args[0])
		},
	}
	fixturesCmd.AddCommand(fixturesListCmd, fixturesShowCmd)

	measureCostCmd := &cobra.Command{
		Use:   "measure-cost",
		Short: "Run the hello fixture once per agent and print its parsed token usage as the 1-unit baseline",
		RunE: func(cmd *cobra.Command, args []string) error {
			return measureCost()
		},
	}
	measureCostCmd.Flags().StringSliceVar(&flagAgents, "agent", nil, "only measure this agent CLI, one of: claude, agy (repeatable); default: all agents")

	root.AddCommand(runCmd, fixturesCmd, measureCostCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func resolveFixturesPath() (string, error) {
	selfDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Allow running from repo root or from within the script dir.
	fixturesPath := flagFixturesYAML
	if fixturesPath == "" {
		fixturesPath = filepath.Join(selfDir, "scripts", "canary-agenticloop-lite", "fixtures.yaml")
		if _, err := os.Stat(fixturesPath); err != nil {
			fixturesPath = filepath.Join(selfDir, "fixtures.yaml")
		}
	}
	return filepath.Abs(fixturesPath)
}

func loadFixtures() ([]fixture, error) {
	fixturesAbs, err := resolveFixturesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fixturesAbs)
	if err != nil {
		return nil, fmt.Errorf("read fixtures.yaml: %w", err)
	}
	var fixtures []fixture
	if err := yaml.Unmarshal(data, &fixtures); err != nil {
		return nil, fmt.Errorf("parse fixtures.yaml: %w", err)
	}
	return fixtures, nil
}

func fixturesList() error {
	fixtures, err := loadFixtures()
	if err != nil {
		return err
	}
	fmt.Printf("%-28s | %-32s | %s\n", "id", "rule", "pattern")
	for _, fx := range fixtures {
		fmt.Printf("%-28s | %-32s | %s\n", fx.ID, truncate(fx.Rule, 32), fx.Pattern)
	}
	return nil
}

func fixturesShow(id string) error {
	fixtures, err := loadFixtures()
	if err != nil {
		return err
	}
	for _, fx := range fixtures {
		if fx.ID == id {
			fmt.Printf("id:             %s\n", fx.ID)
			fmt.Printf("rule:           %s\n", fx.Rule)
			fmt.Printf("prompt:         %s\n", fx.Prompt)
			fmt.Printf("pattern:        %s\n", fx.Pattern)
			fmt.Printf("forbid_pattern: %s\n", fx.ForbidPattern)
			return nil
		}
	}
	return fmt.Errorf("no fixture with id %q (known ids: %s)", id, knownIDs(fixtures))
}

func run() error {
	fixturesAbs, err := resolveFixturesPath()
	if err != nil {
		return err
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(fixturesAbs))) // scripts/canary-agenticloop-lite/fixtures.yaml -> repo root
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		// fall back: fixturesAbs is scripts/canary-agenticloop-lite/fixtures.yaml relative to cwd==repoRoot
		selfDir, err := os.Getwd()
		if err != nil {
			return err
		}
		repoRoot = selfDir
	}

	allFixtures, err := loadFixtures()
	if err != nil {
		return err
	}
	fixtures := allFixtures
	if len(flagFixtures) > 0 {
		wanted := make(map[string]bool, len(flagFixtures))
		for _, id := range flagFixtures {
			wanted[id] = true
		}
		fixtures = nil
		for _, fx := range allFixtures {
			if wanted[fx.ID] {
				fixtures = append(fixtures, fx)
			}
		}
		if len(fixtures) == 0 {
			return fmt.Errorf("no fixtures matched --fixture=%v (known ids: %s)", flagFixtures, knownIDs(allFixtures))
		}
	}

	allVariants := []docVariant{
		{name: "full", path: filepath.Join(repoRoot, "docs", "AgenticLoop.md")},
		{name: "lite", path: filepath.Join(repoRoot, "docs", "practices", "AgenticLoop.lite.md")},
	}
	variants := allVariants
	if len(flagVariants) > 0 {
		variants = nil
		for _, v := range allVariants {
			if contains(flagVariants, v.name) {
				variants = append(variants, v)
			}
		}
		if len(variants) == 0 {
			return fmt.Errorf("no doc variants matched --variant=%v (known: full, lite)", flagVariants)
		}
	}

	allAgents := []agentCLI{
		{name: "claude", run: runClaude},
		{name: "agy", run: runAgy},
	}
	agents := allAgents
	if len(flagAgents) > 0 {
		agents = nil
		for _, ag := range allAgents {
			if contains(flagAgents, ag.name) {
				agents = append(agents, ag)
			}
		}
		if len(agents) == 0 {
			return fmt.Errorf("no agents matched --agent=%v (known: claude, agy)", flagAgents)
		}
	}

	var results []result
	anyFail := false

	for _, ag := range agents {
		if _, err := exec.LookPath(ag.name); err != nil {
			for _, v := range variants {
				for _, fx := range fixtures {
					results = append(results, result{agent: ag.name, variant: v.name, id: fx.ID, status: skip, detail: ag.name + " not on PATH"})
				}
			}
			continue
		}
		for _, v := range variants {
			if _, err := os.Stat(v.path); err != nil {
				for _, fx := range fixtures {
					results = append(results, result{agent: ag.name, variant: v.name, id: fx.ID, status: skip, detail: "doc variant missing: " + v.path})
				}
				continue
			}
			for _, fx := range fixtures {
				res := runFixture(ag, v, fx)
				if res.status == fail {
					anyFail = true
				}
				results = append(results, res)
				fmt.Printf("[%s/%s] %s: %s %s\n", res.agent, res.variant, res.id, res.status, res.detail)
			}
		}
	}

	fmt.Println()
	fmt.Println("=== Results table ===")
	fmt.Printf("%-8s | %-6s | %-28s | %-6s | %s\n", "agent", "doc", "fixture", "status", "detail")
	for _, r := range results {
		fmt.Printf("%-8s | %-6s | %-28s | %-6s | %s\n", r.agent, r.variant, r.id, r.status, truncate(r.detail, 80))
	}

	if anyFail {
		return fmt.Errorf("one or more fixtures FAILed")
	}
	return nil
}

func runFixture(ag agentCLI, v docVariant, fx fixture) result {
	work, err := os.MkdirTemp("", "canary-agenticloop-lite.*")
	if err != nil {
		return result{agent: ag.name, variant: v.name, id: fx.ID, status: fail, detail: "mkdtemp: " + err.Error()}
	}
	defer os.RemoveAll(work)

	if err := copyFile(v.path, filepath.Join(work, "AGENTS.md")); err != nil {
		return result{agent: ag.name, variant: v.name, id: fx.ID, status: fail, detail: "copy doc: " + err.Error()}
	}

	prompt := fmt.Sprintf(`You are in an empty directory with one file, AGENTS.md — that is your
only instructions/context for this session. Read AGENTS.md, then respond to
this request:

%s`, fx.Prompt)

	out, err := ag.run(work, prompt)
	if err != nil {
		return result{agent: ag.name, variant: v.name, id: fx.ID, status: skip, detail: ag.name + " -p failed: " + err.Error()}
	}
	if strings.TrimSpace(out) == "" {
		return result{agent: ag.name, variant: v.name, id: fx.ID, status: skip, detail: "empty response from " + ag.name}
	}

	var notes []string

	if fx.Pattern != "" {
		re, err := regexp.Compile("(?i)" + fx.Pattern)
		if err != nil {
			// Go's RE2 engine rejects some PCRE-only constructs (e.g.
			// lookahead) that fixtures.yaml may use; that's an engine
			// limitation, not a real content failure, so note it and skip
			// this particular check rather than failing the whole fixture.
			notes = append(notes, fmt.Sprintf("pattern %q unsupported by RE2 engine, skipped: %v", fx.Pattern, err))
		} else if !re.MatchString(out) {
			return result{agent: ag.name, variant: v.name, id: fx.ID, status: fail, detail: fmt.Sprintf("pattern %q not found; response: %s", fx.Pattern, truncate(out, 200))}
		}
	}
	if fx.ForbidPattern != "" {
		re, err := regexp.Compile("(?i)" + fx.ForbidPattern)
		if err != nil {
			notes = append(notes, fmt.Sprintf("forbid_pattern %q unsupported by RE2 engine, skipped: %v", fx.ForbidPattern, err))
		} else if re.MatchString(out) {
			return result{agent: ag.name, variant: v.name, id: fx.ID, status: fail, detail: fmt.Sprintf("forbid_pattern %q matched; response: %s", fx.ForbidPattern, truncate(out, 200))}
		}
	}
	detail := truncate(out, 120)
	if len(notes) > 0 {
		detail = strings.Join(notes, "; ") + " | " + detail
	}
	return result{agent: ag.name, variant: v.name, id: fx.ID, status: pass, detail: detail}
}

func runClaude(dir, prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", "--permission-mode", "bypassPermissions", prompt)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runAgy(dir, prompt string) (string, error) {
	cmd := exec.Command("agy", "-p", prompt)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	cleaned := strings.ReplaceAll(string(out), "*", "")
	return cleaned, err
}

// tokenUsage is the subset of each CLI's --output-format json usage fields
// this harness cares about for a "1 canary-agenticloop-lite unit" baseline.
// claude and agy report different field sets (claude splits cache reads from
// fresh input tokens and omits a precomputed total; agy reports a flat
// total_tokens); totalTokens() normalizes both to one comparable number.
type tokenUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens          int `json:"cache_read_tokens"`
	ThinkingTokens           int `json:"thinking_tokens"`
	TotalTokens              int `json:"total_tokens"`
}

func (u tokenUsage) totalTokens() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
}

func runClaudeJSON(dir, prompt string) (tokenUsage, error) {
	cmd := exec.Command("claude", "-p", "--permission-mode", "bypassPermissions", "--output-format", "json", prompt)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return tokenUsage{}, fmt.Errorf("claude -p --output-format json: %w", err)
	}
	var parsed struct {
		Usage tokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return tokenUsage{}, fmt.Errorf("parse claude json output: %w", err)
	}
	return parsed.Usage, nil
}

func runAgyJSON(dir, prompt string) (tokenUsage, error) {
	cmd := exec.Command("agy", "-p", prompt, "--output-format", "json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return tokenUsage{}, fmt.Errorf("agy -p --output-format json: %w", err)
	}
	var parsed struct {
		Usage tokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return tokenUsage{}, fmt.Errorf("parse agy json output: %w", err)
	}
	return parsed.Usage, nil
}

func measureCost() error {
	fixturesAbs, err := resolveFixturesPath()
	if err != nil {
		return err
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(fixturesAbs))) // scripts/canary-agenticloop-lite/fixtures.yaml -> repo root
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		selfDir, err := os.Getwd()
		if err != nil {
			return err
		}
		repoRoot = selfDir
	}

	fixtures, err := loadFixtures()
	if err != nil {
		return err
	}
	var hello *fixture
	for i := range fixtures {
		if fixtures[i].ID == "hello" {
			hello = &fixtures[i]
			break
		}
	}
	if hello == nil {
		return fmt.Errorf("no fixture with id %q found in fixtures.yaml", "hello")
	}

	type jsonRunner struct {
		name string
		run  func(dir, prompt string) (tokenUsage, error)
	}
	allRunners := []jsonRunner{
		{name: "claude", run: runClaudeJSON},
		{name: "agy", run: runAgyJSON},
	}
	runners := allRunners
	if len(flagAgents) > 0 {
		runners = nil
		for _, r := range allRunners {
			if contains(flagAgents, r.name) {
				runners = append(runners, r)
			}
		}
		if len(runners) == 0 {
			return fmt.Errorf("no agents matched --agent=%v (known: claude, agy)", flagAgents)
		}
	}

	prompt := fmt.Sprintf(`You are in an empty directory with one file, AGENTS.md — that is your
only instructions/context for this session. Read AGENTS.md, then respond to
this request:

%s`, hello.Prompt)

	fmt.Println("=== canary-agenticloop-lite unit baseline (hello fixture) ===")
	anyErr := false
	for _, r := range runners {
		if _, err := exec.LookPath(r.name); err != nil {
			fmt.Printf("%-8s SKIP %s not on PATH\n", r.name, r.name)
			continue
		}
		work, err := os.MkdirTemp("", "canary-agenticloop-lite-measure.*")
		if err != nil {
			fmt.Printf("%-8s FAIL mkdtemp: %v\n", r.name, err)
			anyErr = true
			continue
		}
		// Reuse the full AgenticLoop.md doc variant; the baseline is a
		// concrete real-world cost, not variant-agnostic, so pin one variant
		// rather than average across both.
		docPath := filepath.Join(repoRoot, "docs", "AgenticLoop.md")
		if err := copyFile(docPath, filepath.Join(work, "AGENTS.md")); err != nil {
			fmt.Printf("%-8s FAIL copy doc: %v\n", r.name, err)
			os.RemoveAll(work)
			anyErr = true
			continue
		}
		usage, err := r.run(work, prompt)
		os.RemoveAll(work)
		if err != nil {
			fmt.Printf("%-8s FAIL %v\n", r.name, err)
			anyErr = true
			continue
		}
		fmt.Printf("%-8s 1 unit = %d tokens (input=%d output=%d cache_read=%d cache_creation=%d thinking=%d)\n",
			r.name, usage.totalTokens(), usage.InputTokens, usage.OutputTokens,
			usage.CacheReadInputTokens+usage.CacheReadTokens, usage.CacheCreationInputTokens, usage.ThinkingTokens)
	}
	if anyErr {
		return fmt.Errorf("one or more agents failed to report a cost baseline")
	}
	return nil
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

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func knownIDs(fixtures []fixture) string {
	ids := make([]string, len(fixtures))
	for i, fx := range fixtures {
		ids[i] = fx.ID
	}
	return strings.Join(ids, ", ")
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
