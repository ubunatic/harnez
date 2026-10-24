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

func remoteLoadCachePath(host string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(host)))
	return filepath.Join(StateDir(""), "remote-load-"+hex.EncodeToString(digest[:])[:16]+".json")
}

// RemoteLoadCache is the last successful remote load snapshot for one host and
// the time it was written. A zero UpdatedAt is an entry with no recorded time
// (a legacy cache file); a nil Snapshot means there is no usable cache at all.
type RemoteLoadCache struct {
	Snapshot  *LoadSnapshot
	UpdatedAt time.Time
}

// ReadCachedRemoteLoadEntry returns the cached snapshot for host with its
// write time. A missing, malformed, or other-host file yields the zero value.
func ReadCachedRemoteLoadEntry(host string) RemoteLoadCache {
	host = strings.TrimSpace(host)
	if host == "" {
		return RemoteLoadCache{}
	}
	data, err := os.ReadFile(remoteLoadCachePath(host))
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

// WriteCachedRemoteLoad atomically persists the last successful snapshot for host.
func WriteCachedRemoteLoad(host string, snapshot *LoadSnapshot) error {
	host = strings.TrimSpace(host)
	if host == "" || snapshot == nil {
		return fmt.Errorf("remote load cache requires a host and snapshot")
	}
	path := remoteLoadCachePath(host)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(remoteLoadCacheEntry{Host: host, UpdatedAt: time.Now().UTC(), Snapshot: *snapshot})
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".remote-load-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
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
