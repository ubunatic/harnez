package subagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ErrSessionNameInUse indicates that a registry name or ID is already reserved.
var ErrSessionNameInUse = errors.New("session name or ID is already in use")

// Session represents an active subagent session with metadata and telemetry.
type Session struct {
	ID                string `json:"id"`
	ProviderSessionID string `json:"provider_session_id,omitempty"`
	Name              string `json:"name"`
	StartPrompt       string `json:"start_prompt,omitempty"`
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	Tier              string `json:"tier"`
	WorkingDir        string `json:"working_dir"`
	ParentSessionID   string `json:"parent_session_id,omitempty"`
	CallerPID         int    `json:"caller_pid"`
	ProcessPID        int    `json:"process_pid,omitempty"`
	ControlSocket     string `json:"control_socket,omitempty"`
	HarnessType       string `json:"harness_type"`
	Status            string `json:"status"`
	TokensCumulative  int    `json:"tokens_cumulative"`
	InputTokensTotal  int    `json:"input_tokens_total,omitempty"`
	CachedTokensTotal int    `json:"cached_tokens_total,omitempty"`
	OutputTokensTotal int    `json:"output_tokens_total,omitempty"`
	TokenTotalsKnown  bool   `json:"token_totals_known,omitempty"`
	// TokensSinceCompact counts uncached tokens since the last compaction and
	// drives ShouldCompact; TokensCumulative stays a lifetime telemetry total.
	TokensSinceCompact int          `json:"tokens_since_compact,omitempty"`
	TokensTurn         int          `json:"tokens_turn"`
	CachedTokens       int          `json:"cached_tokens"`
	CreatedAt          time.Time    `json:"created_at"`
	LastActiveAt       time.Time    `json:"last_active_at"`
	Role               string       `json:"role,omitempty"`
	LastError          string       `json:"last_error,omitempty"`
	ResumeFailures     int          `json:"resume_failures,omitempty"`
	Turn               int          `json:"turn,omitempty"`
	TurnRecords        []TurnRecord `json:"turn_records,omitempty"`
}

// TurnRecord stores the usage and optional human rating for one provider turn.
type TurnRecord struct {
	Turn              int    `json:"turn"`
	NewInputTokens    int    `json:"new_input_tokens"`
	CachedInputTokens int    `json:"cached_input_tokens"`
	OutputTokens      int    `json:"output_tokens"`
	Rating            *int   `json:"rating,omitempty"`
	RatingReason      string `json:"rating_reason,omitempty"`
}

// ProviderID returns the provider-side identifier used for lifecycle commands.
// Older registry entries used ID for both the registry and provider identifiers.
func (s *Session) ProviderID() string {
	if s.ProviderSessionID != "" {
		return s.ProviderSessionID
	}
	return s.ID
}

// Find resolves a session by registry ID or short name.
func (s *FileSessionStore) Find(identifier string) (*Session, error) {
	if sess, err := s.Get(identifier); err == nil {
		return sess, nil
	}
	sessions, err := s.List("", true)
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		if sess.Name == identifier {
			return sess, nil
		}
	}
	return nil, fmt.Errorf("session %q not found", identifier)
}

// SessionStore manages persistent session metadata.
type SessionStore interface {
	Save(s *Session) error
	Create(s *Session) error
	Get(id string) (*Session, error)
	Find(identifier string) (*Session, error)
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

// OpenSessionStore opens an existing store without creating directories. It is
// intended for read-only callers such as shell completion.
func OpenSessionStore(dir string) (*FileSessionStore, error) {
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

// Create atomically reserves a session name and creates its initial registry entry.
func (s *FileSessionStore) Create(sess *Session) error {
	if sess.ID == "" || sess.Name == "" {
		return fmt.Errorf("session must have an ID and name")
	}
	lock, err := os.OpenFile(filepath.Join(s.dir, ".registry.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open session registry lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock session registry: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
	sessions, err := s.List("", true)
	if err != nil {
		return err
	}
	for _, existing := range sessions {
		if existing.ID == sess.ID || existing.Name == sess.Name || existing.ID == sess.Name || existing.Name == sess.ID {
			return fmt.Errorf("%w: %q", ErrSessionNameInUse, sess.Name)
		}
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	path := filepath.Join(s.dir, sess.ID+".json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %q", ErrSessionNameInUse, sess.ID)
		}
		return fmt.Errorf("create session file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write session file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close session file: %w", err)
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
	return s.listFrom(s.dir, parentID, allSessions)
}

// ListDeleted returns archived session records kept for long-term stats.
func (s *FileSessionStore) ListDeleted() ([]*Session, error) {
	return s.listFrom(filepath.Join(s.dir, "deleted"), "", true)
}

func (s *FileSessionStore) listFrom(dir, parentID string, allSessions bool) ([]*Session, error) {
	entries, err := os.ReadDir(dir)
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
			data, err := os.ReadFile(filepath.Join(dir, id+".json"))
			if err != nil {
				continue
			}
			var decoded Session
			if err := json.Unmarshal(data, &decoded); err != nil {
				continue
			}
			sess := &decoded
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
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session %q not found", id)
		}
		return fmt.Errorf("failed to read session before deletion: %w", err)
	}
	archiveDir := filepath.Join(s.dir, "deleted")
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return fmt.Errorf("failed to create deleted-session archive: %w", err)
	}
	archivePath := filepath.Join(archiveDir, id+".json")
	if err := os.WriteFile(archivePath, data, 0600); err != nil {
		return fmt.Errorf("failed to archive session before deletion: %w", err)
	}
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
	if target.ID == callerParentID || target.Name == callerParentID {
		return true
	}
	if target.ParentSessionID == callerParentID {
		return true
	}
	return false
}

// ShouldCompact returns true when tokens since the last compaction >= 100,000.
func ShouldCompact(tokensSinceCompact int) bool {
	return tokensSinceCompact >= 100000
}

// CompactionTokens is the part of a turn that grows context: input re-read
// from the provider cache does not count.
func CompactionTokens(r *TurnResult) int {
	return max(r.TokensTurn-r.CachedTokens, 0)
}

// SplitCompactionAck removes the leading message that acknowledges a queued
// compaction from a turn's messages. It returns the remaining messages and the
// acknowledgement ("" when there was nothing to split off).
func SplitCompactionAck(msgs []string) ([]string, string) {
	if len(msgs) < 2 {
		return msgs, ""
	}
	return msgs[1:], msgs[0]
}
