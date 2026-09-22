package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
)

// This file holds the turn logic behind `agent start` and `agent resume`.
// runStart and runResume read only their request and deps, never cobra flag
// values, so every entry point (the verbs and the runnable `agent` root form)
// shares one implementation.

// agentDeps are the session-store and caller-identity hooks the runners need.
type agentDeps struct {
	store  func() (*subagent.FileSessionStore, error)
	parent func() string
	find   func(*cobra.Command, *subagent.FileSessionStore, string) (*subagent.Session, error)
}

// startRequest describes one new agent turn. Prompt is sent to the agent;
// StoredPrompt is what the session records (files as "path (N bytes)").
type startRequest struct {
	Prompt, StoredPrompt, Name, ModelSpec, Dir, StreamMode, Role string
	JSON, PlanFirst                                              bool
}

// resumeRequest describes one turn on an existing session: chosen by Name,
// else by attribution in Dir (Continue picks the most recent).
type resumeRequest struct {
	Role                                     string // must match the stored role when given
	Prompt, Name, ModelSpec, Dir, StreamMode string
	Continue, JSON, PlanFirst                bool
}

// resolveResumeSession picks the session and reports how: name, dir or continue.
func resolveResumeSession(cmd *cobra.Command, d agentDeps, s *subagent.FileSessionStore, req resumeRequest) (*subagent.Session, string, error) {
	if req.Name != "" {
		x, e := d.find(cmd, s, req.Name)
		return x, "name", e
	}
	xs, e := s.List("", true)
	if e != nil {
		return nil, "", e
	}
	c := attributable(xs, req.Dir, d.parent())
	if len(c) == 0 {
		return nil, "", fmt.Errorf("no resumable agent in %s; start one with: harnez agent start --name <name> ...", req.Dir)
	}
	if !req.Continue && len(c) != 1 {
		return nil, "", fmt.Errorf("multiple resumable agents in %s; pass --name (candidates: %s)", req.Dir, candidateSummary(c))
	}
	if req.Continue {
		return c[0], "continue", nil
	}
	return c[0], "dir", nil
}

