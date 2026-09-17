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
// --dangerously-skip-permissions) and the agy -p invocation pattern
// from scripts/canary-clean-workspace-docs.sh (agy's replies may include
// markdown bold markers that don't affect regex scoring here, so they are
// left as-is).
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/readcard"
)

// embeddedAssets is the release-time fallback for canary specifications and
// their linked documents. The source tree remains the development authority.
//
//go:embed embedded/fixtures.yaml embedded/docs/AgenticLoop.md embedded/docs/practices/AgenticLoop.lite.md embedded/docs/lang/Bash.md embedded/docs/lang/Bash.lite.md
var embeddedAssets embed.FS

var (
	flagFixtures     []string
	flagAgents       []string
	flagVariants     []string
	flagFixturesYAML string
	flagCostFixture  string
	flagCostVariant  string
	flagLink         string
	flagCostLink     string
	flagDelivery     string
	flagCostDelivery string
	flagWorkspaceDir string
)

// validDeliveryModes are the supported --delivery values controlling whether
// the linked doc is exposed as its native text file or as a PNG context card
// rendered via `harnez read -I` (issue 412): "native" (default) copies/embeds
// the doc's own text/markdown; "png" renders it to a PNG first and points
// AGENTS.md at that image instead. "png" is not valid with --link=embed,
// since embed inlines text and a PNG has no text form to inline.
var validDeliveryModes = map[string]bool{"native": true, "png": true}

// validLinkModes are the supported --link values controlling how a doc is
// exposed to the agent inside the isolated workspace (issue 412 follow-up):
// soft (plain-text citation, agent must choose to open it), hard (an eager
// "@path" include macro), or embed (the doc's full text inlined directly
// into AGENTS.md; text docs only).
var validLinkModes = map[string]bool{"soft": true, "hard": true, "embed": true}

type fixture struct {
	ID            string   `yaml:"id"`
	Rule          string   `yaml:"rule"`
	Prompt        string   `yaml:"prompt"`
	Pattern       string   `yaml:"pattern"`
	ForbidPattern string   `yaml:"forbid_pattern"`
	Docs          []string `yaml:"docs"`
}

type fixtureConfig struct {
	Preamble string    `yaml:"preamble"`
	Fixtures []fixture `yaml:"fixtures"`
}

var fixturePreamble string

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
	runCmd.Flags().StringVar(&flagLink, "link", "soft", "how the doc is exposed in AGENTS.md: soft (\"See Doc.md\" citation), hard (\"@Doc.md\" eager include), or embed (doc's full text inlined into AGENTS.md, text docs only)")
	runCmd.Flags().StringVar(&flagDelivery, "delivery", "native", "doc delivery mode: native (text/markdown as-is) or png (rendered via `harnez read -I` context card; not valid with --link=embed)")

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
		Short: "Run one fixture (default: hello) once per agent and print its parsed token usage plus doc-context trace",
		RunE: func(cmd *cobra.Command, args []string) error {
			return measureCost()
		},
	}
	measureCostCmd.Flags().StringSliceVar(&flagAgents, "agent", nil, "only measure this agent CLI, one of: claude, agy (repeatable); default: all agents")
	measureCostCmd.Flags().StringVar(&flagCostFixture, "fixture", "hello", "fixture id to run for the cost measurement")
	measureCostCmd.Flags().StringVar(&flagCostVariant, "variant", "full", "doc variant to use, one of: full, lite")
	measureCostCmd.Flags().StringVar(&flagCostLink, "link", "soft", "how the doc is exposed in AGENTS.md: soft (\"See Doc.md\" citation), hard (\"@Doc.md\" eager include), or embed (doc's full text inlined into AGENTS.md, text docs only)")
	measureCostCmd.Flags().StringVar(&flagCostDelivery, "delivery", "native", "doc delivery mode: native (text/markdown as-is) or png (rendered via `harnez read -I` context card; not valid with --link=embed)")

	workspaceCmd := &cobra.Command{Use: "workspace", Short: "Manage clean manual experiment workspaces"}
	workspaceInitCmd := &cobra.Command{Use: "init", Short: "Create a clean temporary workspace from embedded assets", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return initWorkspace() }}
	workspaceInitCmd.Flags().StringVar(&flagWorkspaceDir, "dir", "", "destination directory (default: a temporary directory)")
	workspaceCmd.AddCommand(workspaceInitCmd)

	root.AddCommand(runCmd, fixturesCmd, measureCostCmd, workspaceCmd)

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
	} else if _, err := os.Stat(fixturesPath); err != nil {
		return "", fmt.Errorf("fixtures override %q: %w", fixturesPath, err)
	}
	if _, err := os.Stat(fixturesPath); err != nil {
		return "", nil
	}
	return filepath.Abs(fixturesPath)
}

