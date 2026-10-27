package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

// agentCLI holds the `harnez agent` flag values and helpers shared by its
// subcommands; newAgentCmd builds one per command tree.
type agentCLI struct {
	agentTimeout        time.Duration
	all                 bool
	allowExhaustedQuota bool
	children            bool
	detach              bool
	failedOnly          bool
	find                func(cmd *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error)
	includeArchived     bool
	includeDead         bool
	jsonOut             bool
	listColorMode       string
	modelSpec           string
	name                string
	parent              func() string
	planSpec            string
	roleSpec            string
	rootPrompt          string
	statusAllReplies    bool
	statusReply         bool
	statusTurn          int
	store               func() (*subagent.FileSessionStore, error)
	storeDir            string
	streamMode          string
	workDir             string
	workerID            string
}

func (ac *agentCLI) startCmd() *cobra.Command {
	var startFiles []string
	var startPrompt string
	var startInteractive bool
	start := &cobra.Command{Use: "start [role] [name] [prompt...]", Short: "Start a new agent session", Long: "Start a new agent session. A leading role or alias may be positional; add a session name before the prompt when needed.\n\n" + sessionBackgroundHelp, Example: "  harnez agent start --name w --model luna -f task.md -- \"extra instructions\"\n  harnez agent start explorer \"audit SQLite queries\"\n  harnez agent start coder my-worker \"implement ticket 041\"\n  harnez agent start -i --model claude:haiku --name chat", Args: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		planFirst, err := parsePlanSpec(ac.planSpec)
		if err != nil {
			return err
		}
		words, tail := promptArgs(args, cmd.Flags().ArgsLenAtDash())
		startRoleSpec := ac.roleSpec
		startName := ac.name
		if len(words) > 0 {
			if positionalRole, roleErr := subagent.ResolveRole(words[0]); roleErr == nil {
				startRoleSpec = positionalRole
				words = words[1:]
				if startName == "" && (len(words) >= 2 || len(tail) > 0 || startPrompt != "" || len(startFiles) > 0) && len(words) > 0 {
					startName, words = words[0], words[1:]
				}
			}
		}
		if startPrompt != "" {
			words = append([]string{startPrompt}, words...)
		}
		if ac.rootPrompt != "" && strings.HasPrefix(ac.rootPrompt, "--") {
			return fmt.Errorf("-p needs prompt text but got flag %q; put -p last or use -- \"<text>\"", ac.rootPrompt)
		}
		if ac.modelSpec == "" && len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		prompt := ""
		if !startInteractive || len(startFiles) > 0 || len(words) > 0 || len(tail) > 0 {
			prompt, err = assemblePrompt(startFiles, words, tail, cmd.InOrStdin())
			if err != nil {
				return err
			}
		}
		if startInteractive {
			if err := validateInteractiveFlags(cmd, ac.detach, ac.jsonOut, ac.streamMode, ac.agentTimeout, ac.workerID != ""); err != nil {
				return err
			}
			return runInteractiveStart(cmd, interactiveDeps{store: ac.store, parent: ac.parent, storeDir: ac.storeDir}, interactiveStartRequest{Prompt: prompt, Name: startName, ModelSpec: ac.modelSpec, Dir: ac.workDir, Role: startRoleSpec, AllowExhaustedQuota: ac.allowExhaustedQuota})
		}
		req := startRequest{Role: startRoleSpec, Prompt: prompt, StoredPrompt: promptStorage(startFiles, words, tail, prompt), Name: startName, ModelSpec: ac.modelSpec, Dir: ac.workDir, StreamMode: ac.streamMode, JSON: ac.jsonOut, PlanFirst: planFirst, SessionID: ac.workerID, AllowExhaustedQuota: ac.allowExhaustedQuota}
		if ac.workerID != "" {
			return runDetachedWorker(cmd, req, ac.storeDir)
		}
		if ac.detach {
			return launchDetachedWithPreflight(cmd, req, ac.storeDir, ac.parent(), subagent.CheckCodexAuth)
		}
		return withAgentTimeout(cmd, ac.agentTimeout, func(ctx context.Context) error {
			return runStart(ctx, cmd, agentDeps{store: ac.store, parent: ac.parent, find: ac.find, storeDir: ac.storeDir, dbPath: agentUsageDBPath()}, req)
		})
	}}
	start.Flags().StringSliceVarP(&startFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	start.Flags().StringVarP(&startPrompt, "prompt", "p", "", "prompt text")
	start.Flags().BoolVarP(&startInteractive, "interactive", "i", false, "launch the provider's interactive terminal")
	start.Flags().BoolVar(&ac.jsonOut, "json", false, "JSON output")
	// --detach/--async stay for internal use but are hidden: hosts that used them lost
	// track of their workers (issue 630); they must use their session background instead.
	start.Flags().BoolVar(&ac.detach, "detach", false, "internal: run the agent in the machine background")
	start.Flags().BoolVar(&ac.detach, "async", false, "internal: alias for --detach")
	_ = start.Flags().MarkHidden("detach")
	_ = start.Flags().MarkHidden("async")
	start.Flags().BoolVar(&ac.allowExhaustedQuota, "allow-exhausted-quota", false, "start even when cached provider quota is exhausted")
	start.Flags().StringVar(&ac.workerID, "worker-session", "", "internal detached worker session ID")
	start.Flags().StringVar(&ac.planSpec, "plan", "no", "planning gate: yes or no")
	_ = start.RegisterFlagCompletionFunc("plan", flagValueCompletion("yes", "no"))
	_ = start.RegisterFlagCompletionFunc("stream", flagValueCompletion(streamFull, streamStats))
	start.Flags().StringVar(&ac.streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")
	withHostTrackingLine(start, ac.store, ac.parent, func() bool { return ac.workerID != "" })
	return start
}

func (ac *agentCLI) modelsCmd() *cobra.Command {
	var modelNamesOnly bool
	var picksOnly bool
	models := &cobra.Command{Use: "models", Short: "List known agent models with roles and when to use them", RunE: func(cmd *cobra.Command, _ []string) error {
		if picksOnly {
			if modelNamesOnly {
				return fmt.Errorf("--picks cannot be combined with --names")
			}
			picks, err := cachedModelPicks()
			if err != nil {
				return err
			}
			if ac.jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(picks)
			}
			return writeModelPicks(cmd.OutOrStdout(), picks)
		}
		defaultSpec, err := subagent.DefaultModelSpec()
		if err != nil {
			return err
		}
		if modelNamesOnly {
			specs, err := subagent.KnownModelSpecsE()
			if err != nil {
				return err
			}
			for _, spec := range specs {
				fmt.Fprintln(cmd.OutOrStdout(), spec)
			}
			return nil
		}
		knownEntries, err := subagent.KnownModelEntriesE()
		if err != nil {
			return err
		}
		entries := subagent.ModelEntriesWithDriver(knownEntries, func(m subagent.Model) subagent.Driver {
			return agentDriver(m, ac.workDir)
		})
		if ac.jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
		}
		availabilityByModel := modelProviderAvailabilities(entries, usage.CachedProviderQuotaAvailabilities)
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "SPEC\tMODEL\tSOURCE\tAVAILABILITY\tEFFORT\tCOST\tEFF\tSKILLS\tROLES\tUSE")
		for _, e := range entries {
			spec := e.Spec
			if spec == defaultSpec {
				spec += " (default)"
			}
			if !e.Batch {
				spec += " (interactive only)"
			}
			effort := "yes"
			if !e.Effort {
				effort = "no"
			}
			availability := availabilityByModel[e.Model.Provider+"\x00"+e.Model.Name]
			marker := availability.State
			if marker == "unknown" && availability.Age > 0 {
				marker = fmt.Sprintf("unknown (%s)", quotaAgeLabel(availability.Age))
			} else if marker == "exhausted" {
				marker = fmt.Sprintf("exhausted (resets %s)", quotaAgeLabel(availability.ResetIn))
			} else if marker == "stale" {
				marker = fmt.Sprintf("stale (%s)", quotaAgeLabel(availability.Age))
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n", spec, e.Model.Name, e.Source, marker, effort, e.Cost, e.Eff, e.Skills, e.Roles, e.Use)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		legend, err := subagent.ModelsLegend()
		if err != nil {
			return err
		}
		if legend != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "\n"+legend)
		}
		picks, err := cachedModelPicks()
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout())
		return writeModelPicks(cmd.OutOrStdout(), picks)
	}}
	models.Flags().BoolVar(&modelNamesOnly, "names", false, "print only the model specs, one per line")
	models.Flags().BoolVar(&picksOnly, "picks", false, "print role model picks")
	models.Flags().BoolVar(&ac.jsonOut, "json", false, "JSON output")
	return models
}

