package subagent

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// julesSessionIDPattern accepts the IDs Jules issues, and nothing that could
// change the request path.
var julesSessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// JulesSessionReader reads a Jules session by ID without changing it.
type JulesSessionReader interface {
	InspectJulesSession(ctx context.Context, id string) (JulesSessionStatus, error)
}

// JulesSessionStatus is the read-only view of one Jules session.
type JulesSessionStatus struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	URL         string `json:"url,omitempty"`
	PullRequest string `json:"pull_request,omitempty"`
	// Reason is Jules's failure reason for a FAILED session.
	Reason string `json:"reason,omitempty"`
	// LastMessage is the newest message Jules wrote: its question while it
	// waits, or its summary once done.
	LastMessage string `json:"last_message,omitempty"`
	// Warning says why the activities could not be read.
	Warning string `json:"warning,omitempty"`
}

// ValidateJulesSessionID refuses an ID that cannot be a Jules session ID, before
// any request or write.
func ValidateJulesSessionID(id string) error {
	if !julesSessionIDPattern.MatchString(id) {
		return fmt.Errorf("Jules session ID %q must contain only letters, digits, '-' and '_'", id)
	}
	return nil
}

// InspectJulesSession reads a session once. It never polls, sends a message or
// writes a record. The API key stays in the request header and never reaches an
// error.
func (d JulesDriver) InspectJulesSession(ctx context.Context, id string) (JulesSessionStatus, error) {
	if err := ValidateJulesSessionID(id); err != nil {
		return JulesSessionStatus{}, err
	}
	client, _, _, err := d.client()
	if err != nil {
		return JulesSessionStatus{}, err
	}
	var session julesSession
	if err := client.call(ctx, http.MethodGet, "sessions/"+url.PathEscape(id), nil, &session); err != nil {
		return JulesSessionStatus{}, fmt.Errorf("read Jules session %s: %w", id, err)
	}
	status := JulesSessionStatus{ID: id, State: session.State, URL: session.URL}
	if pr := session.pullRequest(); pr != nil {
		status.PullRequest = pr.URL
	}
	name := "sessions/" + url.PathEscape(id)
	if last, err := d.lastAgentMessage(ctx, client, name); err != nil {
		status.Warning = "could not read Jules activities: " + err.Error()
	} else {
		status.LastMessage = last
	}
	if session.State == "FAILED" && status.Warning == "" {
		if reason, err := d.failureReason(ctx, client, name); err == nil {
			status.Reason = reason
		}
	}
	return status, nil
}

// Summary formats a status for people, with the next step for a session that
// waits for its owner.
func (s JulesSessionStatus) Summary() string {
	state := s.State
	if state == "" {
		state = "state unknown"
	}
	lines := []string{fmt.Sprintf("Jules session %s: %s", s.ID, state)}
	if s.URL != "" {
		lines = append(lines, "Session: "+s.URL)
	}
	if s.PullRequest != "" {
		lines = append(lines, "Pull request: "+s.PullRequest)
	}
	if s.Reason != "" {
		lines = append(lines, "Reason: "+s.Reason)
	}
	if s.LastMessage != "" {
		lines = append(lines, "Jules says: "+s.LastMessage)
	}
	if s.Warning != "" {
		lines = append(lines, "Warning: "+s.Warning)
	}
	if s.State == "AWAITING_USER_FEEDBACK" {
		lines = append(lines, fmt.Sprintf(`Next: answer with harnez agent resume --model jules --id %s "<message>".`, s.ID))
	}
	return strings.Join(lines, "\n")
}

// AdoptJulesSession returns the local record for a Jules session, creating it on
// first use. A repeat adopt reuses the record. The ID is checked with one GET
// before anything is written, so an unknown ID leaves the store untouched.
func AdoptJulesSession(ctx context.Context, store *FileSessionStore, reader JulesSessionReader, id, name, workDir string) (*Session, error) {
	if err := ValidateJulesSessionID(id); err != nil {
		return nil, err
	}
	sessions, err := store.List("", true)
	if err != nil {
		return nil, err
	}
	for _, s := range sessions {
		if s.Provider != "jules" || s.ProviderSessionID != id {
			continue
		}
		if name != "" && name != s.Name {
			return nil, fmt.Errorf("Jules session %s is already adopted as %q; drop --name or use --name %s", id, s.Name, s.Name)
		}
		return s, nil
	}
	if name == "" {
		suffix := id
		if len(suffix) > 6 {
			suffix = suffix[len(suffix)-6:]
		}
		name = "jules-" + suffix
	}
	for _, s := range sessions {
		if s.Name == name || s.ID == name {
			return nil, fmt.Errorf("session name %q is already in use by %s: %w", name, s.ID, ErrSessionNameInUse)
		}
	}
	if _, err := reader.InspectJulesSession(ctx, id); err != nil {
		return nil, err
	}
	model, err := ResolveModel("jules")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sess := &Session{
		ID: uuid.NewString(), ProviderSessionID: id, Name: name,
		Provider: model.Provider, Model: model.Name, Tier: model.Tier,
		WorkingDir: workDir, HarnessType: "harnez", Status: "completed",
		CreatedAt: now, LastActiveAt: now,
	}
	if err := store.Create(sess); err != nil {
		return nil, err
	}
	return sess, nil
}