// runStart starts a new session, streams or prints the turn and saves the session.
func runStart(cmd *cobra.Command, d agentDeps, req startRequest) error {
	spec := req.ModelSpec
	if spec == "" {
		var defaultErr error
		spec, defaultErr = subagent.DefaultModelSpec()
		if defaultErr != nil {
			return defaultErr
		}
	}
	if err := checkStreamMode(req.StreamMode); err != nil {
		return err
	}
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		return fmt.Errorf("agent start %q rejected: %w; ask for guidance rather than using a different model", spec, err)
	}
	role, err := startRole(req.Role)
	if err != nil {
		return err
	}
	s, err := d.store()
	if err != nil {
		return err
	}
	id := uuid.NewString()
	sessName := req.Name
	if sessName == "" {
		sessions, listErr := s.List("", true)
		if listErr != nil {
			return listErr
		}
		taken := make(map[string]bool, len(sessions)*2)
		for _, existing := range sessions {
			taken[existing.Name] = true
			taken[existing.ID] = true
		}
		sessName, err = subagent.GenerateSessionName(func(candidate string) bool { return taken[candidate] })
		if err != nil {
			return err
		}
	}
	if req.Name != "" {
		if existing, findErr := resolveSession(s, req.Name, ""); findErr == nil {
			return fmt.Errorf("session name %q is already in use by %s in %s", req.Name, existing.ID, existing.WorkingDir)
		}
	}
	canonicalWorkDir, err := filepath.Abs(req.Dir)
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	tl := newTimeline(cmd)
	opts := subagent.RunOptions{Prompt: req.Prompt, Model: m, Dir: canonicalWorkDir}
	parentID := d.parent() // read before the child's environment replaces it
	defer setAgentEnv(role, sessName)()
	driver := agentDriver(m)
	sd, streaming := driver.(subagent.StreamingDriver)
	streaming = streaming && !req.JSON
	var ts *turnStream
	var r *subagent.TurnResult
	if streaming {
		ts = newTurnStream(cmd, req.StreamMode, false)
		ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sessName, sessName, req.PlanFirst
		opts.Prompt = withProtocol(req.Prompt, req.PlanFirst, role)
		ts.watch()
		r, err = sd.RunStream(cmd.Context(), opts, func(ev subagent.Event) {
			if ev.Kind == "session" {
				extra := []string{fmt.Sprintf("name=%s dir=%s", sessName, canonicalWorkDir)}
				if req.ModelSpec == "" {
					extra = append(extra, "model: "+spec+" (default)")
				}
				ts.info(ev.Text, m.Provider+":"+m.Name, "start", "new", append(extra, "reconnect: harnez agent resume "+ev.Text+" \"<prompt>\"")...)
			}
			ts.onEvent(ev)
		})
		if err != nil {
			ts.abort()
		}
	} else {
		tl.announceTurn("start", m.Provider+":"+m.Name, sessName)
		r, err = driver.Run(cmd.Context(), opts)
	}
	if err != nil {
		return fmt.Errorf("agent start %q failed: %w; verify the provider/model configuration or ask for guidance", spec, err)
	}
	if r.SessionID != "" {
		id = r.SessionID
	}
	now := time.Now()
	sess := &subagent.Session{ID: id, Name: sessName, StartPrompt: req.StoredPrompt, Role: role, Provider: m.Provider, Model: m.Name, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parentID, CallerPID: os.Getpid(), HarnessType: "harnez", Status: "completed", TokensCumulative: r.TokensCumulative, TokensSinceCompact: subagent.CompactionTokens(r), TokensTurn: r.TokensTurn, CachedTokens: r.CachedTokens, CreatedAt: now, LastActiveAt: now}
	if err := s.Save(sess); err != nil {
		return err
	}
	out := agentOutput{Session: sess, Response: r.Response, Messages: r.Messages, ReconnectCmd: "harnez agent resume " + id + " \"<prompt>\""}
	if streaming {
		ts.finish(r)
		return nil
	}
	tl.finishTurn(r, id, out.ReconnectCmd)
	if req.JSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
	}
	return printAgentMessages(cmd, r.Messages, r.Response)
}

