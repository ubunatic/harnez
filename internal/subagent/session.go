package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session represents an active subagent session with metadata and telemetry.
type Session struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	Tier            string    `json:"tier"`
	WorkingDir      string    `json:"working_dir"`
	ParentSessionID string    `json:"parent_session_id,omitempty"`
	CallerPID       int       `json:"caller_pid"`
	HarnessType     string    `json:"harness_type"`
	Status          string    `json:"status"`
	TokensCumulative int      `json:"tokens_cumulative"`
	TokensTurn      int       `json:"tokens_turn"`
	CachedTokens    int       `json:"cached_tokens"`
	CreatedAt       time.Time `json:"created_at"`
	LastActiveAt    time.Time `json:"last_active_at"`
}

// SessionStore manages persistent session metadata.
type SessionStore interface {
	Save(s *Session) error
	Get(id string) (*Session, error)
	List(parentID string, allSessions bool) ([]*Session, error)
	Delete(id string) error
}

// FileSessionStore persists sessions as JSON files in a directory.
type FileSessionStore struct {
	dir string
}

// NewSessionStore creates a new file-based session store at the given directory.
func NewSessionStore(dir string) (*FileSessionStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create session store directory: %w", err)
	}
	return &FileSessionStore{dir: dir}, nil
}

// DefaultStoreDir returns the default session store directory.
func DefaultStoreDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.harnez/agents"
	}
	return filepath.Join(home, ".harnez", "agents")
}

// Save persists a session to disk.
func (s *FileSessionStore) Save(sess *Session) error {
	if sess.ID == "" {
		return fmt.Errorf("session must have an ID")
	}
	path := filepath.Join(s.dir, sess.ID+".json")
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	return nil
}

// Get retrieves a session by ID.
func (s *FileSessionStore) Get(id string) (*Session, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %q not found", id)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	return &sess, nil
}

// List returns all sessions, optionally filtered by parentID.
func (s *FileSessionStore) List(parentID string, allSessions bool) ([]*Session, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, fmt.Errorf("failed to read session store directory: %w", err)
	}
	var sessions []*Session
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			id := entry.Name()[:len(entry.Name())-5]
			sess, err := s.Get(id)
			if err != nil {
				continue
			}
			if allSessions || parentID == "" || sess.ParentSessionID == parentID {
				sessions = append(sessions, sess)
			}
		}
	}
	return sessions, nil
}

// Delete removes a session file.
func (s *FileSessionStore) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session %q not found", id)
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}
	return nil
}

// CanManage checks if the caller (identified by callerParentID) can manage the target session.
// Returns true if:
// - callerParentID is empty (root/human caller)
// - target session is a direct/indirect child of callerParentID
func CanManage(callerParentID string, target *Session) bool {
	if callerParentID == "" {
		return true
	}
	if target.ID == callerParentID {
		return true
	}
	if target.ParentSessionID == callerParentID {
		return true
	}
	return false
}

// ShouldCompact returns true when cumulative tokens >= 100,000.
func ShouldCompact(tokensCumulative int) bool {
	return tokensCumulative >= 100000
}
