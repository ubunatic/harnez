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

// ReadCachedRemoteLoad returns the last successful remote load snapshot for host.
func ReadCachedRemoteLoad(host string) *LoadSnapshot {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil
	}
	data, err := os.ReadFile(remoteLoadCachePath(host))
	if err != nil {
		return nil
	}
	var entry remoteLoadCacheEntry
	if json.Unmarshal(data, &entry) != nil || entry.Host != host {
		return nil
	}
	return &entry.Snapshot
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