// runResume runs one more turn on an existing session.
func runResume(cmd *cobra.Command, d agentDeps, req resumeRequest) error {

	sessionName := req.Name
	resolved := "name"
	var sess *subagent.Session
	if req.Continue && sessionName != "" {
		return fmt.Errorf("resume: --continue cannot be combined with --name")
	}
	if err := checkStreamMode(req.StreamMode); err != nil {
		return err
	}
	s, err := d.store()
	if err != nil {
		return err
	}
	sess, resolved, err = resolveResumeSession(cmd, d, s, req)
	if err != nil {
		return err
	}

	if !subagent.CanManage(d.parent(), sess) {
		return fmt.Errorf("session %q is outside caller lineage", sess.ID)
	}
	if req.ModelSpec != "" {
		m, modelErr := subagent.ResolveModel(req.ModelSpec)
		if modelErr != nil || m.Provider != sess.Provider || m.Name != sess.Model || m.Tier != sess.Tier {
			return fmt.Errorf("--model %q conflicts with session %q model %s:%s:%s", req.ModelSpec, sess.Name, sess.Provider, sess.Model, sess.Tier)
		}
	}
	role, err := resumeRole(req.Role, sess)
	if err != nil {
		return err
	}
	defer setAgentEnv(role, sess.Name)()
	if sess.Status == "active" && sess.HarnessType == "interactive" {
		if err := subagent.SendControl(cmd.Context(), sess.ControlSocket, "prompt", req.Prompt); err != nil {
			return err
		}
		sess.LastActiveAt = time.Now()
		if err := s.Save(sess); err != nil {
			return err
		}
		return writeAgentOutput(cmd, req.JSON, agentOutput{Session: sess, Response: "prompt delivered"})
	}
	if sess.HarnessType == "interactive" && sess.ProviderSessionID == "" {
		return fmt.Errorf("session %q cannot be resumed: %s did not expose a provider session ID", sess.Name, sess.Provider)
	}
	driver := agentDriver(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier})
	if checker, ok := driver.(subagent.ResumeChecker); ok {
		if resumable, reason := checker.CheckResumable(sess.ProviderID()); !resumable {
			return fmt.Errorf("session %q cannot be resumed: %s; start a new session with: harnez agent start --name <new-name> ...", sess.Name, reason)
		}
	}
	tl := newTimeline(cmd)
	compacted, compactNote := false, ""
	if subagent.ShouldCompact(sess.TokensSinceCompact) {
		if _, err = driver.Compact(cmd.Context(), sess.ProviderID()); err != nil {
			return err
		}
		compactNote = fmt.Sprintf("queued /compact at %s new tokens since the last compaction; the agent acknowledges it before its reply", humanCount(sess.TokensSinceCompact))
		sess.TokensSinceCompact = 0
		compacted = true
	}
	var ts *turnStream
	var r *subagent.TurnResult
	sd, streaming := driver.(subagent.StreamingDriver)
	streaming = streaming && !req.JSON
	if streaming {
		ts = newTurnStream(cmd, req.StreamMode, compacted)
		ts.info(sess.ID, sess.Provider+":"+sess.Model, "resume", resolved)
		if compacted {
			ts.printf("[compact: %s]\n", compactNote)
		}
		ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sess.Name, sess.Name, req.PlanFirst
		ts.watch()
		r, err = sd.ResumeStream(cmd.Context(), sess.ProviderID(), withProtocol(req.Prompt, req.PlanFirst, role), subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}, ts.onEvent)
		if err != nil {
			ts.abort()
		}
	} else {
		if compacted {
			tl.log("compact", "%s", compactNote)
		}
		tl.announceTurn("resume", sess.Provider+":"+sess.Model, sess.Name)
		r, err = driver.Resume(cmd.Context(), sess.ProviderID(), req.Prompt, subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier})
	}
	if err != nil {
		recordResumeFailure(s, sess, err)
		return fmt.Errorf("agent resume %q (%s:%s:%s) failed: %w; verify the provider/model configuration or ask for guidance", sess.Name, sess.Provider, sess.Model, sess.Tier, err)
	}
	sess.LastError = ""
	sess.ResumeFailures = 0
	sess.TokensTurn = r.TokensTurn
	sess.TokensCumulative += r.TokensTurn
	sess.TokensSinceCompact += subagent.CompactionTokens(r)
	sess.CachedTokens = r.CachedTokens
	sess.LastActiveAt = time.Now()
	if err = s.Save(sess); err != nil {
		return err
	}
	if streaming {
		ts.finish(r)
		return nil
	}
	if compacted {
		var ack string
		if r.Messages, ack = subagent.SplitCompactionAck(r.Messages); ack != "" {
			tl.log("compact", "agent acknowledged: %s", firstLine(ack))
		}
	}
	tl.finishTurn(r, sess.ID, "harnez agent resume "+sess.ID+" \"<prompt>\"")
	return writeAgentOutput(cmd, req.JSON, agentOutput{Session: sess, Response: r.Response, Messages: r.Messages})
}

func writeAgentOutput(cmd *cobra.Command, jsonOut bool, v agentOutput) error {
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
	}
	return printAgentMessages(cmd, v.Messages, v.Response)
}

// knownSlashCommands are intercepted by the root form and never sent to the model.
var knownSlashCommands = map[string]bool{"/compact": true, "/stop": true, "/status": true}

