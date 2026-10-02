package subagent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DiscoverExternalSessions returns local sessions described by the embedded
// provider discovery spec. These records are selectable metadata, not proof a
// provider process is currently running.
func DiscoverExternalSessions(home string) ([]*Session, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	var sessions []*Session
	seen := make(map[string]struct{})
	providers := make([]string, 0, len(spec.ExternalSessions))
	for provider := range spec.ExternalSessions {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		discovery := spec.ExternalSessions[provider]
		if discovery.Format != "jsonl" {
			return nil, fmt.Errorf("external session spec: unsupported format %q for %s", discovery.Format, provider)
		}
		if discovery.MaxRecordBytes <= 0 {
			return nil, fmt.Errorf("external session spec: positive max_record_bytes required for %s", provider)
		}
		root := strings.ReplaceAll(discovery.Root, "{home}", home)
		matches, err := filepath.Glob(filepath.Join(root, discovery.Pattern))
		if err != nil {
			return nil, fmt.Errorf("external session spec: glob %s: %w", provider, err)
		}
		for _, path := range matches {
			session, err := readExternalSession(provider, discovery, path)
			if err != nil {
				// Provider files can be truncated while the CLI is writing them.
				// Skip malformed or oversized metadata and keep usable sessions visible.
				continue
			}
			key := provider + "\x00" + session.ProviderID()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func readExternalSession(provider string, spec ExternalSessionSpec, path string) (*Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(io.LimitReader(file, int64(spec.MaxRecordBytes)+1))
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	if len(line) > spec.MaxRecordBytes {
		return nil, fmt.Errorf("external session record exceeds configured limit")
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		return nil, err
	}
	if externalStringField(record, spec.RecordTypePath) != spec.RecordType {
		return nil, fmt.Errorf("external session metadata record not found")
	}
	id := externalStringField(record, spec.Fields.ID)
	workingDir := externalStringField(record, spec.Fields.WorkingDir)
	if id == "" || workingDir == "" {
		return nil, fmt.Errorf("external session metadata is missing identity or working directory")
	}
	providerID := externalStringField(record, spec.Fields.ProviderSessionID)
	if providerID == "" {
		providerID = id
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, externalStringField(record, spec.Fields.CreatedAt))
	shortID := id
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return &Session{
		ID:                id,
		ProviderSessionID: providerID,
		Name:              spec.NamePrefix + "-" + shortID,
		Provider:          provider,
		Source:            externalStringField(record, spec.Fields.Source),
		ModelProvider:     externalStringField(record, spec.Fields.ModelProvider),
		WorkingDir:        workingDir,
		HarnessType:       "external",
		Status:            spec.AvailableStatus,
		CreatedAt:         createdAt,
		LastActiveAt:      info.ModTime(),
	}, nil
}

func externalStringField(record map[string]any, path string) string {
	var value any = record
	for _, part := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		value = object[part]
	}
	text, _ := value.(string)
	return text
}
