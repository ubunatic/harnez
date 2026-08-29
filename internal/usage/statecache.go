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
// CollectAll attempts a live recollect for that agent instead of serving the
// cache as-is. Twice the default collector interval gives slack for one
// missed tick (a slow network fetch, a momentarily unreachable RPC) without
// immediately reverting every reader to live collection.
//
// This threshold governs *when a live recollect is attempted only* — it is
// deliberately not the threshold for when a display stops trusting/showing
// an agent's last known snapshot (see DefaultDisplayStaleness). Conflating
// the two meant a live process outage (e.g. AGY not currently running)
// could blank out real quota numbers that were only 31 minutes old (issue
// 101).
const DefaultCacheStaleness = 2 * DefaultCollectorInterval

// DefaultDisplayStaleness is how old an agent's last known snapshot may be
// before renderers (RenderText, buildWatchFrame) auto-hide it entirely
// (issue 083's self-hiding behavior). Once an agent has ever produced real
// usage data, its most recently known state stays visible — with a "last
// updated" annotation — until it crosses this much longer window, since a
// merely-not-running agent process is not the same as an abandoned/
// uninstalled one (issue 101).
const DefaultDisplayStaleness = 7 * 24 * time.Hour

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

// PersistAgentSnapshot writes agent's freshly-collected usage to stateDir via
// WriteAgentSnapshot, unless doing so would durably discard quota/token
// signal a previously persisted snapshot already captured — e.g. AGY isn't
// currently running this collector tick, so this live result has no
// Session/Weekly/ModelGroups/Tokens even though the on-disk snapshot does.
//
// cacheOrLive's "don't blank a stale-but-real snapshot on a lossy live
// recollect" guard (issue 101) only helps a reader if a richer snapshot is
// still on disk to fall back to. Both write paths that persist collector
// output (`harnez agent-collector`'s ticking loop and its `--once` mode)
// used to call WriteAgentSnapshot directly and unconditionally, so a single
// collection pass while AGY (or any agent) wasn't running would permanently
// clobber the last known real quota numbers on disk — defeating the read
// side's protection entirely, since there was nothing richer left to serve.
// Callers that persist collector output should use this instead of calling
// WriteAgentSnapshot directly.
func PersistAgentSnapshot(stateDir string, agent AgentUsage) error {
	if existing, err := ReadAgentSnapshot(stateDir, agent.AgentID); err == nil && existing != nil {
		if existing.Usage.hasQuotaSignal() && !agent.hasQuotaSignal() {
			return nil
		}
	}
	return WriteAgentSnapshot(stateDir, agent.AgentID, agent)
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
// tell a cached reading from a live one, and stamping LastRefreshed with the
// snapshot's FetchedAt time; otherwise it runs collect and returns that live
// result (LastRefreshed stamped to now). It never writes back to the cache —
// only the collector daemon (RunCollector) does that.
//
// A live recollect can legitimately come back with *less* quota/token data
// than the cache already had — e.g. AGY's Session/Weekly windows only
// populate when a running agy process answers on a local port (agy.go); a
// stopped process still leaves Authenticated/Sources/etc. populated, but
// loses the quota signal. In that case the emptier live result is discarded
// in favor of the last known snapshot (still tagged with its true
// LastRefreshed time) so a live process outage doesn't blank numbers that
// were real minutes or hours ago (issue 101).
func cacheOrLive(stateDir, agentID string, maxAge time.Duration, collect func() AgentUsage) AgentUsage {
	snap, err := ReadAgentSnapshot(stateDir, agentID)
	hasCache := err == nil && snap != nil

	if hasCache && snap.IsFresh(maxAge) {
		u := snap.Usage
		u.LastRefreshed = snap.FetchedAt
		u.Sources = append(u.Sources, fmt.Sprintf("%s (cached)", snapshotPath(stateDir, agentID)))
		return u
	}

	live := collect()
	live.LastRefreshed = time.Now()

	if hasCache && snap.Usage.hasQuotaSignal() && !live.hasQuotaSignal() {
		u := snap.Usage
		u.LastRefreshed = snap.FetchedAt
		u.Sources = append(u.Sources, fmt.Sprintf("%s (cached, stale)", snapshotPath(stateDir, agentID)))
		return u
	}

	return live
}