func compactSession(cmd *cobra.Command, s *subagent.FileSessionStore, x *subagent.Session) error {
	if x.Status == "active" && x.HarnessType == "interactive" {
		if err := subagent.SendControl(cmd.Context(), x.ControlSocket, "compact", ""); err != nil {
			return err
		}
		x.LastActiveAt = time.Now()
		return s.Save(x)
	}
	if x.HarnessType == "interactive" && x.ProviderSessionID == "" {
		return fmt.Errorf("session %q cannot be compacted: %s did not expose a provider session ID", x.Name, x.Provider)
	}
	_, err := agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model, Tier: x.Tier}).Compact(cmd.Context(), x.ProviderID())
	return err
}

func stopSession(cmd *cobra.Command, s *subagent.FileSessionStore, x *subagent.Session) error {
	if x.HarnessType == "interactive" {
		if x.Status != "active" {
			return fmt.Errorf("session %q is not active", x.Name)
		}
		if err := subagent.SendControl(cmd.Context(), x.ControlSocket, "stop", ""); err != nil {
			return err
		}
		x.Status = "stopped"
		x.LastActiveAt = time.Now()
		return s.Save(x)
	}
	err := agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Stop(cmd.Context(), x.ProviderID())
	x.Status = "stopped"
	if err == nil {
		err = s.Save(x)
	}
	return err
}

func statusSession(cmd *cobra.Command, x *subagent.Session, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(x)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\nName: %s\nStatus: %s\nProvider: %s:%s\nTokens: %d (turn %d)\nCached: %d\n", x.ID, x.Name, x.Status, x.Provider, x.Model, x.TokensCumulative, x.TokensTurn, x.CachedTokens)
	if x.Role != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Role: %s\n", x.Role)
	}
	if x.LastError != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Last error: %s\nResume failures: %d\n", x.LastError, x.ResumeFailures)
	}
	return nil
}

const (
	agentRoleEnv    = "HARNEZ_AGENT_ROLE"
	agentSessionEnv = "HARNEZ_SESSION_ID"
)

// startRole picks the role of a new session and checks that the caller's own
// role (from the environment harnez gave it) may start it.
func startRole(requested string) (string, error) {
	role := requested
	if role == "" {
		var err error
		if role, err = subagent.DefaultRole(); err != nil {
			return "", err
		}
	}
	if err := subagent.CheckSpawn(os.Getenv(agentRoleEnv), role); err != nil {
		return "", err
	}
	return role, nil
}

// resumeRole returns the stored role of sess (default role for old records),
// rejects a conflicting --role and checks the caller may manage that role.
func resumeRole(requested string, sess *subagent.Session) (string, error) {
	role := sess.Role
	if role == "" {
		var err error
		if role, err = subagent.DefaultRole(); err != nil {
			return "", err
		}
	}
	if requested != "" && requested != role {
		return "", fmt.Errorf("--role %q conflicts with session %q role %s", requested, sess.Name, role)
	}
	if err := subagent.CheckSpawn(os.Getenv(agentRoleEnv), role); err != nil {
		return "", err
	}
	return role, nil
}

// setAgentEnv gives the provider process it is about to spawn its role and its
// session name (the parent id of anything it starts) and returns a restore func.
func setAgentEnv(role, name string) func() {
	prev := map[string]*string{}
	for _, kv := range [][2]string{{agentRoleEnv, role}, {agentSessionEnv, name}} {
		if v, ok := os.LookupEnv(kv[0]); ok {
			old := v
			prev[kv[0]] = &old
		} else {
			prev[kv[0]] = nil
		}
		_ = os.Setenv(kv[0], kv[1])
	}
	return func() {
		for k, v := range prev {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	}
}

// guardLeafRole refuses mutating agent verbs when the caller is a leaf worker.
// list, status and models stay available for diagnosis.
func guardLeafRole(verb string) error {
	role := os.Getenv(agentRoleEnv)
	if !subagent.IsLeafRole(role) {
		return nil
	}
	switch verb {
	case "list", "status", "models", "help", "agent":
		return nil // the root form checks each start/resume/slash itself
	}
	return subagent.CheckSpawn(role, "developer")
}
