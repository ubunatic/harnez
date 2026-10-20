package subagent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/google/uuid"
)

// QueuedMessage is a durable message waiting for an explicit batch resume.
type QueuedMessage struct {
	ID     string `json:"id"`
	Sender string `json:"sender"`
	Text   string `json:"text"`
}

func (s *FileSessionStore) lockRegistry() (func(), error) {
	lock, err := os.OpenFile(filepath.Join(s.dir, ".registry.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open session registry lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("lock session registry: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
	}, nil
}

// SaveQueuedTurn records a successful turn and acknowledges exactly its snapshot
// of queued messages in one write. Sends after the snapshot remain queued.
func (s *FileSessionStore) SaveQueuedTurn(sess *Session, consumed []QueuedMessage) error {
	return s.saveQueued(sess, consumed, true)
}

func (s *FileSessionStore) saveQueued(sess *Session, consumed []QueuedMessage, requireExisting bool) error {
	unlock, err := s.lockRegistry()
	if err != nil {
		return err
	}
	defer unlock()
	_, err = os.Stat(filepath.Join(s.dir, sess.ID+".json"))
	if err != nil && (!os.IsNotExist(err) || requireExisting) {
		return err
	}
	if err == nil {
		current, err := s.Get(sess.ID)
		if err != nil {
			return err
		}
		acked := make(map[string]bool, len(consumed))
		for _, message := range consumed {
			acked[message.ID] = true
		}
		// Ordinary writers carry stale copies of the queue. Only the on-disk
		// queue is authoritative; they must neither drop nor resurrect messages.
		sess.PendingMessages = nil
		for _, message := range current.PendingMessages {
			if !acked[message.ID] {
				sess.PendingMessages = append(sess.PendingMessages, message)
			}
		}
	}
	return s.save(sess)
}

// QueueMessage resolves an exact name, validates the exec
// session, and appends under the same lock used by registry writers.
func (s *FileSessionStore) QueueMessage(name, dir, sender, text string) (*Session, error) {
	unlock, err := s.lockRegistry()
	if err != nil {
		return nil, err
	}
	defer unlock()
	sessions, err := s.List("", true)
	if err != nil {
		return nil, err
	}
	var matches []*Session
	var candidates []string
	for _, sess := range sessions {
		if dir != "" {
			want, _ := filepath.Abs(dir)
			have, _ := filepath.Abs(sess.WorkingDir)
			if want != have {
				continue
			}
		}
		if sess.Name == name {
			matches = append(matches, sess)
			candidates = append(candidates, fmt.Sprintf("%q id=%s provider=%s dir=%q", sess.Name, sess.ID, sess.Provider, sess.WorkingDir))
		}
	}
	if len(matches) == 0 {
		deleted, err := s.ListDeleted()
		if err != nil {
			return nil, err
		}
		for _, sess := range deleted {
			if sess.Name == name {
				return nil, fmt.Errorf("session %q was deleted", name)
			}
		}
		return nil, fmt.Errorf("unknown registry exec session %q", name)
	}
	if len(matches) > 1 {
		sort.Strings(candidates)
		return nil, fmt.Errorf("ambiguous registry exec session %q; candidates:\n  %s", name, strings.Join(candidates, "\n  "))
	}
	sess := matches[0]
	if sess.Status != "running" && sess.Status != "failed" && sess.Status != "stopped" && sess.Status != "completed" && sess.Status != "active" && sess.Status != "idle" {
		return nil, fmt.Errorf("session %q cannot queue messages: status %s", name, sess.Status)
	}
	if sess.ResumeBlockedReason != "" {
		return nil, fmt.Errorf("session %q cannot be resumed: %s", name, sess.ResumeBlockedReason)
	}
	if sess.Provider == "codex" && sess.CodexQuarantine != nil {
		return nil, fmt.Errorf("session %q is quarantined: %s", name, sess.CodexQuarantine.Reason)
	}
	sess.PendingMessages = append(sess.PendingMessages, QueuedMessage{ID: uuid.NewString(), Sender: sender, Text: text})
	if err := s.save(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// PrependQueuedMessages preserves FIFO order and labels each sender.
func PrependQueuedMessages(messages []QueuedMessage, prompt string) string {
	var b strings.Builder
	for _, message := range messages {
		b.WriteString(message.Sender)
		b.WriteString("\n\n")
		b.WriteString(message.Text)
		b.WriteString("\n\n")
	}
	b.WriteString(prompt)
	return b.String()
}
