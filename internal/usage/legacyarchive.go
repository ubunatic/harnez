package usage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
	"ubunatic.com/harnez/internal/xdgpath"
)

const legacyUsageArchiveManifest = "manifest.json"

// LegacyUsageArchivePaths identifies only the usage files being consolidated.
// Fetch-duration cache files are deliberately excluded.
type LegacyUsageArchivePaths struct {
	Home, DataHome, CacheHome, StateHome, ArchiveBase string
}

// LegacyUsageArchiveManifest describes a verified copy of legacy usage data.
type LegacyUsageArchiveManifest struct {
	Version       int                        `json:"version"`
	CreatedAt     time.Time                  `json:"created_at"`
	ImportCommand string                     `json:"import_command"`
	Files         []LegacyUsageArchiveRecord `json:"files"`
}

type LegacyUsageArchiveRecord struct {
	Kind        string `json:"kind"`
	SourcePath  string `json:"source_path"`
	ArchivePath string `json:"archive_path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
}

// LegacyUsageArchiveResult summarizes a copy without moving or removing any
// source file.
type LegacyUsageArchiveResult struct {
	Path  string
	Files int
}

// DefaultLegacyUsageArchivePaths resolves the active home and XDG roots.
func DefaultLegacyUsageArchivePaths() LegacyUsageArchivePaths {
	home, _ := os.UserHomeDir()
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" || !filepath.IsAbs(stateHome) {
		stateHome = filepath.Join(home, ".local", "state")
	}
	dataHome := xdgpath.DataHome()
	return LegacyUsageArchivePaths{
		Home: home, DataHome: dataHome, CacheHome: xdgpath.CacheHome(), StateHome: stateHome,
		ArchiveBase: filepath.Join(dataHome, "harnez", "archive", "usage-legacy"),
	}
}

// ArchiveLegacyUsageData copies known legacy usage stores and writes a
// checksummed manifest last. Existing source files remain in place.
func ArchiveLegacyUsageData(paths LegacyUsageArchivePaths) (LegacyUsageArchiveResult, error) {
	if paths.Home == "" || paths.DataHome == "" || paths.CacheHome == "" || paths.StateHome == "" || paths.ArchiveBase == "" {
		return LegacyUsageArchiveResult{}, errors.New("usage archive: all source and destination roots are required")
	}
	created := time.Now().UTC()
	root := filepath.Join(paths.ArchiveBase, created.Format("20060102T150405.000000000Z"))
	if err := os.MkdirAll(root, 0o700); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	manifest := LegacyUsageArchiveManifest{Version: 1, CreatedAt: created, ImportCommand: "harnez usage archive import <archive-directory>"}
	seen := map[string]bool{}
	add := func(kind, namespace, base string, matches []string) error {
		for _, source := range matches {
			abs, err := filepath.Abs(source)
			if err != nil {
				return err
			}
			if seen[abs] {
				continue
			}
			info, err := os.Stat(abs)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				continue
			}
			rel, err := filepath.Rel(base, abs)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("usage archive: source %s is outside root %s", abs, base)
			}
			archiveRel := filepath.Join("sources", namespace, rel)
			destination := filepath.Join(root, archiveRel)
			if err := copyAndHash(abs, destination); err != nil {
				return err
			}
			data, err := os.ReadFile(destination)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			manifest.Files = append(manifest.Files, LegacyUsageArchiveRecord{Kind: kind, SourcePath: abs, ArchivePath: archiveRel, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])})
			seen[abs] = true
		}
		return nil
	}
	glob := func(pattern string) ([]string, error) { return filepath.Glob(pattern) }
	addPattern := func(kind, namespace, base, pattern string) error {
		matches, err := glob(pattern)
		if err != nil {
			return err
		}
		return add(kind, namespace, base, matches)
	}
	for _, provider := range []string{"claude", "codex", "agy"} {
		if err := addPattern("provider-cache", "cache", filepath.Join(paths.CacheHome, "harnez"), filepath.Join(paths.CacheHome, "harnez", "quota-cache-"+provider+".json")); err != nil {
			return LegacyUsageArchiveResult{}, err
		}
	}
	for _, dir := range []string{
		".claude", ".codex", filepath.Join(".gemini", "antigravity-cli"),
	} {
		if err := addPattern("provider-cache", "home", paths.Home, filepath.Join(paths.Home, dir, "harnez-quota-cache.json")); err != nil {
			return LegacyUsageArchiveResult{}, err
		}
	}
	stateUsage := filepath.Join(paths.StateHome, "harnez", "agents", "usage")
	if err := addPattern("state-snapshot", "state", filepath.Join(paths.StateHome, "harnez", "agents"), filepath.Join(stateUsage, "*.json")); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	history := filepath.Join(paths.DataHome, "harnez", "usage-history")
	if err := addPattern("usage-history", "data", filepath.Join(paths.DataHome, "harnez"), filepath.Join(history, "*.jsonl")); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	legacyHistory := filepath.Join(paths.Home, ".claude", "harnez", "usage-history")
	if err := addPattern("usage-history", "home", paths.Home, filepath.Join(legacyHistory, "*.jsonl")); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	turnRoot := filepath.Join(paths.Home, ".harnez", "agents")
	turnFiles, err := filepath.Glob(filepath.Join(turnRoot, "*", "quota-readings.jsonl"))
	if err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	if err := add("turn-quota-jsonl", "home", paths.Home, turnFiles); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].ArchivePath < manifest.Files[j].ArchivePath })
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	manifestPath := filepath.Join(root, legacyUsageArchiveManifest)
	tmp := manifestPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	if err := os.Rename(tmp, manifestPath); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	return LegacyUsageArchiveResult{Path: root, Files: len(manifest.Files)}, nil
}

// ensureLegacyUsageArchive archives the legacy sources only when no complete
// archive exists yet. Once one does, the sources are mirrors regenerated from
// the telemetry store, so copying them again on every write only fills the disk.
func ensureLegacyUsageArchive(paths LegacyUsageArchivePaths) error {
	manifests, err := filepath.Glob(filepath.Join(paths.ArchiveBase, "*", legacyUsageArchiveManifest))
	if err != nil {
		return err
	}
	if len(manifests) > 0 {
		return nil
	}
	_, err = ArchiveLegacyUsageData(paths)
	return err
}

func copyAndHash(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// ImportLegacyUsageArchive validates every archived file before importing its
// usage records. The manifest remains available as the copy's recovery map.
func ImportLegacyUsageArchive(ctx context.Context, dbPath, root string) (LegacyUsageArchiveResult, error) {
	if dbPath == "" {
		return LegacyUsageArchiveResult{}, errors.New("usage archive: telemetry database path is required")
	}
	manifestPath := filepath.Join(root, legacyUsageArchiveManifest)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	var manifest LegacyUsageArchiveManifest
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version != 1 {
		return LegacyUsageArchiveResult{}, fmt.Errorf("usage archive: invalid manifest")
	}
	for _, record := range manifest.Files {
		clean := filepath.Clean(record.ArchivePath)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return LegacyUsageArchiveResult{}, fmt.Errorf("usage archive: invalid relative path %q", record.ArchivePath)
		}
		file := filepath.Join(root, clean)
		content, err := os.ReadFile(file)
		if err != nil {
			return LegacyUsageArchiveResult{}, err
		}
		sum := sha256.Sum256(content)
		if int64(len(content)) != record.Size || hex.EncodeToString(sum[:]) != record.SHA256 {
			return LegacyUsageArchiveResult{}, fmt.Errorf("usage archive: checksum mismatch for %s", record.ArchivePath)
		}
	}
	store, err := usagestore.Open(dbPath)
	if err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	manifestHash := sha256.Sum256(data)
	marker := "legacy-archive-import-v1:" + hex.EncodeToString(manifestHash[:])
	if err := store.Exec(ctx, `CREATE TABLE IF NOT EXISTS usage_store_migrations (name TEXT PRIMARY KEY, completed_at TEXT NOT NULL)`); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	var done int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM usage_store_migrations WHERE name=?`, marker).Scan(&done); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	if done > 0 {
		return LegacyUsageArchiveResult{Path: root, Files: len(manifest.Files)}, nil
	}
	for _, record := range manifest.Files {
		path := filepath.Join(root, filepath.Clean(record.ArchivePath))
		switch record.Kind {
		case "provider-cache":
			if err := importArchivedProviderCache(ctx, store, path); err != nil {
				return LegacyUsageArchiveResult{}, err
			}
		case "state-snapshot":
			if err := importArchivedStateSnapshot(ctx, store, path); err != nil {
				return LegacyUsageArchiveResult{}, err
			}
		case "usage-history":
			if err := importArchivedHistory(ctx, store, path); err != nil {
				return LegacyUsageArchiveResult{}, err
			}
		case "turn-quota-jsonl":
			if _, err := ImportTurnQuotaJSONL(ctx, dbPath, path); err != nil {
				return LegacyUsageArchiveResult{}, err
			}
		default:
			return LegacyUsageArchiveResult{}, fmt.Errorf("usage archive: unsupported record kind %q", record.Kind)
		}
	}
	if err := store.Exec(ctx, `INSERT OR IGNORE INTO usage_store_migrations(name,completed_at) VALUES(?,?)`, marker, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return LegacyUsageArchiveResult{}, err
	}
	return LegacyUsageArchiveResult{Path: root, Files: len(manifest.Files)}, nil
}

