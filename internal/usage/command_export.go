package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"ubunatic.com/harnez/internal/privacy"
	"ubunatic.com/harnez/internal/telemetry"
)

const usageExportFormatVersion = 2

type usageExportEnvelope struct {
	FormatVersion int              `json:"format_version"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Telemetry     telemetry.Export `json:"telemetry"`
	Usage         UsageExport      `json:"usage"`
}

// runUsageExport preserves the original 3-arg signature at
// privacy.LevelPublic — the pre-issue-204-v2-privacy-levels default
// behavior. Existing callers/tests are unaffected by the addition of
// privacy levels.
func runUsageExport(out, dbPath, historyDir string) error {
	return runUsageExportLevel(out, dbPath, historyDir, privacy.LevelPublic)
}

// runUsageExportLevel is runUsageExport's privacy-level-aware form. A
// privacy.NoteSanitizer (the real ClaudeCLISanitizer) is only constructed
// when level is privacy.LevelAgentSanitized — every other level never
// shells out to the claude CLI.
func runUsageExportLevel(out, dbPath, historyDir string, level privacy.Level) error {
	return runUsageExportFull(out, dbPath, historyDir, level, false)
}

// runUsageExportFull supports both privacy levels and the --classify option.
func runUsageExportFull(out, dbPath, historyDir string, level privacy.Level, classify bool) error {
	envelope, err := buildUsageExport(context.Background(), dbPath, historyDir, level, classify, time.Now())
	if err != nil {
		return err
	}
	return writeUsageExport(out, envelope)
}

// buildUsageExport assembles the envelope at a given now, so tests can pin
// its exact output (see the golden test).
func buildUsageExport(ctx context.Context, dbPath, historyDir string, level privacy.Level, classify bool, now time.Time) (usageExportEnvelope, error) {
	if dbPath == "" {
		p, err := telemetry.DefaultDBPath()
		if err != nil {
			return usageExportEnvelope{}, fmt.Errorf("resolve telemetry db path: %w", err)
		}
		dbPath = p
	}
	var sanitizer privacy.NoteSanitizer
	if level == privacy.LevelAgentSanitized {
		sanitizer = privacy.NewClaudeCLISanitizer()
	}

	var classifier telemetry.NoteBatchClassifier
	if classify {
		classifier = &telemetry.DefaultLocalClassifier{}
	}

	var telExport telemetry.Export
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		telExport = telemetry.BuildExportLevel(nil, now, level, nil)
	} else {
		db, err := telemetry.OpenReadOnly(dbPath)
		if err != nil {
			return usageExportEnvelope{}, fmt.Errorf("open telemetry db: %w", err)
		}
		telExport, err = telemetry.ExportAllWithClassifier(ctx, db, now, level, sanitizer, classifier)
		_ = db.Close()
		if err != nil {
			return usageExportEnvelope{}, fmt.Errorf("export telemetry: %w", err)
		}
	}

	if historyDir == "" {
		historyDir = HistoryDir("")
	}
	usageExport, err := ExportHistoryLevel(historyDir, now, level)
	if err != nil {
		return usageExportEnvelope{}, fmt.Errorf("export usage history: %w", err)
	}

	return usageExportEnvelope{
		FormatVersion: usageExportFormatVersion,
		GeneratedAt:   now,
		Telemetry:     telExport,
		Usage:         usageExport,
	}, nil
}

// writeUsageExport writes envelope as indented JSON to out.
func writeUsageExport(out string, envelope usageExportEnvelope) error {
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal export envelope: %w", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write export file %s: %w", out, err)
	}
	return nil
}

// RunUsageExportFull builds and writes a sanitized usage and telemetry export.
func RunUsageExportFull(out, dbPath, historyDir string, level privacy.Level, classify bool) error {
	return runUsageExportFull(out, dbPath, historyDir, level, classify)
}
