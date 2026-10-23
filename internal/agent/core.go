// Package agent contains Cobra-free agent policy and protocol logic.
package agent

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

const (
	StreamFull  = "full"
	StreamStats = "stats"
)

func CheckStreamMode(mode string) error {
	if mode != StreamFull && mode != StreamStats {
		return fmt.Errorf("invalid --stream %q: want %q or %q", mode, StreamFull, StreamStats)
	}
	return nil
}

func WithProtocol(prompt string, planFirst bool, role string) string {
	pre := protocolPreamble
	if rules, err := subagent.RoleRules(role); err == nil && rules != "" {
		pre += "Harnez role: " + role + ". " + rules + "\n\n"
	}
	if planFirst {
		pre += planFirstPreamble
	}
	return pre + prompt
}

func WithResumeNote(prompt string, idle time.Duration) string {
	threshold, note, err := subagent.ResumeNote()
	if err != nil || idle <= threshold {
		return prompt
	}
	return note + "\n\n" + prompt
}

const protocolPreamble = "Harnez dispatch protocol: before any tool call, file read or other work, your very first message must be one line starting with `CONFIRM:` that states what you will do. " +
	"If your task asks you to plan first, send the plan as a message starting with `PLAN:`. Then continue with your task.\n\n"

const planFirstPreamble = "This is a plan-first turn: after your `CONFIRM:` message, inspect read-only if you must, send your plan as a message starting with `PLAN:`, and then end your turn. " +
	"Do not modify anything or start executing; the caller reviews the plan and resumes you with the go-ahead.\n\n"

func HeartbeatAt(i int, schedule []time.Duration, repeat time.Duration) time.Duration {
	if i < len(schedule) {
		return schedule[i]
	}
	return schedule[len(schedule)-1] + time.Duration(i-len(schedule)+1)*repeat
}

func FirstPromptSentence(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	if end := strings.IndexAny(prompt, "\r\n"); end >= 0 {
		prompt = strings.TrimSpace(prompt[:end])
	}
	if end := strings.IndexAny(prompt, ".?!"); end >= 0 {
		prompt = strings.TrimSpace(prompt[:end+1])
	}
	return prompt
}

func StartRole(requested, prompt, callerRole string, errOut io.Writer) (string, error) {
	role := requested
	if role != "" {
		resolved, classified, err := subagent.ResolveRoleDynamic(role)
		if err != nil {
			return "", err
		}
		if classified && errOut != nil {
			fmt.Fprintf(errOut, "harnez: resolved --role %q to %q\n", role, resolved)
		}
		role = resolved
	} else if sentence := FirstPromptSentence(prompt); sentence != "" {
		if inferred, _, err := subagent.ResolveRoleDynamic(sentence); err == nil {
			role = inferred
			if errOut != nil {
				fmt.Fprintf(errOut, "harnez: inferred role %q from prompt\n", role)
			}
		}
	}
	if role == "" {
		var err error
		if role, err = subagent.DefaultRole(); err != nil {
			return "", err
		}
	}
	if err := subagent.CheckSpawn(callerRole, role); err != nil {
		return "", err
	}
	return role, nil
}

func ResumeRole(requested string, sess *subagent.Session, callerRole string) (string, error) {
	role := sess.Role
	if role == "" {
		var err error
		if role, err = subagent.DefaultRole(); err != nil {
			return "", err
		}
	}
	if requested != "" {
		canonical, err := subagent.ResolveRole(requested)
		if err != nil {
			return "", err
		}
		if canonical != role {
			return "", fmt.Errorf("--role %q conflicts with session %q role %s", requested, sess.Name, role)
		}
	}
	if err := subagent.CheckSpawn(callerRole, role); err != nil {
		return "", err
	}
	return role, nil
}

func FindResumeSelector(sessions []*subagent.Session, selector string) (*subagent.Session, bool, error) {
	var exact *subagent.Session
	exactMatches := 0
	for _, sess := range sessions {
		if sess.ID == selector || sess.Name == selector || sess.ProviderSessionID == selector {
			exact = sess
			exactMatches++
		}
	}
	if exactMatches == 1 {
		return exact, true, nil
	}
	if exactMatches > 1 {
		return nil, false, fmt.Errorf("ambiguous agent session %q", selector)
	}
	var matches []*subagent.Session
	if len(selector) >= 8 {
		for _, sess := range sessions {
			if strings.HasPrefix(sess.ID, selector) || strings.HasPrefix(sess.ProviderSessionID, selector) {
				matches = append(matches, sess)
			}
		}
	}
	if len(matches) == 0 && len([]rune(selector)) >= 4 {
		for _, sess := range sessions {
			if strings.Contains(strings.ToLower(sess.Name), strings.ToLower(selector)) {
				matches = append(matches, sess)
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	return nil, false, nil
}

func QuotaAgeLabel(age time.Duration) string {
	if age < time.Minute {
		return age.Round(time.Second).String()
	}
	if age >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(age/(24*time.Hour)))
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm", int(age/time.Minute))
	}
	return fmt.Sprintf("%dh%dm", int(age/time.Hour), int(age/time.Minute)%60)
}

func RejectExhaustedQuota(spec string, model subagent.Model, override bool, availability func(string, string) usage.ProviderQuotaAvailability) error {
	if override {
		return nil
	}
	if availability == nil {
		availability = func(p, m string) usage.ProviderQuotaAvailability {
			return usage.CachedProviderQuotaAvailability(p, m, time.Now())
		}
	}
	quota := availability(model.Provider, model.Name)
	if quota.State != "exhausted" {
		return nil
	}
	message := fmt.Sprintf("agent start %q refused: provider %s quota is exhausted (snapshot %s old); pass --allow-exhausted-quota to override", spec, model.Provider, QuotaAgeLabel(quota.Age))
	if alternatives := QuotaAlternatives(model, availability); len(alternatives) > 0 {
		message += "; available cheaper alternatives: " + strings.Join(alternatives, ", ")
	}
	return fmt.Errorf("%s", message)
}

func QuotaAlternatives(selected subagent.Model, availability func(string, string) usage.ProviderQuotaAvailability) []string {
	if availability == nil {
		availability = func(p, m string) usage.ProviderQuotaAvailability {
			return usage.CachedProviderQuotaAvailability(p, m, time.Now())
		}
	}
	type candidate struct {
		spec string
		cost int
	}
	var candidates []candidate
	seen := map[string]bool{}
	for _, entry := range subagent.KnownModelEntries() {
		if entry.Model.Provider == selected.Provider || entry.Cost >= quotaModelCost(selected) || seen[entry.Model.Spec()] {
			continue
		}
		if availability(entry.Model.Provider, entry.Model.Name).State == "available" {
			seen[entry.Model.Spec()] = true
			candidates = append(candidates, candidate{spec: entry.Model.Spec(), cost: entry.Cost})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].cost == candidates[j].cost {
			return candidates[i].spec < candidates[j].spec
		}
		return candidates[i].cost < candidates[j].cost
	})
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	out := make([]string, len(candidates))
	for i := range candidates {
		out[i] = candidates[i].spec
	}
	return out
}

func quotaModelCost(model subagent.Model) int {
	for _, entry := range subagent.KnownModelEntries() {
		if entry.Model.Provider == model.Provider && entry.Model.Name == model.Name {
			return entry.Cost
		}
	}
	return 0
}
