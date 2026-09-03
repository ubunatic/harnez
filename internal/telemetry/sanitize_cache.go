// sanitize_cache.go implements issue 204's Level 2 (agent-sanitized)
// export privacy level: a content-hash cache over note_sanitization_cache
// (see schema.go) plus the batching orchestration that turns a set of raw
// notes into sanitized text while calling privacy.NoteSanitizer as few
// times as possible.
package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/privacy"
)

// noteHash returns the cache key for a raw note: the hex-encoded sha256
// of its exact text.
func noteHash(note string) string {
	sum := sha256.Sum256([]byte(note))
	return hex.EncodeToString(sum[:])
}

// GetCachedNotes looks up any of hashes already present in
// note_sanitization_cache, returning a hash -> clean_text map containing
// only the hits (misses are simply absent from the returned map).
func (d *DB) GetCachedNotes(ctx context.Context, hashes []string) (map[string]string, error) {
	out := make(map[string]string, len(hashes))
	if len(hashes) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(hashes))
	args := make([]any, len(hashes))
	for i, h := range hashes {
		placeholders[i] = "?"
		args[i] = h
	}
	query := fmt.Sprintf(
		"SELECT raw_hash, clean_text FROM note_sanitization_cache WHERE raw_hash IN (%s)",
		strings.Join(placeholders, ","),
	)
	rows, err := d.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query note_sanitization_cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var hash, clean string
		if err := rows.Scan(&hash, &clean); err != nil {
			return nil, fmt.Errorf("telemetry: scan note_sanitization_cache row: %w", err)
		}
		out[hash] = clean
	}
	return out, rows.Err()
}

// PutCachedNotes writes/updates entries (raw_hash -> clean_text) into
// note_sanitization_cache in one transaction. Existing rows for the same
// hash are overwritten (a hash is a content hash of the raw note, so this
// only happens if the sanitizer is deliberately re-run over an
// already-cached note, e.g. after a prompt change).
func (d *DB) PutCachedNotes(ctx context.Context, entries map[string]string, now time.Time) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("telemetry: begin note_sanitization_cache write: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO note_sanitization_cache (raw_hash, clean_text, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(raw_hash) DO UPDATE SET
			clean_text = excluded.clean_text,
			created_at = excluded.created_at`)
	if err != nil {
		return fmt.Errorf("telemetry: prepare note_sanitization_cache upsert: %w", err)
	}
	defer stmt.Close()

	createdAt := now.Format(time.RFC3339Nano)
	for hash, clean := range entries {
		if _, err := stmt.ExecContext(ctx, hash, clean, createdAt); err != nil {
			return fmt.Errorf("telemetry: upsert note_sanitization_cache: %w", err)
		}
	}
	return tx.Commit()
}

// SanitizeNotes resolves every distinct, non-empty note in notes to its
// sanitized text, returning a raw-note -> sanitized-text map suitable for
// BuildExportLevel's LevelAgentSanitized branch. It:
//  1. Hashes each distinct note and checks note_sanitization_cache for a
//     hit, skipping the sanitizer entirely for anything already seen.
//  2. Batches every cache miss into chunks of at most
//     privacy.DefaultSanitizeBatchSize notes and calls sanitizer.SanitizeBatch
//     once per chunk — never once per note (issue 204's explicit batching
//     requirement).
//  3. Writes every newly-sanitized note back into the cache before
//     returning, so the next export run (even for a different privacy
//     level that happens to also want Level 2) sees a cache hit.
//
// sanitizer may be nil only if notes contains no cache misses (e.g. every
// note was already sanitized in a previous run) — SanitizeNotes returns an
// error rather than panicking if a nil sanitizer is actually needed.
func SanitizeNotes(ctx context.Context, db *DB, sanitizer privacy.NoteSanitizer, notes []string, now time.Time) (map[string]string, error) {
	result := make(map[string]string)
	if len(notes) == 0 {
		return result, nil
	}

	// Dedupe: an identical note string appearing on many rows (common —
	// e.g. repeated "git status: clean" notes) should only ever be
	// hashed/looked-up/sanitized once per export run.
	seen := make(map[string]bool, len(notes))
	var distinct []string
	var hashes []string
	for _, n := range notes {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		distinct = append(distinct, n)
		hashes = append(hashes, noteHash(n))
	}
	if len(distinct) == 0 {
		return result, nil
	}

	cached, err := db.GetCachedNotes(ctx, hashes)
	if err != nil {
		return nil, err
	}

	var missNotes, missHashes []string
	for i, n := range distinct {
		if clean, ok := cached[hashes[i]]; ok {
			result[n] = clean
		} else {
			missNotes = append(missNotes, n)
			missHashes = append(missHashes, hashes[i])
		}
	}
	if len(missNotes) == 0 {
		return result, nil
	}
	if sanitizer == nil {
		return nil, fmt.Errorf("telemetry: %d note(s) need agent sanitization but no NoteSanitizer was provided", len(missNotes))
	}

	batchSize := privacy.DefaultSanitizeBatchSize
	for start := 0; start < len(missNotes); start += batchSize {
		end := start + batchSize
		if end > len(missNotes) {
			end = len(missNotes)
		}
		chunkNotes := missNotes[start:end]
		chunkHashes := missHashes[start:end]

		clean, err := sanitizer.SanitizeBatch(ctx, chunkNotes)
		if err != nil {
			return nil, fmt.Errorf("telemetry: sanitize note batch [%d:%d]: %w", start, end, err)
		}
		if len(clean) != len(chunkNotes) {
			return nil, fmt.Errorf("telemetry: sanitizer returned %d note(s) for a batch of %d", len(clean), len(chunkNotes))
		}

		toCache := make(map[string]string, len(clean))
		for i, c := range clean {
			result[chunkNotes[i]] = c
			toCache[chunkHashes[i]] = c
		}
		if err := db.PutCachedNotes(ctx, toCache, now); err != nil {
			return nil, err
		}
	}
	return result, nil
}