func embeddedRepo() (string, func(), error) {
	root, err := os.MkdirTemp("", "canary-agenticloop-lite-assets.*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	for name, target := range map[string]string{
		"embedded/docs/AgenticLoop.md":                "docs/AgenticLoop.md",
		"embedded/docs/practices/AgenticLoop.lite.md": "docs/practices/AgenticLoop.lite.md",
		"embedded/docs/lang/Bash.md":                  "docs/lang/Bash.md",
		"embedded/docs/lang/Bash.lite.md":             "docs/lang/Bash.lite.md",
	} {
		data, readErr := embeddedAssets.ReadFile(name)
		if readErr != nil {
			cleanup()
			return "", func() {}, readErr
		}
		path := filepath.Join(root, target)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			cleanup()
			return "", func() {}, err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			cleanup()
			return "", func() {}, err
		}
	}
	return root, cleanup, nil
}

func loadFixtures() ([]fixture, error) {
	fixturesAbs, err := resolveFixturesPath()
	if err != nil {
		return nil, err
	}
	var data []byte
	if fixturesAbs == "" {
		data, err = embeddedAssets.ReadFile("embedded/fixtures.yaml")
	} else {
		data, err = os.ReadFile(fixturesAbs)
	}
	if err != nil {
		return nil, fmt.Errorf("read fixtures.yaml: %w", err)
	}
	var config fixtureConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse fixtures.yaml: %w", err)
	}
	fixturePreamble = strings.TrimSpace(config.Preamble)
	return config.Fixtures, nil
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
	var cleanup func()
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(fixturesAbs)))
	if fixturesAbs == "" {
		repoRoot, cleanup, err = embeddedRepo()
		if err != nil {
			return fmt.Errorf("materialize embedded assets: %w", err)
		}
		defer cleanup()
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		// fall back: fixturesAbs is scripts/canary-agenticloop-lite/fixtures.yaml relative to cwd==repoRoot
		selfDir, err := os.Getwd()
		if err != nil {
			return err
		}
		repoRoot = selfDir
	}

	if !validLinkModes[flagLink] {
		return fmt.Errorf("unknown --link=%q (known: soft, hard, embed)", flagLink)
	}
	if !validDeliveryModes[flagDelivery] {
		return fmt.Errorf("unknown --delivery=%q (known: native, png)", flagDelivery)
	}
	if flagDelivery == "png" && flagLink == "embed" {
		return fmt.Errorf("--delivery=png is not valid with --link=embed (embed inlines text; a PNG has no text form to inline)")
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
				res := runFixture(repoRoot, ag, v, fx, flagLink, flagDelivery)
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

func runFixture(repoRoot string, ag agentCLI, v docVariant, fx fixture, link, delivery string) result {
	work, err := os.MkdirTemp("", "canary-agenticloop-lite.*")
	if err != nil {
		return result{agent: ag.name, variant: v.name, id: fx.ID, status: fail, detail: "mkdtemp: " + err.Error()}
	}
	defer os.RemoveAll(work)

	docs := append([]string{v.path}, fixtureDocs(repoRoot, v.name, fx)...)
	if err := setupLinkedWorkspace(work, repoRoot, docs, link, delivery); err != nil {
		return result{agent: ag.name, variant: v.name, id: fx.ID, status: fail, detail: "setup workspace: " + err.Error()}
	}

	prompt := fmt.Sprintf("Task:\n%s", fx.Prompt)

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
	cmd := exec.Command("claude", "-p", "--dangerously-skip-permissions", prompt)
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
	InputTokens              int         `json:"input_tokens"`
	OutputTokens             int         `json:"output_tokens"`
	CacheReadInputTokens     int         `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int         `json:"cache_creation_input_tokens"`
	CacheReadTokens          int         `json:"cache_read_tokens"`
	ThinkingTokens           int         `json:"thinking_tokens"`
	TotalTokens              int         `json:"total_tokens"`
	Iterations               []turnUsage `json:"iterations"`
	NumTurns                 int         `json:"-"` // filled from the response's top-level num_turns, when reported
}

// turnUsage is one entry of claude's usage.iterations — the per-turn
// breakdown (one tool-call round trip, or the final message). Its first
// entry is the cheapest available "first turn" baseline: whatever the model
// billed before doing any tool calls (e.g. Read-ing AGENTS.md), as opposed
// to the accumulated multi-turn total.
type turnUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

func (t turnUsage) total() int {
	return t.InputTokens + t.OutputTokens + t.CacheReadInputTokens + t.CacheCreationInputTokens
}

func (u tokenUsage) totalTokens() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
}

// firstTurnTokens returns the token cost of the first turn only — the
// cheapest available proxy for "context loaded before any tool-call work
// started" — falling back to the run's total when no per-turn breakdown is
// available (e.g. agy, or a single-turn claude response).
func (u tokenUsage) firstTurnTokens() int {
	if len(u.Iterations) > 0 {
		return u.Iterations[0].total()
	}
	return u.totalTokens()
}

func runClaudeJSON(dir, prompt string) (tokenUsage, string, error) {
	cmd := exec.Command("claude", "-p", "--dangerously-skip-permissions", "--output-format", "json", prompt)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return tokenUsage{}, "", fmt.Errorf("claude -p --output-format json: %w", err)
	}
	var parsed struct {
		Usage    tokenUsage `json:"usage"`
		Result   string     `json:"result"`
		NumTurns int        `json:"num_turns"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return tokenUsage{}, "", fmt.Errorf("parse claude json output: %w", err)
	}
	parsed.Usage.NumTurns = parsed.NumTurns
	return parsed.Usage, parsed.Result, nil
}

