package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/channel"
	"ubunatic.com/harnez/internal/subagent"
)

type channelIdentity struct {
	ID         string
	ProviderID string
	Name       string
}

// Claude's session metadata is undocumented and optional. Never let missing,
// malformed, oversized or future-version files prevent an explicit-ID send.
func channelIdentities(cfg channel.Config, storeDir string) []channelIdentity {
	var identities []channelIdentity
	if store, err := subagent.OpenSessionStore(storeDir); err == nil {
		if sessions, err := store.List("", true); err == nil {
			for _, session := range sessions {
				if session.Provider == "claude" {
					identities = append(identities, channelIdentity{session.ID, session.ProviderID(), session.Name})
				}
			}
		}
	}
	paths, _ := filepath.Glob(filepath.Join(cfg.ClaudeMetadataDir, cfg.ClaudeMetadataPattern))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > int64(cfg.MaxMetadataBytes) {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(file, int64(cfg.MaxMetadataBytes)+1))
		_ = file.Close()
		if err != nil || len(data) > cfg.MaxMetadataBytes {
			continue
		}
		var session struct {
			SessionID string `json:"sessionId"`
			Name      string `json:"name"`
		}
		if json.Unmarshal(data, &session) != nil || session.SessionID == "" || strings.TrimSpace(session.Name) == "" {
			continue
		}
		identities = append(identities, channelIdentity{session.SessionID, session.SessionID, session.Name})
	}
	return identities
}

