package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// stateCacheDirEnv is the XDG Base Directory env var that, when set,
// overrides the default state directory location.
const stateCacheDirEnv = "XDG_STATE_HOME"

// stateAppDir/stateAgentsUsageSubdir compose the shared harnez agent-usage
// state path: <state-base>/harnez/agents/usage/<agent>.json. This is where
// `harnez agent-collector` (issue 082) writes snapshots and where CollectAll
// reads them from before falling back to a live collect.
const stateAppDir = "harnez"

var stateAgentsUsageSubdir = filepath.Join("agents", "usage")

// DefaultCollectorInterval is the default tick cadence for `harnez
// agent-collector`, matching Omarchy 4.0's Quickshell "Agents" widget
// (issue 082's stated prior art).
const DefaultCollectorInterval = 900 * time.Second

// DefaultCacheStaleness is how old a cached snapshot may be before
// CollectAll treats it as stale and falls back to a live collect for that
// agent. Twice the default collector interval gives slack for one missed
// tick (a slow network fetch, a momentarily unreachable RPC) without
// immediately reverting every reader to live collection.
const DefaultCacheStaleness = 2 * DefaultCollectorInterval

// AgentSnapshot is the on-disk shape of <state-dir>/<agent-id>.json: one
// file per agent, so a reader can check freshness and availability
// independently per agent rather than only for the summary as a whole.
type AgentSnapshot struct {
	FetchedAt time.Time  `json:"fetched_at"`
	Usage     AgentUsage `json:"usage"`
}

// IsFresh reports whether the snapshot exists and is within maxAge of now.
// A nil snapshot (not yet collected) is never fresh.
func (s *AgentSnapshot) IsFresh(maxAge time.Duration) bool {
	if s == nil {
		return false
	}
	return time.Since(s.FetchedAt) < maxAge
}

// StateDir resolves the shared harnez agent-usage state directory following
// the XDG Base Directory spec:
//
//	$XDG_STATE_HOME/harnez/agents/usage
//
// falling back to
//
//	~/.local/state/harnez/agents/usage
//
// when XDG_STATE_HOME is unset or empty. homeDir lets callers (and tests)
// pin the fallback base explicitly instead of relying on the real
// os.UserHomeDir(); pass "" to resolve it automatically.
func StateDir(homeDir string) string {
	if xdg := os.Getenv(stateCacheDirEnv); xdg != "" {
		return filepath.Join(xdg, stateAppDir, stateAgentsUsageSubdir)
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return filepath.Join(homeDir, ".local", "state", stateAppDir, stateAgentsUsageSubdir)
}

// snapshotPath returns the path of one agent's snapshot file within stateDir.
func snapshotPath(stateDir, agentID string) string {
	return filepath.Join(stateDir, agentID+".json")
}

// WriteAgentSnapshot atomically persists one agent's usage snapshot: it
// marshals to a temp file in stateDir, then renames it into place. Rename is
// atomic on the same filesystem, so a concurrent reader (harnez usage / the
// TUI) can never observe a torn/partial write. Unlike the Claude-only
// quotaCache (issue 033), no flock is needed here — the collector daemon is
// the sole writer for these files, so there is no cross-process write race
// to close.
func WriteAgentSnapshot(stateDir, agentID string, usage AgentUsage) error {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("create state dir %s: %w", stateDir, err)
	}
	snap := AgentSnapshot{FetchedAt: time.Now(), Usage: usage}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s snapshot: %w", agentID, err)
	}
	path := snapshotPath(stateDir, agentID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp %s snapshot: %w", agentID, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s snapshot into place: %w", agentID, err)
	}
	return nil
}

// ReadAgentSnapshot reads and parses one agent's snapshot file from
// stateDir. A missing file is reported as (nil, nil) — the daemon not
// having run yet, or not having collected this agent yet, is not an error
// the caller has to unwrap.
func ReadAgentSnapshot(stateDir, agentID string) (*AgentSnapshot, error) {
	data, err := os.ReadFile(snapshotPath(stateDir, agentID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snap AgentSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse %s snapshot: %w", agentID, err)
	}
	return &snap, nil
}

// cacheOrLive returns the agent's cached snapshot from stateDir if present
// and fresher than maxAge, tagging its Sources so renderers/JSON output can
// tell a cached reading from a live one; otherwise it runs collect and
// returns that live result. It never writes back to the cache — only the
// collector daemon (RunCollector) does that.
func cacheOrLive(stateDir, agentID string, maxAge time.Duration, collect func() AgentUsage) AgentUsage {
	snap, err := ReadAgentSnapshot(stateDir, agentID)
	if err == nil && snap.IsFresh(maxAge) {
		u := snap.Usage
		u.Sources = append(u.Sources, fmt.Sprintf("%s (cached)", snapshotPath(stateDir, agentID)))
		return u
	}
	return collect()
}
