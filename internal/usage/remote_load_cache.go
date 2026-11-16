package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type remoteLoadCacheEntry struct {
	Host      string       `json:"host"`
	UpdatedAt time.Time    `json:"updated_at"`
	Snapshot  LoadSnapshot `json:"snapshot"`
}

func remoteLoadCachePath(host string) (string, error) {
	digest := sha256.Sum256([]byte(strings.TrimSpace(host)))
	dir, err := ResolvedStateDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "remote-load-"+hex.EncodeToString(digest[:])[:16]+".json"), nil
}

// RemoteLoadCache is the last successful remote load snapshot for one host and
// its observation time. A zero UpdatedAt is an entry with no recorded time
// (a legacy cache file); a nil Snapshot means there is no usable cache at all.
type RemoteLoadCache struct {
	Snapshot  *LoadSnapshot
	UpdatedAt time.Time
}

// ReadCachedRemoteLoadEntry returns the cached snapshot for host with its
// observation time. A missing, malformed, or other-host file yields the zero value.
func ReadCachedRemoteLoadEntry(host string) RemoteLoadCache {
	host = strings.TrimSpace(host)
	if host == "" {
		return RemoteLoadCache{}
	}
	path, err := remoteLoadCachePath(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnez: resolve remote load cache: %v\n", err)
		return RemoteLoadCache{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return RemoteLoadCache{}
	}
	var entry remoteLoadCacheEntry
	if json.Unmarshal(data, &entry) != nil || entry.Host != host {
		return RemoteLoadCache{}
	}
	return RemoteLoadCache{Snapshot: &entry.Snapshot, UpdatedAt: entry.UpdatedAt}
}

// ReadCachedRemoteLoad returns the last successful remote load snapshot for host.
func ReadCachedRemoteLoad(host string) *LoadSnapshot {
	return ReadCachedRemoteLoadEntry(host).Snapshot
}

// WriteCachedRemoteLoad atomically persists the last successful snapshot and
// observation time for host. Snapshots without an observation time use receipt time.
func WriteCachedRemoteLoad(host string, snapshot *LoadSnapshot) error {
	host = strings.TrimSpace(host)
	if host == "" || snapshot == nil {
		return fmt.Errorf("remote load cache requires a host and snapshot")
	}
	path, err := remoteLoadCachePath(host)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	observedAt := snapshot.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	data, err := json.Marshal(remoteLoadCacheEntry{Host: host, UpdatedAt: observedAt, Snapshot: *snapshot})
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".remote-load-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName) //nolint:errcheck // best-effort cleanup; error not actionable
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}