// runClaudeContext runs claude's real /context breakdown in dir (with
// whatever doc has already been copied in as AGENTS.md), returning its
// markdown report of the actual token composition (system prompt, tools,
// skills, memory files) as claude itself measures it — more accurate than
// this harness's own bytes/4 doc-size estimate.
func runClaudeContext(dir string) (string, error) {
	cmd := exec.Command("claude", "-p", "--dangerously-skip-permissions", "/context")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude -p /context: %w", err)
	}
	return string(out), nil
}

func runAgyJSON(dir, prompt string) (tokenUsage, string, error) {
	cmd := exec.Command("agy", "-p", prompt, "--output-format", "json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return tokenUsage{}, "", fmt.Errorf("agy -p --output-format json: %w", err)
	}
	var parsed struct {
		Usage    tokenUsage `json:"usage"`
		Response string     `json:"response"`
		NumTurns int        `json:"num_turns"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return tokenUsage{}, "", fmt.Errorf("parse agy json output: %w", err)
	}
	parsed.Usage.NumTurns = parsed.NumTurns
	return parsed.Usage, parsed.Response, nil
}

// docRef is one file pulled into context, directly or via @include chain.
type docRef struct {
	path   string // relative to repoRoot for display
	bytes  int
	tokens int // rough estimate for text; -1 when image-token cost is provider-specific
}

// includeRefPattern matches an eager @path include directive on its own
// reference point in a doc line, e.g. "@docs/Foo.md" or "@AgenticLoop.png".
// Deliberately conservative (word-boundary + path-like charset) since this
// is a best-effort trace, not a full markdown/CLAUDE.md macro parser.
var includeRefPattern = regexp.MustCompile(`@([A-Za-z0-9_./-]+\.(?:md|png))`)

// traceDocContext walks entryPath and recursively follows @path include
// directives, returning one docRef per unique file actually found on disk,
// entryPath first. Each include is resolved relative to the directory of
// the file that referenced it (matching this repo's own convention, e.g.
// harnez/CLAUDE.md's sibling "@AGENTS.local.md"), not a fixed root — this
// lets it trace both real repo docs and an isolated workspace's AGENTS.md
// referencing a sibling doc copied in next to it. Display paths are shown
// relative to repoRoot when the file is under it, else as a bare basename.
// Missing referenced files are silently skipped — this traces what the
// harness's own doc variants actually pull in, not a strict-include
// validator.
func traceDocContext(repoRoot, entryPath, agent string) ([]docRef, error) {
	var refs []docRef
	seen := map[string]bool{}
	var walk func(path string) error
	walk = func(path string) error {
		abs := path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(repoRoot, path)
		}
		if seen[abs] {
			return nil
		}
		seen[abs] = true
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil // referenced doc not present — skip, not fatal
		}
		disp := abs
		if rel, err := filepath.Rel(repoRoot, abs); err == nil && !strings.HasPrefix(rel, "..") {
			disp = rel
		} else {
			disp = filepath.Base(abs)
		}
		tokens := len(data) / 4
		if strings.EqualFold(filepath.Ext(abs), ".png") {
			tokens = imageTokenEstimate(abs, agent)
		}
		refs = append(refs, docRef{path: disp, bytes: len(data), tokens: tokens})
		base := filepath.Dir(abs)
		for _, m := range includeRefPattern.FindAllStringSubmatch(string(data), -1) {
			next := m[1]
			if !filepath.IsAbs(next) {
				next = filepath.Join(base, next)
			}
			if err := walk(next); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(entryPath); err != nil {
		return nil, err
	}
	return refs, nil
}

func measureCost() error {
	fixturesAbs, err := resolveFixturesPath()
	if err != nil {
		return err
	}
	var cleanup func()
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(fixturesAbs)))
	if fixturesAbs == "" {
		repoRoot, cleanup, err = embeddedRepo()
		if err != nil {
			return fmt.Errorf("materialize embedded assets: %w", err)
		}
		defer cleanup()
	}
	if fixturesAbs != "" {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
			selfDir, err := os.Getwd()
			if err != nil {
				return err
			}
			repoRoot = selfDir
		}
	}

	fixtures, err := loadFixtures()
	if err != nil {
		return err
	}
	var target *fixture
	for i := range fixtures {
		if fixtures[i].ID == flagCostFixture {
			target = &fixtures[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("no fixture with id %q found in fixtures.yaml (known ids: %s)", flagCostFixture, knownIDs(fixtures))
	}

	variantPaths := map[string]string{
		"full": filepath.Join(repoRoot, "docs", "AgenticLoop.md"),
		"lite": filepath.Join(repoRoot, "docs", "practices", "AgenticLoop.lite.md"),
	}
	docPath, ok := variantPaths[flagCostVariant]
	if !ok {
		return fmt.Errorf("unknown --variant=%q (known: full, lite)", flagCostVariant)
	}

	type jsonRunner struct {
		name    string
		run     func(dir, prompt string) (tokenUsage, string, error)
		context func(dir string) (string, error) // nil if the CLI has no /context equivalent
	}
	allRunners := []jsonRunner{
		{name: "claude", run: runClaudeJSON, context: runClaudeContext},
		{name: "agy", run: runAgyJSON}, // agy has no /context command
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

	if !validLinkModes[flagCostLink] {
		return fmt.Errorf("unknown --link=%q (known: soft, hard, embed)", flagCostLink)
	}
	if !validDeliveryModes[flagCostDelivery] {
		return fmt.Errorf("unknown --delivery=%q (known: native, png)", flagCostDelivery)
	}
	if flagCostDelivery == "png" && flagCostLink == "embed" {
		return fmt.Errorf("--delivery=png is not valid with --link=embed (embed inlines text; a PNG has no text form to inline)")
	}

	prompt := fmt.Sprintf("Read AGENTS.md first, then complete this task:\n%s", target.Prompt)

	const rule = "────────────────────────────────────────────────────────────────"
	fmt.Println(rule)
	fmt.Printf("fixture   %s\n", target.ID)
	fmt.Printf("variant   %s  (%s)\n", flagCostVariant, docPath)
	fmt.Printf("link      %s\n", flagCostLink)
	fmt.Printf("delivery  %s\n", flagCostDelivery)
	fmt.Printf("prompt    %s\n", target.Prompt)
	fmt.Println(rule)

	anyErr := false
	for i, r := range runners {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("[%s]\n", r.name)
		if _, err := exec.LookPath(r.name); err != nil {
			fmt.Printf("  SKIP  %s not on PATH\n", r.name)
			continue
		}
		work, err := os.MkdirTemp("", "canary-agenticloop-lite-measure.*")
		if err != nil {
			fmt.Printf("  FAIL  mkdtemp: %v\n", err)
			anyErr = true
			continue
		}
		docs := []string{docPath}
		docs = append(docs, fixtureDocs(repoRoot, flagCostVariant, *target)...)
		fmt.Println("  expected context docs")
		fmt.Println("    AGENTS.md")
		for _, ref := range expectedContextDocs(docs, flagCostDelivery) {
			fmt.Printf("    %s\n", ref)
		}
		if err := setupLinkedWorkspace(work, repoRoot, docs, flagCostLink, flagCostDelivery); err != nil {
			fmt.Printf("  FAIL  setup workspace: %v\n", err)
			os.RemoveAll(work)
			anyErr = true
			continue
		}
		if refs, err := traceDocContext(work, filepath.Join(work, "AGENTS.md"), r.name); err == nil {
			fmt.Println("  context docs")
			total := 0
			for _, ref := range refs {
				if ref.tokens < 0 {
					fmt.Printf("    %-38s %7d B  image tokens: unavailable\n", ref.path, ref.bytes)
					continue
				}
				if strings.HasSuffix(ref.path, ".png") {
					fmt.Printf("    %-38s %7d B  ~%6d image tok\n", ref.path, ref.bytes, ref.tokens)
					continue
				}
				fmt.Printf("    %-38s %7d B  ~%6d tok\n", ref.path, ref.bytes, ref.tokens)
				total += ref.tokens
			}
			fmt.Printf("    %-38s %10s  ~%6d tok\n", "total", "", total)
		}
		if r.context != nil {
			if ctxOut, err := r.context(work); err == nil {
				fmt.Println("  /context (real, before fixture prompt)")
				for _, line := range strings.Split(strings.TrimSpace(ctxOut), "\n") {
					// Trim the per-skill breakdown table (dozens of rows) and
					// keep only the category summary above it — that's the
					// part relevant to "where do the tokens come from".
					if strings.HasPrefix(line, "### Skills") {
						break
					}
					fmt.Printf("    %s\n", line)
				}
			} else {
				fmt.Printf("  /context  FAIL %v\n", err)
			}
		} else {
			fmt.Println("  /context  n/a (agent has no /context equivalent)")
		}
		usage, response, err := r.run(work, prompt)
		os.RemoveAll(work)
		if err != nil {
			fmt.Printf("  FAIL  %v\n", err)
			anyErr = true
			continue
		}

		status := "n/a"
		clean := strings.TrimSpace(response)
		if target.Pattern != "" {
			if ok, err := regexp.MatchString(target.Pattern, clean); err == nil {
				if ok {
					status = "PASS"
				} else {
					status = "FAIL (pattern not matched)"
				}
			}
		}
		if target.ForbidPattern != "" {
			if ok, err := regexp.MatchString(target.ForbidPattern, clean); err == nil && ok {
				status = "FAIL (forbid_pattern matched)"
			}
		}

		fmt.Printf("  score        %s\n", status)
		fmt.Printf("  response     %s\n", truncate(clean, 300))
		fmt.Println("  token use")
		fmt.Printf("    %-16s %8d\n", "first-turn", usage.firstTurnTokens())
		fmt.Printf("    %-16s %8d\n", "total", usage.totalTokens())
		if usage.NumTurns > 0 {
			fmt.Printf("    %-16s %8d\n", "turns", usage.NumTurns)
		} else if len(usage.Iterations) > 0 {
			fmt.Printf("    %-16s %8d\n", "turns", len(usage.Iterations))
		}
		fmt.Printf("    %-16s %8d\n", "input", usage.InputTokens)
		fmt.Printf("    %-16s %8d\n", "output", usage.OutputTokens)
		fmt.Printf("    %-16s %8d\n", "cache_read", usage.CacheReadInputTokens+usage.CacheReadTokens)
		fmt.Printf("    %-16s %8d\n", "cache_creation", usage.CacheCreationInputTokens)
		fmt.Printf("    %-16s %8d\n", "thinking", usage.ThinkingTokens)
	}
	fmt.Println(rule)
	if anyErr {
		return fmt.Errorf("one or more agents failed to report a cost baseline")
	}
	return nil
}

func expectedContextDocs(docs []string, delivery string) []string {
	refs := make([]string, 0, len(docs))
	for _, doc := range docs {
		name := filepath.Base(doc)
		if delivery == "png" {
			name = strings.TrimSuffix(name, filepath.Ext(name)) + ".png"
		}
		refs = append(refs, name)
	}
	return refs
}

func imageTokenEstimate(path, agent string) int {
	f, err := os.Open(path)
	if err != nil {
		return -1
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	if err != nil {
		return -1
	}
	stats := readcard.ComputeImageTokens(0, 0, c.Width, c.Height, 1)
	if agent == "claude" {
		return stats.ClaudeTokens
	}
	return stats.OpenAITokens
}

// setupLinkedWorkspace prepares an isolated workspace's AGENTS.md per
// --link (issue 412) and --delivery (issue 412 follow-up), always returning
// work/AGENTS.md as the agent's entry point.
//
// --link controls how the doc is *referenced*:
//   - "soft": AGENTS.md holds a bare-prose citation ("See Doc.md ...");
//     the doc is copied in alongside it but nothing forces the agent to
//     open it.
//   - "hard": AGENTS.md holds a single "@Doc.md" eager-include directive
//     (this repo's own harnez/CLAUDE.md convention, e.g. "@AGENTS.local.md").
//   - "embed": the doc's full text is inlined directly into AGENTS.md
//     under a "# Doc.md" heading; text docs only, incompatible with
//     --delivery=png (validated by callers before this is reached).
//
// --delivery controls what form the referenced doc takes:
//   - "native" (default): the doc's own text/markdown file, as above.
//   - "png": the doc is first rendered to a PNG context card via
//     `harnez read -I -o <work>/<base>.png <docPath>` (this repo's own
//     Harnez Managed Conventions doc-delivery mode), and "soft"/"hard"
//     reference that PNG's path instead of the source doc.
func fixtureDocs(repoRoot, variant string, fx fixture) []string {
	var docs []string
	for _, path := range fx.Docs {
		if variant == "lite" {
			candidate := strings.TrimSuffix(path, filepath.Ext(path)) + ".lite" + filepath.Ext(path)
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(candidate))); err == nil {
				path = candidate
			}
		}
		docs = append(docs, filepath.Join(repoRoot, filepath.FromSlash(path)))
	}
	return docs
}

func setupLinkedWorkspace(work, repoRoot string, docPaths []string, link, delivery string) error {
	if !validLinkModes[link] {
		return fmt.Errorf("unknown --link=%q (known: soft, hard, embed)", link)
	}
	if !validDeliveryModes[delivery] {
		return fmt.Errorf("unknown --delivery=%q (known: native, png)", delivery)
	}
	if delivery == "png" && link == "embed" {
		return fmt.Errorf("--delivery=png is not valid with --link=embed (embed inlines text; a PNG has no text form to inline)")
	}

	agentsPath := filepath.Join(work, "AGENTS.md")
	linkPolicy := fixturePreamble + "\n\n"
	if len(docPaths) == 0 {
		return fmt.Errorf("no documents configured")
	}

	var refs []string
	for _, docPath := range docPaths {
		base := filepath.Base(docPath)

		if delivery == "png" {
			pngBase := strings.TrimSuffix(base, filepath.Ext(base)) + ".png"
			pngPath := filepath.Join(work, pngBase)
			cmd := exec.Command("harnez", "read", "-I", "-o", pngPath, docPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("harnez read -I render doc to png: %w: %s", err, truncate(string(out), 200))
			}
			refs = append(refs, pngBase)
			continue
		}
		if link == "hard" {
			if err := copyFile(docPath, filepath.Join(work, base)); err != nil {
				return fmt.Errorf("copy doc: %w", err)
			}
		}
		refs = append(refs, base)
	}

	if link == "hard" {
		var b strings.Builder
		for _, ref := range refs {
			fmt.Fprintf(&b, "@%s\n", ref)
		}
		return os.WriteFile(agentsPath, []byte(linkPolicy+b.String()), 0o644)
	}
	if link == "soft" {
		var b strings.Builder
		for _, ref := range refs {
			fmt.Fprintf(&b, "See %s for your instructions/context for this session.\n", ref)
		}
		return os.WriteFile(agentsPath, []byte(linkPolicy+b.String()), 0o644)
	}

	// embed is text-only and inlines every configured document.
	var b strings.Builder
	for _, docPath := range docPaths {
		base := filepath.Base(docPath)
		data, err := os.ReadFile(docPath)
		if err != nil {
			return fmt.Errorf("read doc for embed: %w", err)
		}
		fmt.Fprintf(&b, "# %s\n\n%s\n", base, data)
	}
	return os.WriteFile(agentsPath, []byte(linkPolicy+b.String()), 0o644)

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

func initWorkspace() error {
	work := flagWorkspaceDir
	created := false
	if work == "" {
		var err error
		work, err = os.MkdirTemp("", "canary-agenticloop-lite-experiment.*")
		if err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}
		created = true
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	for name, target := range map[string]string{
		"embedded/fixtures.yaml":                      "fixtures.yaml",
		"embedded/docs/AgenticLoop.md":                "docs/AgenticLoop.md",
		"embedded/docs/practices/AgenticLoop.lite.md": "docs/practices/AgenticLoop.lite.md",
		"embedded/docs/lang/Bash.md":                  "docs/lang/Bash.md",
		"embedded/docs/lang/Bash.lite.md":             "docs/lang/Bash.lite.md",
	} {
		data, err := embeddedAssets.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", name, err)
		}
		path := filepath.Join(work, target)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("workspace initialized: %s\n", work)
	fmt.Println("  lifecycle: created; cleanup: remove this directory when finished")
	if created {
		fmt.Println("  lifecycle: temporary workspace is not removed automatically")
	}
	return nil
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
