package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

func InsertCLIInvocation(dbPath string, row telemetry.CLIInvocation) error {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.InsertCLIInvocation(row)
}

func InsertIssueSnapshot(dbPath string, row telemetry.IssueStatusSnapshot) (bool, error) {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return false, err
	}
	defer db.Close()
	return db.InsertIssueSnapshot(row)
}

func EnsureTelemetrySchema(dbPath string) (int, []string, error) {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return 0, nil, err
	}
	defer db.Close()
	if err := db.ValidateSchema(); err != nil {
		return 0, nil, err
	}
	version, err := db.SchemaVersion()
	if err != nil {
		return 0, nil, err
	}
	return version, db.Migrations(), nil
}

func MergeLegacyToolCalls(ctx context.Context, dbPath string, sources []string, now time.Time) (string, error) {
	db, err := telemetry.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()
	before, err := db.ToolCallCount(ctx)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	fmt.Fprintf(&output, "tool calls before: %d\n", before)
	var missing int64
	for _, source := range sources {
		n, err := db.LegacyToolCallsMissing(ctx, source)
		if err != nil {
			return "", err
		}
		missing += n
	}
	if missing == 0 {
		fmt.Fprintf(&output, "nothing to merge from %d file(s)\n", len(sources))
		return output.String(), nil
	}
	backup := dbPath + ".before-merge-" + now.UTC().Format("20060102T150405Z")
	if err := db.BackupInto(ctx, backup); err != nil {
		return "", fmt.Errorf("back up %s: %w", dbPath, err)
	}
	fmt.Fprintf(&output, "backup: %s\n", backup)
	results, err := db.MergeLegacyToolCalls(ctx, sources)
	if err != nil {
		return "", err
	}
	for _, result := range results {
		fmt.Fprintf(&output, "added %d from %s\n", result.Added, result.Source)
	}
	after, err := db.ToolCallCount(ctx)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&output, "tool calls after: %d\n", after)
	return output.String(), nil
}