// liveChannelNames lists the sorted names of live endpoints; unnamed sessions are left out.
func liveChannelNames(endpoints []channel.Endpoint, identities []channelIdentity, dir string) []string {
	wantDir, _ := filepath.Abs(dir)
	seen := map[string]bool{}
	var names []string
	for _, endpoint := range endpoints {
		if haveDir, _ := filepath.Abs(endpoint.WorkingDir); dir != "" && haveDir != wantDir {
			continue
		}
		for _, identity := range identities {
			if (identity.ID == endpoint.SessionID || identity.ProviderID == endpoint.SessionID) && identity.Name != "" && !seen[identity.Name] {
				seen[identity.Name] = true
				names = append(names, identity.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func resolveChannelTarget(endpoints []channel.Endpoint, identities []channelIdentity, name, id, dir string) (channel.Endpoint, error) {
	wantDir := ""
	if dir != "" {
		var err error
		wantDir, err = filepath.Abs(dir)
		if err != nil {
			return channel.Endpoint{}, err
		}
	}
	var matches []channel.Endpoint
	var candidates []string
	for _, endpoint := range endpoints {
		if wantDir != "" {
			haveDir, err := filepath.Abs(endpoint.WorkingDir)
			if err != nil || haveDir != wantDir {
				continue
			}
		}
		names := make(map[string]bool)
		matched := id != "" && id == endpoint.SessionID
		for _, identity := range identities {
			if identity.ProviderID != endpoint.SessionID {
				continue
			}
			if identity.Name != "" {
				names[identity.Name] = true
			}
			if name != "" && identity.Name == name {
				matched = true
			}
		}
		var labels []string
		for label := range names {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		candidates = append(candidates, fmt.Sprintf("%q id=%s dir=%q", strings.Join(labels, ", "), endpoint.SessionID, endpoint.WorkingDir))
		if matched {
			matches = append(matches, endpoint)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		candidates = append(candidates, "(no live registered channel sessions)")
	}
	reason := "unknown"
	if len(matches) > 1 {
		reason = "ambiguous"
	}
	return channel.Endpoint{}, fmt.Errorf("%s channel target %q; candidates:\n  %s", reason, name+id, strings.Join(candidates, "\n  "))
}

type agentSendTarget struct {
	Provider string
	Endpoint channel.Endpoint
}

func resolveAgentSendTarget(endpoints []channel.Endpoint, identities []channelIdentity, threads []channel.CodexThread, name, id, dir string) (agentSendTarget, error) {
	wantDir := ""
	if dir != "" {
		var err error
		wantDir, err = filepath.Abs(dir)
		if err != nil {
			return agentSendTarget{}, err
		}
	}
	inDir := func(candidate string) bool {
		if wantDir == "" {
			return true
		}
		haveDir, err := filepath.Abs(candidate)
		return err == nil && haveDir == wantDir
	}
	var matches []agentSendTarget
	var candidates []string
	for _, endpoint := range endpoints {
		if !inDir(endpoint.WorkingDir) {
			continue
		}
		var labels []string
		matched := id != "" && id == endpoint.SessionID
		for _, identity := range identities {
			if identity.ProviderID != endpoint.SessionID {
				continue
			}
			if identity.Name != "" {
				labels = append(labels, identity.Name)
			}
			if name != "" && identity.Name == name {
				matched = true
			}
		}
		sort.Strings(labels)
		candidates = append(candidates, fmt.Sprintf("%q id=%s provider=claude dir=%q", strings.Join(labels, ", "), endpoint.SessionID, endpoint.WorkingDir))
		if matched {
			matches = append(matches, agentSendTarget{Provider: "claude", Endpoint: endpoint})
		}
	}
	for _, thread := range threads {
		if !inDir(thread.CWD) {
			continue
		}
		matched := (id != "" && id == thread.ID) || (name != "" && name == thread.Name)
		candidates = append(candidates, fmt.Sprintf("%q id=%s provider=codex dir=%q", thread.Name, thread.ID, thread.CWD))
		if matched {
			matches = append(matches, agentSendTarget{Provider: "codex", Endpoint: channel.Endpoint{SessionID: thread.ID, WorkingDir: thread.CWD}})
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		candidates = append(candidates, "(no live Claude or Codex sessions)")
	}
	reason := "unknown"
	if len(matches) > 1 {
		reason = "ambiguous"
	}
	return agentSendTarget{}, fmt.Errorf("%s agent send target %q; live candidates:\n  %s", reason, name+id, strings.Join(candidates, "\n  "))
}

func liveAgentNames(endpoints []channel.Endpoint, identities []channelIdentity, threads []channel.CodexThread, dir string) []string {
	seen := make(map[string]bool)
	names := liveChannelNames(endpoints, identities, dir)
	for _, name := range names {
		seen[name] = true
	}
	wantDir, _ := filepath.Abs(dir)
	for _, thread := range threads {
		haveDir, _ := filepath.Abs(thread.CWD)
		if thread.Name != "" && (dir == "" || haveDir == wantDir) && !seen[thread.Name] {
			seen[thread.Name] = true
			names = append(names, thread.Name)
		}
	}
	sort.Strings(names)
	return names
}

func codexMessageFrame(cfg channel.Config, identities []channelIdentity, threads []channel.CodexThread, senderID, message string) string {
	for _, thread := range threads {
		if thread.ID == senderID && thread.Name != "" {
			identities = append(identities, channelIdentity{ID: senderID, ProviderID: senderID, Name: thread.Name})
			break
		}
	}
	frame := channelSender(cfg, identities, senderID) + "\n\n" + message
	if senderID == "" {
		return frame + "\n\nTo reply, run `harnez agent send --name <sender-session> \"<message>\"`."
	}
	return frame + "\n\nTo reply, run `harnez agent send --id " + senderID + " \"<message>\"`."
}

func queueCodexMessage(ctx context.Context, executable, threadID, message string) error {
	cmd := exec.CommandContext(ctx, executable, "queue", "--thread", threadID, "--message", message)
	var output bytes.Buffer
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if detail := strings.TrimSpace(output.String()); detail != "" {
			return fmt.Errorf("codex queue: %w: %s", err, detail)
		}
		return fmt.Errorf("codex queue: %w", err)
	}
	return nil
}

func sendCodexMessage(ctx context.Context, executable, threadID, message string, out io.Writer) error {
	if err := queueCodexMessage(ctx, executable, threadID, message); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(struct {
		Status    string `json:"status"`
		Provider  string `json:"provider"`
		SessionID string `json:"session_id"`
	}{Status: "queued", Provider: "codex", SessionID: threadID})
}

func channelSender(cfg channel.Config, identities []channelIdentity, id string) string {
	var names []string
	for _, identity := range identities {
		if (identity.ID == id || identity.ProviderID == id) && identity.Name != "" {
			names = append(names, identity.Name)
		}
	}
	sort.Strings(names)
	name := id
	if len(names) > 0 {
		name = names[0]
	}
	if id == "" {
		id, name = cfg.TerminalSender, cfg.TerminalSender
	}
	// Keep identity framing on one line even when a user chose a multiline name.
	name = strings.Join(strings.Fields(name), " ")
	id = strings.Join(strings.Fields(id), " ")
	return fmt.Sprintf(cfg.SenderFormat, name, id)
}

func newAgentSendCmd(storeDir, name, workDir *string) *cobra.Command {
	var message, prompt, id string
	var wait bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use: "send (--name <session> | --id <session-id>) <message>", Short: "Send to a Claude channel or Codex session; optionally wait for a Claude receipt",
		Long: "Send to a live Claude channel or Codex session by --name or --id. The message is the positional argument; -m/--message and -p/--prompt are accepted alternatives. Without a target, send lists the names of live sessions. Claude accepted means an MCP transport write; Codex queued means its local queue command accepted the message. Only Claude supports optional receipt confirmation, and it does not confirm task completion. Claude sessions are started manually by the user.",
		Example: `  harnez agent send --name harnez-53ih "look at ticket 748 in harnez"
  harnez agent send --id 86443124-b692-49e4-ae92-942b0177664f --wait "please confirm"`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			sources := 0
			for _, text := range []string{strings.Join(args, " "), message, prompt} {
				if strings.TrimSpace(text) != "" {
					sources++
					message = text
				}
			}
			if sources != 1 {
				return fmt.Errorf("send requires exactly one message: positional text (recommended), -m or -p")
			}
			if cmd.Flags().Changed("timeout") && (!wait || timeout <= 0) {
				return fmt.Errorf("send --timeout requires --wait and a positive duration")
			}
			cfg, err := channel.LoadConfig()
			if err != nil {
				return err
			}
			endpoints, err := channel.List(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			threads, _ := channel.DiscoverCodexThreads(cmd.Context(), "codex", time.Duration(cfg.IOTimeoutMS)*time.Millisecond, cfg.MaxRPCBytes)
			identities := channelIdentities(cfg, *storeDir)
			dir := ""
			if cmd.Flags().Changed("dir") {
				dir = *workDir
			}
			if *name == "" && id == "" {
				names := liveAgentNames(endpoints, identities, threads, dir)
				if len(names) == 0 {
					return fmt.Errorf("send requires --name <session> or --id <session-id>; no named live sessions (harnez channel list shows IDs)")
				}
				return fmt.Errorf("send requires --name <session> or --id <session-id>; live sessions:\n  %s\nuse: harnez agent send --name %s \"<message>\"", strings.Join(names, "\n  "), names[0])
			}
			if *name != "" && id != "" {
				return fmt.Errorf("send takes --name or --id, not both")
			}
			target, err := resolveAgentSendTarget(endpoints, identities, threads, *name, id, dir)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			if target.Provider == "codex" {
				content := codexMessageFrame(cfg, identities, threads, currentHostSession(), message)
				return sendCodexMessage(cmd.Context(), "codex", target.Endpoint.SessionID, content, cmd.OutOrStdout())
			}
			content := channelSender(cfg, identities, currentHostSession()) + "\n\n" + message
			accepted, err := channel.Send(cmd.Context(), cfg, target.Endpoint.SessionID, content)
			if err != nil {
				return err
			}
			if err := encoder.Encode(accepted); err != nil {
				return err
			}
			if !wait {
				return nil
			}
			if !cmd.Flags().Changed("timeout") {
				timeout = time.Duration(cfg.WaitTimeoutMS) * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			receipt, waitErr := channel.WaitReceipt(ctx, cfg, accepted)
			if err := encoder.Encode(receipt); err != nil {
				return err
			}
			if errors.Is(waitErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			return waitErr
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Claude or Codex session ID")
	cmd.Flags().StringVarP(&message, "message", "m", "", "message text (alternative to the positional message)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "message text (alternative to the positional message)")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for optional recipient acknowledgement")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "positive receipt wait timeout (default from spec/channel.yaml)")
	return cmd
}
