package modelcatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/xdgpath"
)

// RecordAccess appends observations to the user's catalog without losing concurrent writes.
func RecordAccess(provider, model, status string) error {
	dir := filepath.Join(xdgpath.StateHome(), "harnez", "models")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "access.yaml.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	filename := filepath.Join(dir, stateAccessName)
	doc := map[string]any{"version": 1, "complete": false, "coverage": []any{}, "observations": []any{}}
	if data, readErr := os.ReadFile(filename); readErr == nil {
		if _, err = readValidated(filename, "model-access.schema.json"); err != nil {
			return err
		}
		if err = yaml.Unmarshal(data, &doc); err != nil {
			return err
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	catalog, err := Load()
	if err != nil {
		return err
	}
	if current, _ := latestModelAccess(catalog, provider, model); current == status {
		return nil
	}
	observations, ok := doc["observations"].([]any)
	if !ok {
		return fmt.Errorf("model catalog: invalid observations")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, m := range catalog.Models() {
		if m.Identity.Harness != provider || m.Identity.Model != model {
			continue
		}
		observations = append(observations, map[string]any{"identity": m.Identity, "status": status, "method": "turn", "observed_at": now, "facts": []any{}})
	}
	doc["observations"] = observations
	doc["observed_at"] = now
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".access-*.yaml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filename)
}

// LatestModelAccess reads current user state rather than a process-wide cached catalog.
func LatestModelAccess(provider, model string) (string, *time.Time, error) {
	c, err := Load()
	if err != nil {
		return "", nil, err
	}
	status, at := latestModelAccess(c, provider, model)
	return status, at, nil
}

func latestModelAccess(c *Catalog, provider, model string) (string, *time.Time) {
	var latest time.Time
	status := "unknown"
	for _, o := range c.Access() {
		if o.Identity.Harness != provider || o.Identity.Model != model {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, o.ObservedAt)
		if err == nil && (latest.IsZero() || t.After(latest)) {
			latest, status = t, o.Status
		}
	}
	if latest.IsZero() {
		return status, nil
	}
	return status, &latest
}