func importArchivedProviderCache(ctx context.Context, store *usagestore.Store, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var header struct {
		FetchedAt time.Time       `json:"fetched_at"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.FetchedAt.IsZero() {
		return nil
	}
	base := filepath.Base(path)
	provider := ""
	if strings.Contains(base, "claude") || strings.Contains(filepath.ToSlash(path), "/.claude/") {
		provider = "claude"
	} else if strings.Contains(base, "codex") || strings.Contains(filepath.ToSlash(path), "/.codex/") {
		provider = "codex"
	} else if strings.Contains(base, "agy") || strings.Contains(filepath.ToSlash(path), "/.gemini/") {
		provider = "agy"
	}
	if provider == "" {
		return fmt.Errorf("usage archive: cannot identify provider cache %s", path)
	}
	agent := AgentUsage{AgentID: provider, LastRefreshed: header.FetchedAt}
	switch provider {
	case "claude":
		var payload claudeQuotaPayload
		if err := json.Unmarshal(header.Payload, &payload); err != nil {
			return err
		}
		agent.Session, agent.Weekly = payload.Session, payload.Weekly
	case "codex":
		var payload codexQuotaPayload
		if err := json.Unmarshal(header.Payload, &payload); err != nil {
			return err
		}
		agent.Session, agent.Weekly = payload.Session, payload.Weekly
	case "agy":
		var payload agyQuotaPayload
		if err := json.Unmarshal(header.Payload, &payload); err != nil {
			return err
		}
		agent.ModelGroups = payload.ModelGroups
	}
	return importAgent(store, ctx, provider, "provider-cache", header.FetchedAt, agent)
}

func importArchivedStateSnapshot(ctx context.Context, store *usagestore.Store, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var snapshot AgentSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}
	provider := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if snapshot.FetchedAt.IsZero() {
		return nil
	}
	snapshot.Usage.AgentID = provider
	return importAgent(store, ctx, provider, "state-snapshot", snapshot.FetchedAt, snapshot.Usage)
}

func importArchivedHistory(ctx context.Context, store *usagestore.Store, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := append([]byte(nil), scanner.Bytes()...)
		if filepath.Base(path) == QuotaHistoryFilename {
			var entry QuotaHistoryEntry
			if json.Unmarshal(line, &entry) != nil || entry.Agent == "" || entry.Timestamp.IsZero() {
				continue
			}
			window := QuotaWindow{Name: entry.Window, UsedPercent: float64(entry.UsedPercent), RemainingPercent: float64(entry.RemainingPercent), ResetAt: entry.ResetAt}
			agent := AgentUsage{AgentID: entry.Agent, LastRefreshed: entry.Timestamp}
			if entry.Group != "" {
				agent.ModelGroups = []ModelGroup{{Name: entry.Group, Windows: []QuotaWindow{window}}}
			} else if strings.Contains(strings.ToLower(entry.Window), "week") {
				agent.Weekly = &window
			} else {
				agent.Session = &window
			}
			if err := importAgent(store, ctx, entry.Agent, "history", entry.Timestamp, agent); err != nil {
				return err
			}
			continue
		}
		var entry HistoryEntry
		if json.Unmarshal(line, &entry) != nil || entry.Timestamp.IsZero() {
			continue
		}
		if entry.Hostname == "" {
			entry.Hostname = "unknown-host"
		}
		payload, err := json.Marshal(entry.UsageSummary)
		if err != nil {
			return err
		}
		lineHash := sha256.Sum256(line)
		sourceKey := "legacy-history:" + hex.EncodeToString(lineHash[:]) + ":" + fmt.Sprint(lineNumber)
		if err := store.WriteUsageSummary(ctx, usagestore.UsageSummaryRecord{Hostname: entry.Hostname, ObservedAt: entry.Timestamp, Payload: payload, RecordKey: sourceKey}); err != nil {
			return err
		}
		for _, agent := range entry.Agents {
			if agent.AgentID == "" {
				continue
			}
			if err := importAgent(store, ctx, agent.AgentID, "history", entry.Timestamp, agent); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