func cachedModelPicks() ([]subagent.ModelPick, error) {
	return subagent.ModelPicksWithAvailability(func(m subagent.Model) bool {
		return providerAvailability(m.Provider, m.Name).State == "exhausted"
	})
}

func writeModelPicks(out io.Writer, picks []subagent.ModelPick) error {
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ROLE\tTRAIT\tCHAIN\tRESOLVED NOW")
	for _, pick := range picks {
		chain := strings.Join(pick.Chain, ", ")
		if chain == "" {
			chain = "-"
		}
		resolved := pick.Resolved
		if pick.Error != "" {
			resolved = pick.Error
		}
		trait := pick.Trait
		if pick.Default {
			trait += "*"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", pick.Role, trait, chain, resolved)
	}
	return tw.Flush()
}

func modelProviderAvailabilities(entries []subagent.ModelEntry, lookup func(string, []string, time.Time) map[string]usage.ProviderQuotaAvailability) map[string]usage.ProviderQuotaAvailability {
	modelsByProvider := make(map[string]map[string]struct{})
	for _, entry := range entries {
		provider := entry.Model.Provider
		if modelsByProvider[provider] == nil {
			modelsByProvider[provider] = make(map[string]struct{})
		}
		modelsByProvider[provider][entry.Model.Name] = struct{}{}
	}
	providers := make([]string, 0, len(modelsByProvider))
	for provider := range modelsByProvider {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	availabilities := make(map[string]usage.ProviderQuotaAvailability)
	now := time.Now()
	for _, provider := range providers {
		models := make([]string, 0, len(modelsByProvider[provider]))
		for model := range modelsByProvider[provider] {
			models = append(models, model)
		}
		sort.Strings(models)
		for model, availability := range lookup(provider, models, now) {
			availabilities[provider+"\x00"+model] = availability
		}
	}
	return availabilities
}

func (ac *agentCLI) resumeCmd() *cobra.Command {
	var resumeFiles []string
	var resumeWorkerID string
	var continueResume bool
	var resumePrompt string
	var resumeInteractive bool
	resume := &cobra.Command{Use: "resume [session] [prompt...]", Short: "Resume an existing agent session", Long: "Resume an existing agent session.\n\n" + sessionBackgroundHelp, Example: "  harnez agent resume --name w \"next step\"\n  harnez agent resume <id> \"next step\"\n  harnez agent resume -i --name w", Args: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		planFirst, err := parsePlanSpec(ac.planSpec)
		if err != nil {
			return err
		}
		words, tail := promptArgs(args, cmd.Flags().ArgsLenAtDash())
		selector := ""
		if ac.name == "" && len(words) > 0 {
			if s, storeErr := ac.store(); storeErr != nil {
				return storeErr
			} else if matches, matchErr := matchResumeSelector(s, words[0]); matchErr != nil {
				return matchErr
			} else if matches {
				selector = words[0]
				words = words[1:]
			}
		}
		if resumePrompt != "" {
			words = append([]string{resumePrompt}, words...)
		}
		if continueResume && ac.name != "" {
			return fmt.Errorf("resume: --continue cannot be combined with --name")
		}
		if len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		prompt := ""
		if !resumeInteractive || len(resumeFiles) > 0 || len(words) > 0 || len(tail) > 0 {
			prompt, err = assemblePrompt(resumeFiles, words, tail, cmd.InOrStdin())
			if err != nil {
				return err
			}
		}
		if resumeInteractive {
			if err := validateInteractiveFlags(cmd, false, ac.jsonOut, ac.streamMode, ac.agentTimeout, resumeWorkerID != ""); err != nil {
				return err
			}
			return runInteractiveResume(cmd, interactiveDeps{store: ac.store, parent: ac.parent, find: ac.find, storeDir: ac.storeDir}, interactiveResumeRequest{Name: ac.name, Prompt: prompt, Dir: ac.workDir})
		}
		if resumeWorkerID != "" {
			return runDetachedResumeWorker(cmd, resumeWorkerID, resumeRequest{Role: ac.roleSpec, Prompt: prompt, Name: ac.name, ModelSpec: ac.modelSpec, Dir: ac.workDir, StreamMode: ac.streamMode, JSON: true, PlanFirst: planFirst}, ac.storeDir)
		}
		return withAgentTimeout(cmd, ac.agentTimeout, func(ctx context.Context) error {
			return runResume(ctx, cmd, agentDeps{store: ac.store, parent: ac.parent, find: ac.find, storeDir: ac.storeDir, dbPath: agentUsageDBPath()}, resumeRequest{Role: ac.roleSpec, Prompt: prompt, Name: ac.name, Selector: selector, ModelSpec: ac.modelSpec, Dir: ac.workDir, StreamMode: ac.streamMode, Continue: continueResume, JSON: ac.jsonOut, PlanFirst: planFirst})
		})
	}}
	resume.ValidArgsFunction = agentSessionCompletion(ac.storeDir, ac.parent)
	resume.Flags().BoolVar(&ac.jsonOut, "json", false, "JSON output")
	resume.Flags().StringVar(&resumeWorkerID, "worker-session", "", "internal detached worker session ID")
	resume.Flags().StringVar(&ac.planSpec, "plan", "no", "planning gate: yes or no")
	_ = resume.RegisterFlagCompletionFunc("plan", flagValueCompletion("yes", "no"))
	_ = resume.RegisterFlagCompletionFunc("stream", flagValueCompletion(streamFull, streamStats))
	resume.Flags().StringVar(&ac.streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")
	resume.Flags().StringSliceVarP(&resumeFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	resume.Flags().StringVarP(&resumePrompt, "prompt", "p", "", "prompt text")
	resume.Flags().BoolVarP(&resumeInteractive, "interactive", "i", false, "resume in the provider's interactive terminal")
	resume.Flags().BoolVarP(&continueResume, "continue", "c", false, "resume the most recently active attributable session")
	withHostTrackingLine(resume, ac.store, ac.parent, func() bool { return resumeWorkerID != "" })
	return resume
}
